package deception

import (
	"bytes"
	"net/http"

	"github.com/Armur-Ai/heramba/internal/tarpit"
)

// SlowApp wraps a deception App so its responses are dripped out slowly through
// a tarpit — turning the synthetic honeypot into a time sink for a confirmed-bad
// source without changing a single handler. The App computes its answer in full
// (so the synthetic world stays coherent and every interaction is still
// recorded), then SlowApp streams those bytes back on the tarpit schedule,
// honoring request-context cancellation.
//
// This is defensive: it only slows the attacker's own request to our honeypot.
type SlowApp struct {
	App *App
	tp  *tarpit.Tarpit
}

// NewSlowApp wraps app with a tarpit built from cfg.
func NewSlowApp(app *App, cfg tarpit.Config) *SlowApp {
	return &SlowApp{App: app, tp: tarpit.New(cfg)}
}

// NewSlowAppTarpit wraps app with a caller-supplied Tarpit, so several wrappers
// (or a per-source escalator) can share drip configuration.
func NewSlowAppTarpit(app *App, tp *tarpit.Tarpit) *SlowApp {
	return &SlowApp{App: app, tp: tp}
}

// ServeHTTP runs the wrapped App against an in-memory buffer, then drips the
// captured response (status, headers, body) back to the real client slowly.
func (s *SlowApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cap := &captureWriter{header: http.Header{}, status: http.StatusOK}
	s.App.ServeHTTP(cap, r)

	for k, vs := range cap.header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(cap.status)
	_, _ = s.tp.Drip(r.Context(), w, cap.buf.Bytes())
}

// captureWriter is an http.ResponseWriter that buffers the response instead of
// sending it, so a wrapper can re-emit it on its own schedule.
type captureWriter struct {
	header      http.Header
	status      int
	wroteHeader bool
	buf         bytes.Buffer
}

func (c *captureWriter) Header() http.Header { return c.header }

func (c *captureWriter) WriteHeader(status int) {
	if c.wroteHeader {
		return
	}
	c.status = status
	c.wroteHeader = true
}

func (c *captureWriter) Write(b []byte) (int, error) {
	c.wroteHeader = true
	return c.buf.Write(b)
}

// Body returns the captured bytes (useful for tests/telemetry).
func (c *captureWriter) Body() []byte { return c.buf.Bytes() }
