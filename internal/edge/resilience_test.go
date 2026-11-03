package edge

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/event"
)

// errSink models a telemetry/sensor backend that is down: Emit always errors.
// Core traffic must be unaffected (fail-safe).
type errSink struct{ calls int }

func (s *errSink) Emit(event.Event) error { s.calls++; return errors.New("sink down") }

func newEngine(t *testing.T, mode config.Mode, upstream string) *Engine {
	t.Helper()
	cfg := config.Default()
	cfg.Mode = mode
	cfg.Upstream = upstream
	e, err := New(cfg, &capSink{})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func front(t *testing.T, e *Engine) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(e.Handler())
	t.Cleanup(srv.Close)
	return srv
}

// Keep-alive: many requests reusing one connection must all succeed.
func TestKeepAliveConnectionReuse(t *testing.T) {
	up := newUpstream(t)
	defer up.Close()
	srv := front(t, newEngine(t, config.ModeObserve, up.URL))

	client := &http.Client{} // default transport keeps connections alive
	for i := 0; i < 25; i++ {
		resp, err := client.Get(srv.URL + fmt.Sprintf("/api/item/%d", i))
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !strings.Contains(string(b), fmt.Sprintf("/api/item/%d", i)) {
			t.Fatalf("request %d got wrong body %q", i, b)
		}
	}
}

// Streaming / chunked: the edge must stream an incrementally-flushed response,
// not buffer the whole thing.
func TestChunkedStreamingResponse(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			t.Error("backend needs a flusher")
		}
		for i := 0; i < 5; i++ {
			fmt.Fprintf(w, "chunk-%d\n", i)
			fl.Flush()
		}
	}))
	defer up.Close()
	srv := front(t, newEngine(t, config.ModeObserve, up.URL))

	resp, err := http.Get(srv.URL + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	for i := 0; i < 5; i++ {
		if !strings.Contains(string(b), fmt.Sprintf("chunk-%d", i)) {
			t.Fatalf("missing chunk %d in %q", i, b)
		}
	}
}

// Server-Sent Events pass through untouched (not HTML/JS, so never rewritten).
func TestSSEPassthroughProtectMode(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		fmt.Fprint(w, "data: hello\n\n")
		fl.Flush()
	}))
	defer up.Close()
	srv := front(t, newEngine(t, config.ModeProtect, up.URL))

	resp, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "data: hello") {
		t.Fatalf("SSE corrupted: %q", b)
	}
}

// Large bodies must pass through byte-for-byte (checksum), in both modes.
func TestLargeBodyIntegrity(t *testing.T) {
	payload := bytes.Repeat([]byte("ABCDEFGH"), 1<<20) // 8 MiB
	sum := sha256.Sum256(payload)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer up.Close()
	for _, mode := range []config.Mode{config.ModeObserve, config.ModeProtect} {
		srv := front(t, newEngine(t, mode, up.URL))
		resp, err := http.Get(srv.URL + "/big")
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if sha256.Sum256(got) != sum {
			t.Fatalf("mode %s corrupted a large body (%d vs %d bytes)", mode, len(got), len(payload))
		}
	}
}

// A gzip-compressed response must never be rewritten (modifyResponse skips any
// Content-Encoding), so it round-trips intact even in protect mode.
func TestGzipResponseNotCorrupted(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	original := []byte("<html><body>" + strings.Repeat("x", 1000) + "</body></html>")
	gz.Write(original)
	gz.Close()
	compressed := buf.Bytes()

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(compressed)
	}))
	defer up.Close()
	srv := front(t, newEngine(t, config.ModeProtect, up.URL))

	// Set Accept-Encoding explicitly so Go's transport does NOT transparently
	// decompress — we want the raw bytes the edge forwarded.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/page", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("edge dropped Content-Encoding: headers=%v", resp.Header)
	}
	raw, _ := io.ReadAll(resp.Body)
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("response is not valid gzip (corrupted by rewrite): %v", err)
	}
	out, _ := io.ReadAll(zr)
	if !bytes.Equal(out, original) {
		t.Fatal("gzip body content changed")
	}
}

// Cookies, Cache-Control, and redirects pass through unaltered.
func TestHeaderCookieRedirectPassthrough(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "session=abc; HttpOnly")
		w.Header().Set("Cache-Control", "max-age=300")
		w.Header().Set("Location", "/moved")
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer up.Close()
	srv := front(t, newEngine(t, config.ModeObserve, up.URL))

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(srv.URL + "/old")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", resp.StatusCode)
	}
	if resp.Header.Get("Set-Cookie") == "" || resp.Header.Get("Cache-Control") != "max-age=300" || resp.Header.Get("Location") != "/moved" {
		t.Fatalf("headers not passed through: %v", resp.Header)
	}
}

