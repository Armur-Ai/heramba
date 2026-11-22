// Package edge is Heramba's edge runtime: a reverse proxy / sidecar that sits in
// front of the protected app. On every request it classifies the source and
// watches the deception surface, emitting detection events. Its posture is set
// by config.Mode:
//
//   - OBSERVE (default): classify + detect + serve the honeypot decoys, but pass
//     real application traffic through untouched (no header rewriting, no
//     tarpit). Zero inline risk — safe to deploy in front of production first.
//   - PROTECT (opt-in): additionally normalize response headers (signal denial)
//     and tarpit sources the classifier flags as agentic.
//
// Decoy endpoints are always served by the edge (they do not exist upstream), so
// decoy detection works in both modes. The OSS engine serves a neutral 404 for a
// decoy hit — detection is the value; believable decoy *content* is a commercial
// plugin.
package edge

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Armur-Ai/heramba/internal/alert"
	"github.com/Armur-Ai/heramba/internal/classify"
	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/counter"
	"github.com/Armur-Ai/heramba/internal/decoy"
	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/ratelimit"
	"github.com/Armur-Ai/heramba/internal/signal"
)

// Engine is the configured edge runtime. Build with New, serve with ServeHTTP.
//
// All configuration-derived state lives in an atomically-swapped *engineState,
// so Reload can replace the decoy surface, sinks, mode and upstream on a running
// engine without dropping in-flight connections: each request loads the current
// snapshot once and uses it throughout. The classifier is deliberately NOT part
// of the snapshot — it is kept across reloads so learned per-source behavior is
// not lost when config changes.
type Engine struct {
	cls      *classify.Classifier // stable across reloads (keeps learned state)
	honeypot http.Handler         // optional deception app; set via SetHoneypot, not reloaded
	st       atomic.Pointer[engineState]
}

// engineState is the reloadable snapshot of config-derived runtime state.
type engineState struct {
	cfg       config.Config
	reg       *decoy.Registry
	norm      *signal.Normalizer // protect mode only
	jit       *signal.Jitter     // protect-mode tarpit for flagged sources
	sink      alert.Sink
	proxy     *httputil.ReverseProxy // nil in honeypot-only (no upstream)
	trusted   []*net.IPNet           // proxy CIDRs whose XFF we trust
	limiter   *ratelimit.Limiter     // per-source rate limit (protect mode only)
	counterOn bool                   // counter-agent prompt-decoy injection (protect mode)
}

// state returns the current reloadable snapshot.
func (e *Engine) state() *engineState { return e.st.Load() }

// SetHoneypot installs a fallthrough handler used ONLY in honeypot-only
// deployments (no upstream): requests that are not decoys/canaries are served
// by this handler (e.g. a stateful deception app) instead of a neutral 404, so
// an autonomous agent gets a coherent, engaging fake application. It has no
// effect when a reverse-proxy upstream is configured.
func (e *Engine) SetHoneypot(h http.Handler) { e.honeypot = h }

// ctxKey is a private request-context key type.
type ctxKey int

// ctxFlagAgentic marks a request whose source was flagged agentic, so the proxy
// response modifier can decide to inject a counter-agent lure.
const ctxFlagAgentic ctxKey = iota

// New assembles an edge Engine from config. In PROTECT mode it enables response
// normalization and a tarpit. An upstream (reverse-proxy target) is optional in
// observe mode (no upstream => honeypot-only: decoys + 404).
func New(cfg config.Config, sink alert.Sink) (*Engine, error) {
	cls := classify.NewWithOptions(classify.Options{
		CadenceRPS:   cfg.Classifier.CadenceRPS,
		EnumDistinct: cfg.Classifier.EnumDistinct,
		ToolAt:       cfg.Classifier.ToolAt,
		AgenticAt:    cfg.Classifier.AgenticAt,
	})
	e := &Engine{cls: cls}
	st, err := e.buildState(cfg, sink)
	if err != nil {
		return nil, err
	}
	e.st.Store(st)
	return e, nil
}

// buildState assembles a reloadable snapshot from config. It is used by New and
// by Reload; ModifyResponse closes over the Engine (not the snapshot) so it
// always reads the live state, which keeps reloads correct for in-flight proxied
// responses.
func (e *Engine) buildState(cfg config.Config, sink alert.Sink) (*engineState, error) {
	if sink == nil {
		sink = alert.Discard
	}
	st := &engineState{cfg: cfg, reg: decoy.NewRegistry(cfg.Decoys.Paths...), sink: sink}
	if p, ok := decoy.GetPreset(cfg.Decoys.Preset); ok {
		st.reg.ApplyPreset(p) // framework-tailored lures on top of the defaults
	}
	for _, tok := range cfg.Decoys.Canaries {
		st.reg.Canaries().Watch(tok, "configured canary")
	}
	for _, c := range cfg.TrustedProxies {
		if _, n, err := net.ParseCIDR(c); err == nil {
			st.trusted = append(st.trusted, n)
		}
	}
	if cfg.Mode == config.ModeProtect {
		st.norm = signal.DefaultNormalizer()
		st.jit = signal.NewJitter(250*time.Millisecond, 1*time.Second)
		if cfg.RateLimitRPS > 0 {
			st.limiter = ratelimit.New(cfg.RateLimitRPS, cfg.RateLimitBurst)
		}
		st.counterOn = cfg.Counter.Enabled
	}
	if cfg.Upstream != "" {
		u, err := url.Parse(cfg.Upstream)
		if err != nil {
			return nil, fmt.Errorf("edge: bad upstream %q: %w", cfg.Upstream, err)
		}
		rp := httputil.NewSingleHostReverseProxy(u)
		if st.norm != nil || st.counterOn {
			rp.ModifyResponse = e.modifyResponse
		}
		st.proxy = rp
	}
	return st, nil
}

