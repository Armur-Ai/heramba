package deception

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/tarpit"
)

func testApp() *App {
	return NewApp(GenerateWorld(1, 4, 3, 5))
}

func TestSlowAppSameBodyAsApp(t *testing.T) {
	app := testApp()
	// Plain app response for an unauth /api/me.
	recPlain := httptest.NewRecorder()
	app.ServeHTTP(recPlain, httptest.NewRequest("GET", "/api/me", nil))

	slow := NewSlowApp(testApp(), tarpit.Config{ChunkSize: 4, Interval: time.Millisecond})
	recSlow := httptest.NewRecorder()
	slow.ServeHTTP(recSlow, httptest.NewRequest("GET", "/api/me", nil))

	if recSlow.Code != recPlain.Code {
		t.Fatalf("status %d != %d", recSlow.Code, recPlain.Code)
	}
	if strings.TrimSpace(recSlow.Body.String()) != strings.TrimSpace(recPlain.Body.String()) {
		t.Fatalf("slow body %q != plain %q", recSlow.Body.String(), recPlain.Body.String())
	}
	// Headers the app set (content-type, session) must survive the wrapper.
	if recSlow.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content-type lost: %q", recSlow.Header().Get("Content-Type"))
	}
	if recSlow.Header().Get("X-Deception-Session") == "" {
		t.Fatal("session header lost through slow wrapper")
	}
}

func TestSlowAppActuallySlow(t *testing.T) {
	slow := NewSlowApp(testApp(), tarpit.Config{ChunkSize: 1, Interval: 2 * time.Millisecond})
	rec := httptest.NewRecorder()
	start := time.Now()
	slow.ServeHTTP(rec, httptest.NewRequest("GET", "/api/me", nil))
	elapsed := time.Since(start)
	// The 401 JSON body is dozens of bytes; at 2ms/byte it must take real time.
	if elapsed < 20*time.Millisecond {
		t.Fatalf("slow app returned too fast: %v (body %d bytes)", elapsed, rec.Body.Len())
	}
}

func TestSlowAppHonorsCancellation(t *testing.T) {
	slow := NewSlowApp(testApp(), tarpit.Config{ChunkSize: 1, Interval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/api/me", nil).WithContext(ctx)
	go func() {
		time.Sleep(15 * time.Millisecond)
		cancel()
	}()
	rec := httptest.NewRecorder()
	start := time.Now()
	slow.ServeHTTP(rec, req)
	if time.Since(start) > time.Second {
		t.Fatalf("cancellation not honored: took %v", time.Since(start))
	}
}

func TestCaptureWriterDefaults(t *testing.T) {
	c := &captureWriter{header: make(map[string][]string), status: 200}
	// Writing without an explicit WriteHeader keeps the 200 default.
	_, _ = c.Write([]byte("hello"))
	if c.status != 200 || string(c.Body()) != "hello" {
		t.Fatalf("status=%d body=%q", c.status, c.Body())
	}
	// A later WriteHeader after Write is ignored (like the real ResponseWriter).
	c.WriteHeader(500)
	if c.status != 200 {
		t.Fatalf("WriteHeader after Write should be ignored, status=%d", c.status)
	}
}
