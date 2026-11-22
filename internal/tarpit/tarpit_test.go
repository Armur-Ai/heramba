package tarpit

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDripWritesAllBytes(t *testing.T) {
	tp := New(Config{ChunkSize: 1, Interval: time.Millisecond})
	var buf bytes.Buffer
	body := []byte("hello world")
	n, err := tp.Drip(context.Background(), &buf, body)
	if err != nil {
		t.Fatalf("Drip: %v", err)
	}
	if n != len(body) || buf.String() != "hello world" {
		t.Fatalf("got %d %q", n, buf.String())
	}
}

func TestDripTimeScalesWithInterval(t *testing.T) {
	body := []byte("abcdef") // 6 chunks of 1 → 5 inter-chunk waits
	measure := func(interval time.Duration) time.Duration {
		tp := New(Config{ChunkSize: 1, Interval: interval})
		start := time.Now()
		if _, err := tp.Drip(context.Background(), new(bytes.Buffer), body); err != nil {
			t.Fatal(err)
		}
		return time.Since(start)
	}
	// 5 waits * interval is the floor; assert each measurement clears ~80% of it
	// (one-sided bound, not flaky upward).
	slow := measure(10 * time.Millisecond)
	if slow < 5*8*time.Millisecond {
		t.Fatalf("10ms interval drip too fast: %v", slow)
	}
	faster := measure(2 * time.Millisecond)
	if faster < 5*1*time.Millisecond {
		t.Fatalf("2ms interval drip implausibly fast: %v", faster)
	}
	if slow <= faster {
		t.Fatalf("slower interval should take longer: slow=%v faster=%v", slow, faster)
	}
}

func TestDripHonorsCancellationPromptly(t *testing.T) {
	tp := New(Config{ChunkSize: 1, Interval: time.Hour}) // would hang for ages
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	var buf bytes.Buffer
	start := time.Now()
	n, err := tp.Drip(ctx, &buf, []byte("abcdef"))
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("cancellation not prompt: %v", elapsed)
	}
	if n >= len([]byte("abcdef")) {
		t.Fatalf("should have stopped before writing everything, wrote %d", n)
	}
}

func TestDripMaxTotalBudget(t *testing.T) {
	tp := New(Config{ChunkSize: 1, Interval: 10 * time.Millisecond, MaxTotal: 25 * time.Millisecond})
	_, err := tp.Drip(context.Background(), new(bytes.Buffer), bytes.Repeat([]byte("x"), 100))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestDripChunkSize(t *testing.T) {
	tp := New(Config{ChunkSize: 4, Interval: 0})
	var buf bytes.Buffer
	body := []byte("0123456789") // 10 bytes, chunks of 4 → 4,4,2
	n, err := tp.Drip(context.Background(), &buf, body)
	if err != nil || n != 10 || buf.String() != "0123456789" {
		t.Fatalf("got n=%d buf=%q err=%v", n, buf.String(), err)
	}
}

func TestJitterWithinBounds(t *testing.T) {
	tp := NewSeeded(Config{Jitter: 100 * time.Millisecond}, 1)
	for i := 0; i < 1000; i++ {
		j := tp.jitter()
		if j < 0 || j >= 100*time.Millisecond {
			t.Fatalf("jitter out of bounds: %v", j)
		}
	}
	// Zero jitter config must be exactly zero.
	if NewSeeded(Config{}, 1).jitter() != 0 {
		t.Fatal("zero jitter must be 0")
	}
}

func TestServeHandlerFlushes(t *testing.T) {
	tp := New(Config{ChunkSize: 2, Interval: time.Millisecond})
	h := tp.ServeHandler("text/plain", func(r *http.Request) []byte {
		return []byte("tarpit-body")
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/slow", nil)
	h.ServeHTTP(rec, req)
	if rec.Body.String() != "tarpit-body" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "text/plain" {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}
}