// Reload atomically swaps the engine's config-derived state (decoy surface,
// canaries, preset, sinks, mode, upstream, rate limit) without interrupting
// in-flight requests. The classifier's learned state is preserved. A bad
// upstream in the new config is reported and the current state is left in place.
//
// Passing a nil sink keeps the engine's current sink — useful when the alert
// wiring is swapped out-of-band (e.g. behind a SwapSink) and only the detection
// surface/mode needs to change here.
func (e *Engine) Reload(cfg config.Config, sink alert.Sink) error {
	if sink == nil {
		if cur := e.st.Load(); cur != nil {
			sink = cur.sink
		}
	}
	st, err := e.buildState(cfg, sink)
	if err != nil {
		return err
	}
	e.st.Store(st)
	return nil
}

// modifyResponse applies protect-mode response transforms: header normalization
// (signal denial) and, for flagged agentic sources, counter-agent lure
// injection into HTML/JS bodies.
func (e *Engine) modifyResponse(resp *http.Response) error {
	st := e.state()
	if st.norm != nil {
		st.norm.Apply(resp.Header)
	}
	if !st.counterOn || resp.Request == nil {
		return nil
	}
	if flag, _ := resp.Request.Context().Value(ctxFlagAgentic).(bool); !flag {
		return nil
	}
	// Only inject into uncompressed HTML/JS we can safely rewrite.
	if resp.Header.Get("Content-Encoding") != "" {
		return nil
	}
	ct := resp.Header.Get("Content-Type")
	isHTML := strings.Contains(ct, "text/html")
	isJS := strings.Contains(ct, "javascript")
	if !isHTML && !isJS {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return nil // never break the response over an injection failure
	}
	lure := counter.Generate(-1)
	var out []byte
	if isHTML {
		out = counter.InjectHTML(body, lure)
	} else {
		out = counter.InjectJS(body, lure)
	}
	resp.Body = io.NopCloser(bytes.NewReader(out))
	resp.ContentLength = int64(len(out))
	resp.Header.Set("Content-Length", itoaLen(len(out)))

	// Close the loop: the fabricated paths become decoys; the key a canary.
	for _, p := range lure.Paths {
		st.reg.Add(p, "counter-agent lure")
	}
	st.reg.Canaries().Watch(lure.Key, "counter-agent lure key")
	st.sink.Emit(event.New(event.KindCounterDeployed, event.SeverityInfo, "", resp.Request.URL.Path,
		"injected counter-agent lure").WithLabel("lure_key", lure.Key))
	return nil
}

func itoaLen(n int) string {
	return fmt.Sprintf("%d", n)
}

// Handler returns the edge as an http.Handler.
func (e *Engine) Handler() http.Handler { return e }

func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	st := e.state() // one consistent snapshot for the whole request
	ip := e.clientIP(r, st)

	// 1. Classify the source (both modes — detection is the point of observe).
	v := e.cls.Classify(classify.Observation{
		Source: ip, Method: r.Method, Path: r.URL.Path,
		UserAgent: r.UserAgent(), Headers: headerSubset(r), Time: time.Now().UTC(),
	})
	if v.Label == "agentic-suspected" {
		st.sink.Emit(event.New(event.KindAgenticTraffic, event.SeverityMedium, ip, r.URL.Path,
			fmt.Sprintf("agentic-suspected (score %.2f)", v.Score)).
			WithLabel("label", v.Label))
	}

	// 2a. Canary use: a planted canary token appearing in the request is a
	// confirmed intrusion (zero-FP). In protect mode the request is denied.
	if c, ok := e.scanCanary(r, st); ok {
		st.sink.Emit(event.Deception(event.KindCanaryTriggered, ip, c.Label,
			"canary token used ("+c.Label+")").WithLabel("canary_label", c.Label))
		if st.cfg.Mode == config.ModeProtect {
			e.serveNeutral(w, st)
			return
		}
	}

	// 2b. Deception surface: a decoy hit is zero-FP; the edge serves it.
	if _, ok := st.reg.IsDecoy(r.URL.Path); ok {
		if ev, hit := st.reg.InspectPath(r.Method, r.URL.Path, ip); hit {
			st.sink.Emit(ev)
		}
		e.serveNeutral(w, st)
		return
	}

	// 3. Real application traffic.
	// Rate limit abusive sources (protect mode only).
	if st.limiter != nil && !st.limiter.Allow(ip) {
		st.sink.Emit(event.New(event.KindRateLimited, event.SeverityMedium, ip, r.URL.Path,
			"source exceeded request-rate budget"))
		if st.norm != nil {
			st.norm.Apply(w.Header())
		}
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("429 too many requests\n"))
		return
	}
	if st.cfg.Mode == config.ModeProtect && v.Label == "agentic-suspected" && st.jit != nil {
		st.jit.Sleep(r.Context()) // tarpit flagged sources
	}
	if st.counterOn && v.Label == "agentic-suspected" {
		r = r.WithContext(context.WithValue(r.Context(), ctxFlagAgentic, true))
	}
	if st.proxy != nil {
		st.proxy.ServeHTTP(w, r)
		return
	}
	// Honeypot-only deployment (no upstream). If a deception app is installed,
	// serve non-decoy paths from it (coherent synthetic application); otherwise
	// return a neutral 404 indistinguishable from a missing path.
	if e.honeypot != nil {
		e.honeypot.ServeHTTP(w, r)
		return
	}
	e.serveNeutral(w, st)
}

