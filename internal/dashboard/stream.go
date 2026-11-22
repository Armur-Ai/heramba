package dashboard

import (
	"encoding/json"
	"net/http"
	"time"
)

// streamHeartbeat is how often the stream emits an SSE comment when no frames
// have flowed, keeping intermediaries from idling the connection shut.
const streamHeartbeat = 20 * time.Second

// handleStream serves GET /api/stream as a Server-Sent Events feed. It
// subscribes to the broker for the life of the request, relays each published
// frame as `data: {json}\n\n`, and sends a heartbeat comment during quiet
// periods. The subscription is torn down the moment the request context is
// cancelled (client disconnect), so no goroutine or subscriber leaks.
func (s *server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // disable proxy buffering (nginx)
	w.WriteHeader(http.StatusOK)

	sub := s.broker.subscribe()
	defer s.broker.unsubscribe(sub)

	// Opening comment + a priming metrics frame so a fresh client renders at
	// once instead of waiting for the next published frame.
	_, _ = w.Write([]byte(": connected\n\n"))
	writeFrame(w, StreamFrame{Type: "metrics", Metrics: ptr(s.metricsPayload())})
	flusher.Flush()

	ticker := time.NewTicker(streamHeartbeat)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case f, ok := <-sub.ch:
			if !ok {
				return // broker closed
			}
			writeFrame(w, f)
			flusher.Flush()
		case <-ticker.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// writeFrame encodes a frame as one SSE `data:` event. Encoding errors are
// swallowed (a malformed frame is skipped rather than killing the stream).
func writeFrame(w http.ResponseWriter, f StreamFrame) {
	b, err := json.Marshal(f)
	if err != nil {
		return
	}
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(b)
	_, _ = w.Write([]byte("\n\n"))
}

// ptr returns a pointer to v (local helper for optional frame fields).
func ptr[T any](v T) *T { return &v }
