package tarpit

import (
	"sync"
	"testing"
	"time"
)

func TestLinearEscalation(t *testing.T) {
	e := NewEscalator(10*time.Millisecond, 100*time.Millisecond, Linear)
	want := []time.Duration{10, 20, 30, 40, 50}
	for i, w := range want {
		got := e.Offense("1.2.3.4")
		if got != w*time.Millisecond {
			t.Fatalf("offense %d: got %v want %v", i+1, got, w*time.Millisecond)
		}
	}
}

func TestExponentialEscalation(t *testing.T) {
	e := NewEscalator(5*time.Millisecond, time.Second, Exponential)
	want := []time.Duration{5, 10, 20, 40, 80}
	for i, w := range want {
		got := e.Offense("bad")
		if got != w*time.Millisecond {
			t.Fatalf("offense %d: got %v want %v", i+1, got, w*time.Millisecond)
		}
	}
}

func TestEscalationCapped(t *testing.T) {
	for _, g := range []Growth{Linear, Exponential} {
		e := NewEscalator(10*time.Millisecond, 50*time.Millisecond, g)
		var last time.Duration
		for i := 0; i < 100; i++ {
			last = e.Offense("x")
			if last > 50*time.Millisecond {
				t.Fatalf("growth %d exceeded cap: %v", g, last)
			}
		}
		if last != 50*time.Millisecond {
			t.Fatalf("growth %d should saturate at cap, got %v", g, last)
		}
	}
}

func TestNoCap(t *testing.T) {
	e := NewEscalator(time.Millisecond, 0, Linear) // 0 max = uncapped
	var d time.Duration
	for i := 0; i < 5; i++ {
		d = e.Offense("y")
	}
	if d != 5*time.Millisecond {
		t.Fatalf("uncapped linear 5th = %v", d)
	}
}

func TestDelayAndReset(t *testing.T) {
	e := NewEscalator(10*time.Millisecond, 0, Linear)
	if e.Delay("z") != 0 || e.Count("z") != 0 {
		t.Fatal("unknown source should be zero")
	}
	e.Offense("z")
	e.Offense("z")
	if e.Delay("z") != 20*time.Millisecond {
		t.Fatalf("delay after 2 offenses = %v", e.Delay("z"))
	}
	if e.Count("z") != 2 {
		t.Fatalf("count = %d", e.Count("z"))
	}
	e.Reset("z")
	if e.Delay("z") != 0 || e.Count("z") != 0 {
		t.Fatal("reset should clear")
	}
}

func TestConfigFor(t *testing.T) {
	e := NewEscalator(10*time.Millisecond, 0, Linear)
	e.Offense("s")
	e.Offense("s")
	cfg := e.ConfigFor("s", 8, 3*time.Millisecond)
	if cfg.Interval != 20*time.Millisecond || cfg.ChunkSize != 8 || cfg.Jitter != 3*time.Millisecond {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestEscalatorConcurrent(t *testing.T) {
	e := NewEscalator(time.Microsecond, 0, Linear)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				e.Offense("shared")
			}
		}()
	}
	wg.Wait()
	if e.Count("shared") != 5000 {
		t.Fatalf("race lost counts: %d", e.Count("shared"))
	}
}
