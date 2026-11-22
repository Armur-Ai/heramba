package decoy

import (
	"strings"
	"testing"
)

func TestDeriveBodyCanaryDeterministicAndSalted(t *testing.T) {
	a := DeriveBodyCanary("password", "salt1")
	b := DeriveBodyCanary("password", "salt1")
	if a != b {
		t.Fatalf("derivation must be deterministic: %q != %q", a, b)
	}
	if !strings.HasPrefix(a, "hrmb_") {
		t.Errorf("token should carry the canary prefix: %q", a)
	}
	if !Looks(a) {
		t.Errorf("derived token should look like a canary: %q", a)
	}
	// Different salt => different token (unguessable without the salt).
	if c := DeriveBodyCanary("password", "salt2"); c == a {
		t.Error("different salt must change the token")
	}
	// Different field => different token.
	if d := DeriveBodyCanary("api_key", "salt1"); d == a {
		t.Error("different field must change the token")
	}
}

func TestBodyCanariesDedupAndSort(t *testing.T) {
	cans := BodyCanaries([]string{"token", "api_key", "token", ""}, "s")
	if len(cans) != 2 {
		t.Fatalf("expected 2 unique canaries, got %d (%+v)", len(cans), cans)
	}
	if cans[0].Field != "api_key" || cans[1].Field != "token" {
		t.Errorf("canaries should be sorted by field: %+v", cans)
	}
	// Token matches the standalone derivation.
	if cans[1].Token != DeriveBodyCanary("token", "s") {
		t.Error("BodyCanaries token must match DeriveBodyCanary")
	}
}

func TestWatchBodyCanariesDetected(t *testing.T) {
	r := NewRegistry()
	cans := BodyCanaries([]string{"password", "api_key"}, "secret-salt")
	r.WatchBodyCanaries(cans)

	// The derived token is now a watched canary.
	tok := DeriveBodyCanary("password", "secret-salt")
	c, ok := r.Canaries().Check(tok)
	if !ok {
		t.Fatalf("derived body canary %q should be watched", tok)
	}
	if !strings.Contains(c.Label, "password") {
		t.Errorf("canary label should name the field, got %q", c.Label)
	}
	// A value that was never derived must not be watched.
	if _, ok := r.Canaries().Check("hrmb_00000000000000000000000000000000"); ok {
		t.Error("an unrelated token must not be watched")
	}
}
