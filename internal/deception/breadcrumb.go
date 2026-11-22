package deception

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/Armur-Ai/heramba/internal/event"
)

// Breadcrumb is one step on a coherent fake trail: a decoy path, a hint that
// points at the Next step, and — only on the final step — a synthetic "jackpot"
// secret. Each step's Next equals the following step's Path, so an attacker who
// follows the breadcrumbs is led from one decoy to the next, every hop a
// zero-false-positive signal, until a dead-end payoff of fabricated data.
type Breadcrumb struct {
	Step   int    `json:"step"`
	Path   string `json:"path"`
	Hint   string `json:"hint"`
	Next   string `json:"next,omitempty"`
	Secret string `json:"secret,omitempty"` // synthetic; only on the last step
}

// Trail is a deterministic, seeded sequence of breadcrumbs. The same seed and
// length always produce the same trail, so deception labs and replay are
// reproducible — mirroring how World is generated from a seed.
type Trail struct {
	seed   int64
	steps  []Breadcrumb
	byPath map[string]Breadcrumb
}

// trailDirs are plausible "internal" parents that read like something worth
// chasing but that no legitimate client ever walks.
var trailDirs = []string{"/internal", "/legacy", "/api/internal", "/.backup", "/ops", "/private"}

// trailLeafs are file-ish leaves to make a step look like a concrete artifact.
var trailLeafs = []string{"readme.txt", "notes.md", "config.yaml", "dump.sql", "keys.json", "handoff.txt"}

// BuildTrail constructs a coherent breadcrumb trail of the given length (min 2)
// from seed. Every step but the last points to the next; the last carries a
// synthetic secret as the payoff.
func BuildTrail(seed int64, length int) *Trail {
	if length < 2 {
		length = 2
	}
	rng := rand.New(rand.NewSource(seed))

	// Pre-compute all step paths so each step can reference the next.
	paths := make([]string, length)
	leaves := make([]string, length)
	for i := 0; i < length; i++ {
		dir := trailDirs[rng.Intn(len(trailDirs))]
		noun := nameNouns[rng.Intn(len(nameNouns))]
		leaf := trailLeafs[rng.Intn(len(trailLeafs))]
		leaves[i] = leaf
		paths[i] = fmt.Sprintf("%s/%s-%02d/%s", dir, noun, i, leaf)
	}

	t := &Trail{seed: seed, byPath: map[string]Breadcrumb{}}
	for i := 0; i < length; i++ {
		b := Breadcrumb{Step: i, Path: paths[i]}
		if i < length-1 {
			b.Next = paths[i+1]
			b.Hint = fmt.Sprintf("ops-note: superseded — the current %s lives at %s",
				strings.TrimSuffix(leaves[i], ".txt"), b.Next)
		} else {
			b.Secret = synthToken(rng) // looks real, is synthetic + watchable
			b.Hint = "ops-note: FINAL — production credentials below, rotate quarterly"
		}
		t.steps = append(t.steps, b)
		t.byPath[b.Path] = b
	}
	return t
}

// Steps returns the ordered breadcrumbs.
func (t *Trail) Steps() []Breadcrumb { return append([]Breadcrumb(nil), t.steps...) }

// Entry returns the first breadcrumb — where a trail is seeded (e.g. referenced
// from a tripwire or a counter lure).
func (t *Trail) Entry() Breadcrumb { return t.steps[0] }

// Lookup returns the breadcrumb for a path, if the path is on this trail.
func (t *Trail) Lookup(path string) (Breadcrumb, bool) {
	b, ok := t.byPath[path]
	return b, ok
}

// Paths returns every decoy path on the trail, in order — register these as
// decoys so each hop is detected.
func (t *Trail) Paths() []string {
	out := make([]string, len(t.steps))
	for i, b := range t.steps {
		out[i] = b.Path
	}
	return out
}

// Secrets returns the synthetic secret(s) on the trail — register these as
// canaries so exfiltration of the payoff is caught too.
func (t *Trail) Secrets() []string {
	var out []string
	for _, b := range t.steps {
		if b.Secret != "" {
			out = append(out, b.Secret)
		}
	}
	return out
}

// RenderHint renders a breadcrumb as a small response body an attacker reads to
// find the next hop: an intermediate step is an HTML note linking onward; the
// final step is a JSON blob holding the synthetic secret.
func (t *Trail) RenderHint(b Breadcrumb) string {
	if b.Secret != "" {
		return fmt.Sprintf(`{"note":%q,"api_token":%q,"synthetic":true}`+"\n", b.Hint, b.Secret)
	}
	return fmt.Sprintf("<!-- %s -->\n<p>See <a href=%q>%s</a>.</p>\n", b.Hint, b.Next, b.Next)
}

// Event builds a zero-false-positive decoy-hit event for a step on the trail,
// labeled with the trail position so triage can see how deep the attacker
// followed the breadcrumbs.
func (t *Trail) Event(b Breadcrumb, source string) event.Event {
	detail := "breadcrumb trail step " + strconv.Itoa(b.Step) + " — " + b.Path
	ev := event.Deception(event.KindDecoyHit, source, b.Path, detail).
		WithLabel("breadcrumb", "true").
		WithLabel("trail_seed", strconv.FormatInt(t.seed, 10)).
		WithLabel("trail_step", strconv.Itoa(b.Step)).
		WithLabel("trail_len", strconv.Itoa(len(t.steps)))
	if b.Next == "" {
		ev = ev.WithLabel("trail_final", "true")
	}
	return ev
}
