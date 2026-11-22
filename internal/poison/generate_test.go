package poison

import (
	"strings"
	"testing"
)

func newTestGen(t *testing.T, seed int64) *Generator {
	t.Helper()
	return NewGenerator(seed, newTestMarker(t))
}

func TestDeterministic(t *testing.T) {
	a := newTestGen(t, 7)
	b := newTestGen(t, 7)
	if a.UsersSQLDump(5) != b.UsersSQLDump(5) {
		t.Fatal("same seed must yield identical SQL dump")
	}
	if a.ConfigEnv() != b.ConfigEnv() {
		t.Fatal("same seed must yield identical config")
	}
	// Different seed should differ.
	c := newTestGen(t, 8)
	if a.ConfigEnv() == c.ConfigEnv() {
		t.Fatal("different seed should change output")
	}
}

func TestOrderStableAcrossKinds(t *testing.T) {
	g := newTestGen(t, 3)
	// Generating users first must not perturb the API-key stream.
	keysA := g.APIKeys(4)
	_ = g.Users(10)
	keysB := g.APIKeys(4)
	for i := range keysA {
		if keysA[i].Value != keysB[i].Value {
			t.Fatal("api-key stream shifted by unrelated generation")
		}
	}
}

func TestAllGeneratedValuesAreOurs(t *testing.T) {
	m := newTestMarker(t)
	g := NewGenerator(11, m)
	for _, s := range g.APIKeys(8) {
		if !m.IsOurs(s.Value) {
			t.Fatalf("api key not watermarked: %q", s.Value)
		}
	}
	for _, u := range g.Users(8) {
		if !m.IsOurs(u.APIKey) || !m.IsOurs(u.PasswordHash) {
			t.Fatalf("user secrets not watermarked: %+v", u)
		}
	}
	for _, v := range g.AllValues(5, 5) {
		if !m.IsOurs(v) {
			t.Fatalf("AllValues entry not watermarked: %q", v)
		}
	}
}

// realCredentialMarkers are prefixes/substrings of LIVE credential formats. A
// generated fake must never contain one, or a defender could mistake our bait
// for a real leaked secret (or an attacker's scanner could, poisoning THEM is
// fine, but mistaking ours for live on our side is not).
var realCredentialMarkers = []string{
	"AKIA", "ASIA", // AWS access key ids
	"sk_live_", "sk_test_", "rk_live_", "pk_live_", // Stripe
	"ghp_", "gho_", "ghs_", "github_pat_", // GitHub tokens
	"xoxb-", "xoxp-", "xoxa-", // Slack
	"AIza",                 // Google API key
	"ya29.",                // Google OAuth
	"-----BEGIN",           // PEM private keys
	"$2a$", "$2b$", "$2y$", // bcrypt
	"$argon2", // argon2
	"eyJ",     // JWT header
	"glpat-",  // GitLab PAT
}

func TestGeneratedSecretsAreInert(t *testing.T) {
	g := newTestGen(t, 123)
	corpus := g.UsersSQLDump(20) + "\n" + g.ConfigEnv() + "\n"
	for _, s := range g.APIKeys(20) {
		corpus += s.Value + "\n"
	}
	for _, marker := range realCredentialMarkers {
		if strings.Contains(corpus, marker) {
			t.Fatalf("generated bait contains a real credential marker %q — could be mistaken for live", marker)
		}
	}
}

func TestUsersSQLDumpShape(t *testing.T) {
	g := newTestGen(t, 1)
	dump := g.UsersSQLDump(3)
	if !strings.Contains(dump, "INSERT INTO `users`") {
		t.Fatalf("missing insert header:\n%s", dump)
	}
	if strings.Count(dump, "@example.test") != 3 {
		t.Fatalf("expected 3 reserved-TLD emails:\n%s", dump)
	}
	if !strings.HasSuffix(strings.TrimSpace(dump), ";") {
		t.Fatalf("dump should end with ';':\n%s", dump)
	}
}

func TestConfigEnvShape(t *testing.T) {
	g := newTestGen(t, 1)
	cfg := g.ConfigEnv()
	if !strings.Contains(cfg, "DB_PASSWORD=") || !strings.Contains(cfg, "APP_ENV=production") {
		t.Fatalf("config missing expected keys:\n%s", cfg)
	}
	// Every value that looks secret must be watermarked.
	m := newTestMarker(t)
	for _, line := range strings.Split(cfg, "\n") {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := kv[0], kv[1]
		if strings.Contains(key, "PASSWORD") || strings.Contains(key, "KEY") ||
			strings.Contains(key, "TOKEN") || strings.Contains(key, "SECRET") {
			if !m.IsOurs(val) {
				t.Fatalf("secret config value not watermarked: %s", line)
			}
		}
	}
}
