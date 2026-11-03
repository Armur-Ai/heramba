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
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/Armur-Ai/heramba/internal/alert"
	"github.com/Armur-Ai/heramba/internal/classify"
	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/decoy"
	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/signal"
)

// Engine is the configured edge runtime. Build with New, serve with ServeHTTP.
type Engine struct {
	cfg   config.Config
	reg   *decoy.Registry
	cls   *classify.Classifier
	norm  *signal.Normalizer // protect mode only
	jit   *signal.Jitter     // protect-mode tarpit for flagged sources
	sink  alert.Sink
	proxy *httputil.ReverseProxy // nil in honeypot-only (no upstream)
}

// New assembles an edge Engine from config. In PROTECT mode it enables response
// normalization and a tarpit. An upstream (reverse-proxy target) is optional in
// observe mode (no upstream => honeypot-only: decoys + 404).
func New(cfg config.Config, sink alert.Sink) (*Engine, error) {
	if sink == nil {
		sink = alert.Discard
	}
	e := &Engine{cfg: cfg, reg: decoy.NewRegistry(cfg.Decoys.Paths...), cls: classify.New(), sink: sink}
	if cfg.Mode == config.ModeProtect {
		e.norm = signal.DefaultNormalizer()
		e.jit = signal.NewJitter(250*time.Millisecond, 1*time.Second)
	}
	if cfg.Upstream != "" {
		u, err := url.Parse(cfg.Upstream)
		if err != nil {
			return nil, fmt.Errorf("edge: bad upstream %q: %w", cfg.Upstream, err)
		}
		rp := httputil.NewSingleHostReverseProxy(u)
		if e.norm != nil {
			norm := e.norm
			rp.ModifyResponse = func(resp *http.Response) error {
				norm.Apply(resp.Header)
				return nil
			}
		}
		e.proxy = rp
	}
	return e, nil
}

// Handler returns the edge as an http.Handler.
func (e *Engine) Handler() http.Handler { return e }

func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)

	// 1. Classify the source (both modes — detection is the point of observe).
	v := e.cls.Classify(classify.Observation{
		Source: ip, Method: r.Method, Path: r.URL.Path,
		UserAgent: r.UserAgent(), Headers: headerSubset(r), Time: time.Now().UTC(),
	})
	if v.Label == "agentic-suspected" {
		e.sink.Emit(event.New(event.KindAgenticTraffic, event.SeverityMedium, ip, r.URL.Path,
			fmt.Sprintf("agentic-suspected (score %.2f)", v.Score)).
			WithLabel("label", v.Label))
	}

	// 2. Deception surface: a decoy hit is zero-FP; the edge serves it.
	if _, ok := e.reg.IsDecoy(r.URL.Path); ok {
		if ev, hit := e.reg.InspectPath(r.Method, r.URL.Path, ip); hit {
			e.sink.Emit(ev)
		}
		e.serveNeutral(w)
		return
	}

	// 3. Real application traffic.
	if e.cfg.Mode == config.ModeProtect && v.Label == "agentic-suspected" && e.jit != nil {
		e.jit.Sleep(r.Context()) // tarpit flagged sources
	}
	if e.proxy != nil {
		e.proxy.ServeHTTP(w, r)
		return
	}
	// Honeypot-only deployment (no upstream): everything else is a neutral 404.
	e.serveNeutral(w)
}

// serveNeutral writes a minimal 404 with normalized headers (protect mode),
// indistinguishable from a genuinely missing path.
func (e *Engine) serveNeutral(w http.ResponseWriter) {
	if e.norm != nil {
		e.norm.Apply(w.Header())
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("404 page not found\n"))
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// headerSubset extracts the lower-cased headers the classifier reasons about.
func headerSubset(r *http.Request) map[string]string {
	h := map[string]string{}
	for _, name := range []string{"Sec-Fetch-Mode", "Accept-Language", "Referer"} {
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