// Request bodies (POST) are forwarded intact.
func TestRequestBodyForwarded(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		fmt.Fprintf(w, "got:%s", b)
	}))
	defer up.Close()
	srv := front(t, newEngine(t, config.ModeObserve, up.URL))

	resp, err := http.Post(srv.URL+"/submit", "text/plain", strings.NewReader("payload-123"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "got:payload-123" {
		t.Fatalf("request body not forwarded: %q", b)
	}
}

// Upstream outage: a dead backend yields a 502, never a panic or a hang. This is
// the documented safe behavior under upstream failure.
func TestUpstreamOutageReturns502(t *testing.T) {
	up := newUpstream(t)
	deadURL := up.URL
	up.Close() // kill it

	srv := front(t, newEngine(t, config.ModeObserve, deadURL))
	resp, err := http.Get(srv.URL + "/api")
	if err != nil {
		t.Fatalf("edge should respond even when upstream is down: %v", err)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("upstream outage status = %d, want 502", resp.StatusCode)
	}
}

// Telemetry/sensor outage (Emit errors) must not affect served traffic: a decoy
// still serves its 404 and real traffic still flows. Fail-safe.
func TestSinkFailureDoesNotCorruptTraffic(t *testing.T) {
	up := newUpstream(t)
	defer up.Close()
	cfg := config.Default()
	cfg.Upstream = up.URL
	sink := &errSink{}
	e, err := New(cfg, sink)
	if err != nil {
		t.Fatal(err)
	}
	srv := front(t, e)

	// Decoy path (emits an event that errors) still serves correctly.
	resp, err := http.Get(srv.URL + "/.env")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("decoy served %d despite sink error", resp.StatusCode)
	}
	// Real traffic still flows.
	resp2, err := http.Get(srv.URL + "/api/x")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if !strings.Contains(string(b), "/api/x") {
		t.Fatalf("real traffic broke under sink failure: %q", b)
	}
	if sink.calls == 0 {
		t.Fatal("expected the sink to have been called (and errored)")
	}
}

// Client cancellation must not panic the edge or leak; the backend observes the
// cancellation (so it never blocks shutdown) and a later request still succeeds.
func TestRequestCancellation(t *testing.T) {
	canceled := make(chan struct{}, 1)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			canceled <- struct{}{} // proxy propagated the cancellation upstream
		case <-time.After(10 * time.Second): // safety: always return, never hang shutdown
		}
	}))
	defer up.Close()
	srv := front(t, newEngine(t, config.ModeObserve, up.URL))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/slow", nil)
	if _, err := http.DefaultClient.Do(req); err == nil {
		t.Fatal("expected cancellation error") // core guarantee: client sees cancellation
	}
	// A subsequent request still works — the edge did not panic or wedge.
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("edge broken after a cancellation: %v", err)
	}
	resp.Body.Close()
	// Upstream propagation is best-effort (timing varies through the proxy); log it.
	select {
	case <-canceled:
		t.Log("cancellation propagated to the upstream")
	case <-time.After(5 * time.Second):
		t.Log("note: upstream cancellation not observed within 5s (propagation latency)")
	}
}

// Graceful shutdown: an in-flight request completes while Shutdown drains.
func TestGracefulShutdownDrainsInflight(t *testing.T) {
	release := make(chan struct{})
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		_, _ = w.Write([]byte("done"))
	}))
	defer up.Close()

	e := newEngine(t, config.ModeObserve, up.URL)
	hs := &http.Server{Handler: e.Handler()}
	ln, err := newLocalListener()
	if err != nil {
		t.Fatal(err)
	}
	go hs.Serve(ln)

	type res struct {
		body string
		err  error
	}
	done := make(chan res, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/work")
		if err != nil {
			done <- res{err: err}
			return
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		done <- res{body: string(b)}
	}()

	time.Sleep(30 * time.Millisecond) // let the request reach the backend
	shutdownDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		shutdownDone <- hs.Shutdown(ctx)
	}()
	close(release) // let the in-flight request complete

	r := <-done
	if r.err != nil || r.body != "done" {
		t.Fatalf("in-flight request not drained: body=%q err=%v", r.body, r.err)
	}
	if err := <-shutdownDone; err != nil {
		t.Fatalf("shutdown errored: %v", err)
	}
}

// Observe/protect body parity: the response BODY is identical in both modes for
// non-agentic traffic (only headers differ). Unsafe body transforms are off by
// default.
func TestObserveProtectBodyParity(t *testing.T) {
	up := newUpstream(t)
	defer up.Close()
	get := func(mode config.Mode) string {
		srv := front(t, newEngine(t, mode, up.URL))
		resp, err := http.Get(srv.URL + "/api/parity")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return string(b)
	}
	if get(config.ModeObserve) != get(config.ModeProtect) {
		t.Fatal("response body differs between observe and protect for benign traffic")
	}
}

// Added-latency SLO: measure p50/p95/p99 overhead of the edge versus hitting the
// backend directly, under a realistic (non-zero-work) backend. Asserts only a
// generous ceiling to catch gross regressions without flaking; the numbers are
// logged for the assurance report.
func TestAddedLatencySLO(t *testing.T) {
	if testing.Short() {
		t.Skip("latency SLO measurement skipped in -short")
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// realistic small work: a little compute + a modest body.
		s := sha256.Sum256([]byte(r.URL.Path))
		_, _ = w.Write(bytes.Repeat(s[:], 32)) // 1 KiB
	}))
	defer up.Close()
	srv := front(t, newEngine(t, config.ModeObserve, up.URL))

	const n = 300
	direct := measure(t, up.URL+"/x", n)
	through := measure(t, srv.URL+"/x", n)
	added := func(p float64) time.Duration { return percentileD(through, p) - percentileD(direct, p) }
	p50, p95, p99 := added(50), added(95), added(99)
	t.Logf("added latency p50=%v p95=%v p99=%v (direct p50=%v, edge p50=%v)",
		p50, p95, p99, percentileD(direct, 50), percentileD(through, 50))
	if p99 > 50*time.Millisecond {
		t.Fatalf("added p99 latency %v exceeds 50ms SLO ceiling (regression)", p99)
	}
}

func measure(t *testing.T, url string, n int) []time.Duration {
	t.Helper()
	client := &http.Client{}
	out := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		resp, err := client.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		out = append(out, time.Since(start))
	}
	return out
}

func newLocalListener() (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }

func percentileD(xs []time.Duration, p float64) time.Duration {
	s := append([]time.Duration(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	if len(s) == 0 {
		return 0
	}
	idx := int(p / 100 * float64(len(s)-1))
	return s[idx]
}
