package campaign

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
	"github.com/Armur-Ai/heramba/internal/evidence"
)

// mkEvent builds a normalized SecurityEvent at a given offset from a base time.
func mkEvent(base time.Time, off time.Duration, src, method, path, ua string, kind event.Kind) evidence.SecurityEvent {
	e := evidence.NewSecurityEvent(kind, event.SeverityMedium, src, evidence.Provenance{
		Sensor:     "edge",
		ObservedAt: base.Add(off),
	})
	e.Confidence = 0.5
	e = e.WithAttr("method", method).WithAttr("path", path).WithAttr("ua", ua)
	e.Target = path
	return e
}

// buildStore loads events into an evidence store.
func buildStore(t *testing.T, events []evidence.SecurityEvent) *evidence.Store {
	t.Helper()
	s := evidence.NewStore()
	for _, e := range events {
		if err := s.PutEvent(e); err != nil {
			t.Fatalf("put: %v", err)
		}
	}
	return s
}

func TestSessionizationSplitsByIdleGap(t *testing.T) {
	base := time.Now().UTC().Add(-2 * time.Hour)
	events := []evidence.SecurityEvent{
		mkEvent(base, 0, "A", "GET", "/login", "curl/8", event.KindProbe),
		mkEvent(base, 10*time.Second, "A", "GET", "/users/1", "curl/8", event.KindProbe),
		// idle gap > 30m → new session
		mkEvent(base, 50*time.Minute, "A", "GET", "/admin", "curl/8", event.KindProbe),
	}
	sz := DefaultSessionizer()
	sessions := sz.Sessionize(events)
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions from idle gap, got %d", len(sessions))
	}
}

// TestMultiSessionReconstruction is the CP2 acceptance test: a synthetic
// multi-session campaign is reconstructed, and the explanation distinguishes
// observed facts from inferred links.
func TestMultiSessionReconstruction(t *testing.T) {
	base := time.Now().UTC().Add(-3 * time.Hour)
	// Two coordinated scanners (different IPs, identical tool/behavior) hitting
	// the same surface in the same window — a coordinated campaign.
	var events []evidence.SecurityEvent
	for i, src := range []string{"10.0.0.1", "10.0.0.2"} {
		shift := time.Duration(i) * time.Second
		for j, p := range []string{"/login", "/users/1", "/users/2", "/admin", "/.env"} {
			kind := event.KindProbe
			if p == "/.env" {
				kind = event.KindDecoyHit
			}
			e := mkEvent(base, shift+time.Duration(j)*2*time.Second, src, "GET", p, "nuclei/3.1", kind)
			if kind == event.KindDecoyHit {
				e.Confidence = 1.0
				e.Stage = evidence.StageDeceptionTouch
			}
			events = append(events, e)
		}
	}
	store := buildStore(t, events)

	c := DefaultCorrelator()
	res := c.Reconstruct(store)

	if len(res.Sessions) != 2 {
		t.Fatalf("expected 2 sessions (one per source), got %d", len(res.Sessions))
	}
	if len(res.Actors) != 2 {
		t.Fatalf("expected 2 actors (distinct source keys), got %d", len(res.Actors))
	}
	// The two actors share targets in an overlapping window → one campaign.
	if len(res.Campaigns) != 1 {
		t.Fatalf("expected 1 coordinated campaign, got %d", len(res.Campaigns))
	}
	for _, camp := range res.Campaigns {
		if len(camp.ActorIDs) != 2 {
			t.Fatalf("campaign should link 2 actors, got %d", len(camp.ActorIDs))
		}
		if camp.Hypothesis == nil || camp.Hypothesis.Confidence >= 1.0 {
			t.Fatal("campaign must be a hypothesis with confidence < 1.0 (inference, not fact)")
		}
	}

	// Explanation separates observed facts from inferred links.
	var anyActor evidence.ID
	for id := range res.Actors {
		anyActor = id
		break
	}
	exp := res.Explain(anyActor)
	if !strings.Contains(exp, "observed (facts)") || !strings.Contains(exp, "inferred (hypothesis)") {
		t.Fatalf("explanation must separate facts from inference:\n%s", exp)
	}
	if !strings.Contains(exp, "decoy/canary") {
		t.Fatalf("explanation should surface the zero-FP deception hit:\n%s", exp)
	}

	// Timeline retrievable and chronological.
	tl := res.ActorTimeline(store, anyActor)
	if len(tl) != 5 {
		t.Fatalf("expected 5 timeline entries, got %d", len(tl))
	}
	for i := 1; i < len(tl); i++ {
		if tl[i].Time.Before(tl[i-1].Time) {
			t.Fatal("timeline not chronological")
		}
	}
}