// serveNeutral writes a minimal 404 with normalized headers (protect mode),
// indistinguishable from a genuinely missing path.
func (e *Engine) serveNeutral(w http.ResponseWriter, st *engineState) {
	if st.norm != nil {
		st.norm.Apply(w.Header())
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("404 page not found\n"))
}

// scanCanary looks for any watched canary token in the request's path, query,
// and header values (cookies/auth headers included). Body scanning is a roadmap
// item; this covers the common vector (a planted fake credential used in a
// header or query).
func (e *Engine) scanCanary(r *http.Request, st *engineState) (decoy.Canary, bool) {
	cans := st.reg.Canaries().All()
	if len(cans) == 0 {
		return decoy.Canary{}, false
	}
	var sb strings.Builder
	sb.WriteString(r.URL.Path)
	sb.WriteByte(' ')
	sb.WriteString(r.URL.RawQuery)
	for _, vals := range r.Header {
		for _, v := range vals {
			sb.WriteByte(' ')
			sb.WriteString(v)
		}
	}
	hay := sb.String()
	for _, c := range cans {
		if strings.Contains(hay, c.Token) {
			return c, true
		}
	}
	// Scan a bounded prefix of the body too (restoring it for the proxy), so a
	// canary planted as a POST field / JSON value is caught.
	if body := e.readBodyPrefix(r); body != "" {
		for _, c := range cans {
			if strings.Contains(body, c.Token) {
				return c, true
			}
		}
	}
	return decoy.Canary{}, false
}

// maxBodyScan bounds how much request body is buffered for canary scanning.
const maxBodyScan = 64 * 1024

// readBodyPrefix reads up to maxBodyScan bytes of the body and restores the body
// (prefix + remaining stream) so the reverse proxy still forwards it intact.
func (e *Engine) readBodyPrefix(r *http.Request) string {
	if r.Body == nil || r.Body == http.NoBody {
		return ""
	}
	orig := r.Body
	prefix, _ := io.ReadAll(io.LimitReader(orig, maxBodyScan))
	r.Body = &replayBody{r: io.MultiReader(bytes.NewReader(prefix), orig), c: orig}
	return string(prefix)
}

// replayBody re-serves an already-read prefix followed by the rest of the
// original body, closing the original on Close.
type replayBody struct {
	r io.Reader
	c io.Closer
}

func (b *replayBody) Read(p []byte) (int, error) { return b.r.Read(p) }
func (b *replayBody) Close() error               { return b.c.Close() }

// clientIP returns the peer address, or the left-most X-Forwarded-For entry
// when the direct peer is a configured trusted proxy. By default no proxy is
// trusted, so a client can never spoof its IP via a forged XFF header.
func (e *Engine) clientIP(r *http.Request, st *engineState) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	if len(st.trusted) == 0 {
		return host
	}
	peer := net.ParseIP(host)
	if peer == nil || !isTrusted(st.trusted, peer) {
		return host
	}
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return host
	}
	first := xff
	if i := strings.IndexByte(xff, ','); i >= 0 {
		first = xff[:i]
	}
	first = strings.TrimSpace(first)
	if net.ParseIP(first) != nil {
		return first
	}
	return host
}

func isTrusted(trusted []*net.IPNet, ip net.IP) bool {
	for _, n := range trusted {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// headerSubset extracts the lower-cased headers the classifier reasons about.
func headerSubset(r *http.Request) map[string]string {
	h := map[string]string{}
	for _, name := range []string{"Sec-Fetch-Mode", "Sec-Ch-Ua", "Accept-Language", "Referer", "Accept", "Accept-Encoding"} {
		if v := r.Header.Get(name); v != "" {
			h[toLower(name)] = v
		}
	}
	return h
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
