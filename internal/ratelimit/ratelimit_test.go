package ratelimit

import (
	"testing"
	"time"
)

func TestDisabledAllowsEverything(t *testing.T) {
	var l *Limiter
	if !l.Allow("x") {
		t.Fatal("nil limiter must allow")
	}
	if !New(0, 1).Allow("x") {
		t.Fatal("rps<=0 must allow")
	}
}

func TestBurstThenThrottle(t *testing.T) {
	now := time.Unix(0, 0)
	l := New(1, 3) // 1 rps, burst 3
	l.now = func() time.Time { return now }

	// burst of 3 allowed immediately
	for i := 0; i < 3; i++ {
		if !l.Allow("s") {
			t.Fatalf("burst request %d should be allowed", i)
		}
	}
	// 4th denied (no time passed)
	if l.Allow("s") {
		t.Fatal("4th request within the same instant should be denied")
	}
	// after 1s, one token refilled
	now = now.Add(time.Second)
	if !l.Allow("s") {
		t.Fatal("after 1s one request should be allowed")
	}
	if l.Allow("s") {
		t.Fatal("only one token should have refilled")
	}
}

func TestPerKeyIsolation(t *testing.T) {
	now := time.Unix(0, 0)
	l := New(1, 1)
	l.now = func() time.Time { return now }
	if !l.Allow("a") || !l.Allow("b") {
		t.Fatal("different keys have independent buckets")
	}
	if l.Allow("a") {
		t.Fatal("key a should be throttled after its one token")
	}
}
