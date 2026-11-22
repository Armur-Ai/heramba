package main

import (
	"strings"
	"testing"

	"github.com/Armur-Ai/heramba/internal/decoy"
)

func TestVerifyDecoy(t *testing.T) {
	r := decoy.NewRegistry()
	p, _ := decoy.GetPreset("rails")
	r.ApplyPreset(p)
	c, err := r.Canaries().Mint("test-plant")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		value   string
		matched bool
		substr  string
	}{
		{"default decoy path", "/.env", true, "decoy path"},
		{"default decoy zone", "/.git/HEAD", true, "decoy path"},
		{"preset lure", "/config/master.key", true, "decoy path"},
		{"preset seed canary", "0123456789abcdef0123456789abcdef", true, "watched canary"},
		{"minted canary", c.Token, true, "watched canary"},
		{"canary-shaped but unwatched", "hrmb_" + strings.Repeat("a", 32), false, "NOT currently watched"},
		{"plain unknown string", "/about", false, "not a known"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			desc, matched := verifyDecoy(r, tc.value)
			if matched != tc.matched {
				t.Errorf("matched = %v, want %v (desc=%q)", matched, tc.matched, desc)
			}
			if !strings.Contains(desc, tc.substr) {
				t.Errorf("desc = %q, want substring %q", desc, tc.substr)
			}
		})
	}
}
