package poison

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

// Derived sub-seeds keep each dataset independent and order-stable: generating
// users never shifts the API-key stream, so the same seed always yields the
// same Nth record of a kind regardless of what else was generated.
const (
	saltKeys   = 0x51
	saltUsers  = 0x75
	saltConfig = 0xc0
)

// Generator produces deterministic, inert, watermarked fake intelligence. Given
// the same seed and marker it always yields byte-identical output, which makes
// poisoned responses reproducible for tests and replay. Safe for concurrent use
// (each call builds its own RNG).
type Generator struct {
	marker *Marker
	seed   int64
}

// NewGenerator returns a Generator seeded from seed, watermarking every secret
// value with m so exfiltrated data is always traceable back to us.
func NewGenerator(seed int64, m *Marker) *Generator {
	return &Generator{marker: m, seed: seed}
}

func (g *Generator) rng(salt int64) *rand.Rand {
	return rand.New(rand.NewSource(g.seed ^ salt))
}

func hexOf(rng *rand.Rand, n int) string {
	const h = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = h[rng.Intn(16)]
	}
	return string(b)
}

// Secret is one fake credential. Value is inert (no real vendor format) and
// always watermarked, so if it reappears anywhere we know it is ours and fake.
type Secret struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Value     string `json:"value"`
	Synthetic bool   `json:"synthetic"`
}

var secretNames = []string{
	"PAYMENTS_SERVICE_TOKEN", "INTERNAL_API_KEY", "ANALYTICS_WRITE_KEY",
	"QUEUE_SIGNING_SECRET", "REPORTING_ACCESS_TOKEN", "WEBHOOK_SHARED_SECRET",
	"CACHE_AUTH_TOKEN", "SEARCH_INDEX_KEY",
}

// fakeToken builds an inert, clearly non-vendor token body and watermarks it.
// The "tok_" prefix is generic — deliberately not AKIA/sk_live/ghp_/AIza/etc. —
// so it can never be mistaken for a live credential, yet reads as a real secret
// to an attacker scraping our honeypot.
func (g *Generator) fakeToken(rng *rand.Rand) string {
	return g.marker.Watermark("tok_" + hexOf(rng, 40))
}

// APIKeys returns n deterministic fake API-key secrets.
func (g *Generator) APIKeys(n int) []Secret {
	rng := g.rng(saltKeys)
	out := make([]Secret, 0, n)
	for i := 0; i < n; i++ {
		name := secretNames[i%len(secretNames)]
		if i >= len(secretNames) {
			name = fmt.Sprintf("%s_%d", name, i/len(secretNames))
		}
		out = append(out, Secret{
			Name:      name,
			Kind:      "api_key",
			Value:     g.fakeToken(rng),
			Synthetic: true,
		})
	}
	return out
}

// User is a fake user record. The password hash uses an inert "$fake$" scheme
// (never a real bcrypt/argon2/scrypt format) and is watermarked; the API key is
// watermarked. Emails use the RFC 2606 reserved .test TLD and are never real.
type User struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
	APIKey       string `json:"api_key"`
	Synthetic    bool   `json:"synthetic"`
}

var firstNames = []string{"alex", "sam", "jordan", "taylor", "casey", "morgan", "riley", "jamie", "devin", "quinn"}
var lastNames = []string{"ng", "patel", "garcia", "khan", "silva", "nguyen", "rossi", "haddad", "osei", "ivanov"}

// fakeHash builds an inert, clearly-fake password hash and watermarks it.
func (g *Generator) fakeHash(rng *rand.Rand) string {
	return g.marker.Watermark("$fake$1$" + hexOf(rng, 32))
}

// Users returns n deterministic fake user records.
func (g *Generator) Users(n int) []User {
	rng := g.rng(saltUsers)
	out := make([]User, 0, n)
	for i := 0; i < n; i++ {
		fn := firstNames[rng.Intn(len(firstNames))]
		ln := lastNames[rng.Intn(len(lastNames))]
		user := fmt.Sprintf("%s.%s%d", fn, ln, 100+i)
		out = append(out, User{
			ID:           1000 + i,
			Username:     user,
			Email:        user + "@example.test",
			PasswordHash: g.fakeHash(rng),
			APIKey:       g.fakeToken(rng),
			Synthetic:    true,
		})
	}
	return out
}

// UsersSQLDump renders n fake users as an INSERT dump — the classic artifact an
// attacker believes they have exfiltrated. Every secret column is watermarked.
func (g *Generator) UsersSQLDump(n int) string {
	var b strings.Builder
	b.WriteString("-- MySQL dump (synthetic honeypot export)\n")
	b.WriteString("INSERT INTO `users` (`id`,`username`,`email`,`password_hash`,`api_key`) VALUES\n")
	users := g.Users(n)
	for i, u := range users {
		sep := ","
		if i == len(users)-1 {
			sep = ";"
		}
		b.WriteString(fmt.Sprintf("  (%d,'%s','%s','%s','%s')%s\n",
			u.ID, sqlEscape(u.Username), sqlEscape(u.Email),
			sqlEscape(u.PasswordHash), sqlEscape(u.APIKey), sep))
	}
	return b.String()
}

// ConfigEnv renders a fake .env-style config blob full of watermarked secrets,
// plus some plausible non-secret settings for believability.
func (g *Generator) ConfigEnv() string {
	rng := g.rng(saltConfig)
	var b strings.Builder
	b.WriteString("# application config (synthetic)\n")
	b.WriteString("APP_ENV=production\n")
	b.WriteString("LOG_LEVEL=info\n")
	b.WriteString("DB_HOST=db.internal.example.test\n")
	b.WriteString("DB_USER=app_ro\n")
	b.WriteString("DB_PASSWORD=" + g.marker.Watermark("pw_"+hexOf(rng, 24)) + "\n")
	for _, name := range secretNames[:5] {
		b.WriteString(name + "=" + g.fakeToken(rng) + "\n")
	}
	return b.String()
}

// AllValues returns every watermarked secret value this generator would produce
// for the given counts, sorted — handy to register them as canaries to watch for
// exfiltration (so a later appearance anywhere fires a canary event too).
func (g *Generator) AllValues(nKeys, nUsers int) []string {
	var vals []string
	for _, s := range g.APIKeys(nKeys) {
		vals = append(vals, s.Value)
	}
	for _, u := range g.Users(nUsers) {
		vals = append(vals, u.PasswordHash, u.APIKey)
	}
	sort.Strings(vals)
	return vals
}

func sqlEscape(s string) string {
	return strings.NewReplacer("'", "''", "\\", "\\\\").Replace(s)
}