// TestFalseMergeCorrectable verifies entity-resolution merge/split correction.
func TestFalseMergeCorrectable(t *testing.T) {
	base := time.Now().UTC().Add(-time.Hour)
	r := DefaultResolver()
	// Two sessions from the SAME source key → merged into one actor.
	s1 := Session{ID: evidence.NewID("sess"), SourceKey: "9.9.9.9",
		Start: base, End: base.Add(time.Minute), Targets: map[string]int{}, Fingerprint: newFingerprint()}
	s2 := Session{ID: evidence.NewID("sess"), SourceKey: "9.9.9.9",
		Start: base.Add(2 * time.Minute), End: base.Add(3 * time.Minute), Targets: map[string]int{}, Fingerprint: newFingerprint()}
	a1, _ := r.Assign(s1)
	a2, _ := r.Assign(s2)
	if a1 != a2 {
		t.Fatal("same-source sessions should resolve to the same actor")
	}
	if len(a1.SessionIDs) != 2 {
		t.Fatalf("actor should hold 2 sessions, got %d", len(a1.SessionIDs))
	}

	// Correct a false merge: split s2 back out.
	before := len(r.Actors())
	split, err := r.Split(a1, s2.ID, "shared NAT: later evidence shows distinct operators")
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if len(r.Actors()) != before+1 {
		t.Fatalf("split should add an actor: %d → %d", before, len(r.Actors()))
	}
	if len(a1.SessionIDs) != 1 {
		t.Fatalf("original actor should have 1 session after split, got %d", len(a1.SessionIDs))
	}
	if len(a1.Link.Contradicting) == 0 {
		t.Fatal("split should record contradicting evidence on the original link")
	}
	if len(split.Link.Supporting) == 0 {
		t.Fatal("split actor should carry seed evidence")
	}
}

// TestCrossSourceMergeOnlyWithIdentityToken verifies the identity-resolution
// rule: a generic shared tool does NOT merge two IPs, but a distinctive shared
// identity token (here a canary) DOES — across source keys.
func TestCrossSourceMergeOnlyWithIdentityToken(t *testing.T) {
	base := time.Now().UTC().Add(-time.Hour)
	r := DefaultResolver()

	// Two IPs, same generic tool, NO identity token → stay distinct.
	fp := func(tokens ...string) Fingerprint {
		f := newFingerprint()
		f.ToolSignals["nuclei"] = struct{}{}
		for _, tk := range tokens {
			f.IdentityTokens[tk] = struct{}{}
		}
		return f
	}
	sGeneric1 := Session{ID: evidence.NewID("sess"), SourceKey: "1.1.1.1", Start: base, End: base.Add(time.Minute), Targets: map[string]int{}, Fingerprint: fp()}
	sGeneric2 := Session{ID: evidence.NewID("sess"), SourceKey: "2.2.2.2", Start: base, End: base.Add(time.Minute), Targets: map[string]int{}, Fingerprint: fp()}
	a1, _ := r.Assign(sGeneric1)
	a2, _ := r.Assign(sGeneric2)
	if a1 == a2 {
		t.Fatal("generic tool overlap must NOT merge distinct source keys into one actor")
	}

	// Two IPs sharing a distinctive canary token → merge into one actor.
	r2 := DefaultResolver()
	sTok1 := Session{ID: evidence.NewID("sess"), SourceKey: "3.3.3.3", Start: base, End: base.Add(time.Minute), Targets: map[string]int{}, Fingerprint: fp("canary-abc123")}
	sTok2 := Session{ID: evidence.NewID("sess"), SourceKey: "4.4.4.4", Start: base, End: base.Add(time.Minute), Targets: map[string]int{}, Fingerprint: fp("canary-abc123")}
	b1, _ := r2.Assign(sTok1)
	b2, _ := r2.Assign(sTok2)
	if b1 != b2 {
		t.Fatal("shared distinctive identity token should merge across source keys")
	}
	if !b1.SpansMultipleSources() {
		t.Fatal("merged actor should span multiple source keys")
	}
}

// TestBenignControlNoFalseCampaign is the negative control: unrelated, benign,
// low-rate traffic to different endpoints must NOT be fused into a campaign.
func TestBenignControlNoFalseCampaign(t *testing.T) {
	base := time.Now().UTC().Add(-5 * time.Hour)
	var events []evidence.SecurityEvent
	// Two ordinary browsers, different IPs, different pages, spread out, no
	// shared targets, no decoy hits.
	events = append(events,
		mkEvent(base, 0, "203.0.113.5", "GET", "/home", "Mozilla/5.0", event.KindProbe),
		mkEvent(base, time.Hour, "198.51.100.7", "GET", "/pricing", "Mozilla/5.0", event.KindProbe),
	)
	// mark them low confidence / not deception
	store := buildStore(t, events)
	res := DefaultCorrelator().Reconstruct(store)
	if len(res.Campaigns) != 0 {
		t.Fatalf("benign unrelated traffic must not form a campaign, got %d", len(res.Campaigns))
	}
}

// TestGraphPerformance records a rough reconstruction throughput so regressions
// are visible. It is a measurement, not a hard SLA; it only fails if grossly
// slow (safety valve against accidental O(n^3)).
func TestGraphPerformance(t *testing.T) {
	base := time.Now().UTC().Add(-24 * time.Hour)
	const sources = 50
	const perSource = 40
	var events []evidence.SecurityEvent
	for s := 0; s < sources; s++ {
		src := fmt.Sprintf("10.1.%d.%d", s/256, s%256)
		for i := 0; i < perSource; i++ {
			events = append(events, mkEvent(base, time.Duration(s)*time.Second+time.Duration(i)*time.Second,
				src, "GET", fmt.Sprintf("/api/items/%d", i), "scanner/1", event.KindProbe))
		}
	}
	store := buildStore(t, events)
	start := time.Now()
	res := DefaultCorrelator().Reconstruct(store)
	elapsed := time.Since(start)
	n := sources * perSource
	t.Logf("reconstructed %d events → %d sessions, %d actors in %s (%.0f events/ms)",
		n, len(res.Sessions), len(res.Actors), elapsed, float64(n)/float64(elapsed.Milliseconds()+1))
	if elapsed > 5*time.Second {
		t.Fatalf("reconstruction too slow for %d events: %s", n, elapsed)
	}
}
