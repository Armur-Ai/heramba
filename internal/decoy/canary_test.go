package decoy

import "testing"

func TestMintUniqueAndCheckable(t *testing.T) {
	s := NewCanaryStore()
	c1, err := s.Mint("db-config")
	if err != nil {
		t.Fatal(err)
	}
	c2, _ := s.Mint("env-file")
	if c1.Token == c2.Token {
		t.Fatal("tokens must be unique")
	}
	if !Looks(c1.Token) {
		t.Fatalf("minted token should look like a canary: %q", c1.Token)
	}
	got, ok := s.Check(c1.Token)
	if !ok || got.Label != "db-config" {
		t.Fatalf("check failed: %+v ok=%v", got, ok)
	}
	if _, ok := s.Check("hrmb_not_a_real_token"); ok {
		t.Fatal("unknown token must not check out")
	}
}

func TestLooksRejectsNonCanary(t *testing.T) {
	for _, v := range []string{"", "password123", "hrmb_tooShort", "sk-1234567890abcdef1234567890abcdef"} {
		if Looks(v) {
			t.Fatalf("%q should not look like a canary", v)
		}
	}
}

func TestAllReturnsPlanted(t *testing.T) {
	s := NewCanaryStore()
	_, _ = s.Mint("a")
	_, _ = s.Mint("b")
	if len(s.All()) != 2 {
		t.Fatalf("want 2 planted, got %d", len(s.All()))
	}
}
