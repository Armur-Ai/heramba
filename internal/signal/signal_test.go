package signal

import (
	"net/http"
	"testing"
	"time"
)

func TestNormalizerStripsLeakyHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("X-Powered-By", "PHP/8.1.2")
	h.Set("Server", "nginx/1.25.1")
	h.Set("Content-Type", "application/json")
	DefaultNormalizer().Apply(h)

	if h.Get("X-Powered-By") != "" {
		t.Fatal("X-Powered-By should be stripped")
	}
	if h.Get("Server") != "heramba" {
		t.Fatalf("Server should be normalized, got %q", h.Get("Server"))
	}
	if h.Get("Content-Type") != "application/json" {
		t.Fatal("non-leaky header must be preserved")
	}
}

func TestJitterDelayWithinBounds(t *testing.T) {
	j := NewJitter(10*time.Millisecond, 50*time.Millisecond)
	for i := 0; i < 1000; i++ {
		d := j.Delay()
		if d < 10*time.Millisecond || d > 50*time.Millisecond {
			t.Fatalf("delay %v out of [10ms,50ms]", d)
		}
	}
}

func TestZeroJitterNoDelay(t *testing.T) {
	var j *Jitter
	if j.Delay() != 0 {
		t.Fatal("nil jitter must add no delay")
	}
	if NewJitter(0, 0).Delay() != 0 {
		t.Fatal("zero jitter must add no delay")
	}
}

func TestJitterClampsInvertedRange(t *testing.T) {
	j := NewJitter(20*time.Millisecond, 5*time.Millisecond) // max<min
	if j.Max != j.Min {
		t.Fatalf("inverted range should clamp max to min, got min=%v max=%v", j.Min, j.Max)
	}
}
