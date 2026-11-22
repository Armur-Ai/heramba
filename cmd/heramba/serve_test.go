package main

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDrainFlipsReadyAndWaits(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)
	var slept time.Duration
	drain(&ready, 3*time.Second, func(d time.Duration) { slept = d })

	if ready.Load() {
		t.Error("drain must flip ready to false")
	}
	if slept != 3*time.Second {
		t.Errorf("drain should wait the window, slept %s want 3s", slept)
	}
}

func TestDrainZeroWindowDoesNotSleep(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)
	called := false
	drain(&ready, 0, func(time.Duration) { called = true })

	if ready.Load() {
		t.Error("drain must flip ready to false even with no window")
	}
	if called {
		t.Error("drain with zero window must not sleep")
	}
}

func TestDrainNilReadyIsSafe(t *testing.T) {
	called := false
	drain(nil, 1*time.Second, func(time.Duration) { called = true })
	if !called {
		t.Error("drain with a window should still sleep even if ready is nil")
	}
}
