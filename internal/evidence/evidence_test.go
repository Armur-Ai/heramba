package evidence

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func sampleEvent(src string) SecurityEvent {
	return NewSecurityEvent(event.KindDecoyHit, event.SeverityHigh, src, Provenance{
		Sensor:     "edge",
		RawRef:     "req-123",
		ObservedAt: time.Now().UTC(),
	}).WithAttr("path", "/.env").WithStage(StageDeceptionTouch)
}

func TestSecurityEventValidate(t *testing.T) {
	e := sampleEvent("10.0.0.4")
	e.Confidence = 1.0
	if err := e.Validate(); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}

	bad := e
	bad.Confidence = 2.0
	if err := bad.Validate(); err == nil {
		t.Fatal("expected out-of-range confidence to be rejected")
	}

	noProv := sampleEvent("10.0.0.4")
	noProv.Provenance = Provenance{}
	if err := noProv.Validate(); err == nil {
		t.Fatal("expected missing provenance to be rejected")
	}
}

func TestFromDetectionCarriesConfidenceAndLabels(t *testing.T) {
	d := event.Deception(event.KindCanaryTriggered, "1.2.3.4", "db-password", "canary used")
	d = d.WithLabel("where", "header")
	se := FromDetection(d, "edge", "req-9")
	if se.Confidence != 1.0 {
		t.Fatalf("deception confidence not carried: got %v", se.Confidence)
	}
	if se.Attributes["where"] != "header" {
		t.Fatalf("label not carried: %v", se.Attributes)
	}
	if se.Stage != StageDeceptionTouch {
		t.Fatalf("stage mapping wrong: %v", se.Stage)
	}
	if err := se.Validate(); err != nil {
		t.Fatalf("lifted event invalid: %v", err)
	}
}

func TestStoreRejectsInvalid(t *testing.T) {
	s := NewStore()
	bad := sampleEvent("x")
	bad.ID = "" // invalid
	if err := s.PutEvent(bad); err == nil {
		t.Fatal("store accepted an invalid event")
	}
	if ev, _, _, _ := s.Counts(); ev != 0 {
		t.Fatalf("invalid event leaked into store: %d", ev)
	}
}

func TestFactRequiresDerivation(t *testing.T) {
	f := NewFact("no sources")
	if err := f.Validate(); err == nil {
		t.Fatal("fact with no derivation should be invalid")
	}
	f2 := NewFact("real", NewID("se"))
	if err := f2.Validate(); err != nil {
		t.Fatalf("fact with derivation rejected: %v", err)
	}
}

func TestHypothesisLifecycle(t *testing.T) {
	h := NewHypothesis("sessions A and B are the same actor", 0.3, time.Hour)
	if h.Status != StatusOpen {
		t.Fatalf("new hypothesis not open: %v", h.Status)
	}
	start := h.Confidence
	h.Support(EvidenceRef{ID: NewID("se")}, 0.5)
	if h.Confidence <= start {
		t.Fatalf("support did not raise confidence: %v -> %v", start, h.Confidence)
	}
	mid := h.Confidence
	h.Contradict(EvidenceRef{ID: NewID("se")}, 0.9)
	if h.Confidence >= mid {
		t.Fatalf("contradict did not lower confidence: %v -> %v", mid, h.Confidence)
	}
	if h.Confidence < 0 || h.Confidence > 1 {
		t.Fatalf("confidence left [0,1]: %v", h.Confidence)
	}

	// Expiry: a stale open hypothesis past review-by expires.
	h.ReviewBy = time.Now().Add(-time.Minute)
	if !h.Expire(time.Now()) || h.Status != StatusExpired {
		t.Fatalf("expected expiry, got status %v", h.Status)
	}
}

func TestHypothesisDecay(t *testing.T) {
	h := NewHypothesis("stale link", 0.8, time.Hour)
	// One half-life elapsed should roughly halve confidence.
	h.Decay(2*time.Hour, 2*time.Hour, 0.05)
	if h.Confidence > 0.45 || h.Confidence < 0.35 {
		t.Fatalf("decay to ~0.4 expected, got %v", h.Confidence)
	}
	// Decaying far past the floor rejects it.
	h.Decay(100*time.Hour, 2*time.Hour, 0.1)
	if h.Status != StatusRejected {
		t.Fatalf("expected rejection below floor, got %v (conf %v)", h.Status, h.Confidence)
	}
}

func TestRelationshipValidation(t *testing.T) {
	a, b := NewID("se"), NewID("se")
	r := NewRelationship(RelSameSource, a, b, 1.0)
	if err := r.Validate(); err != nil {
		t.Fatalf("valid rel rejected: %v", err)
	}
	if r.IsInference() {
		t.Fatal("same_source should be observational, not inference")
	}
	self := NewRelationship(RelPrecedes, a, a, 1.0)
	if err := self.Validate(); err == nil {
		t.Fatal("self-loop should be invalid")
	}
	actor := NewRelationship(RelSameActor, a, b, 0.6)
	if !actor.IsInference() {
		t.Fatal("same_actor must be flagged as inference")
	}
}

func TestQueriesBySourceAndWindow(t *testing.T) {
	s := NewStore()
	now := time.Now().UTC()
	mk := func(src string, off time.Duration) SecurityEvent {
		e := NewSecurityEvent(event.KindProbe, event.SeverityLow, src, Provenance{
			Sensor:     "edge",
			ObservedAt: now.Add(off),
		})
		e.Confidence = 0.5
		return e
	}
	for _, e := range []SecurityEvent{
		mk("A", -3*time.Minute), mk("A", -1*time.Minute),
		mk("B", -2*time.Minute), mk("A", 5*time.Minute),
	} {
		if err := s.PutEvent(e); err != nil {
			t.Fatalf("put: %v", err)
		}
	}
	a := s.EventsBySource("A")
	if len(a) != 3 {
		t.Fatalf("expected 3 A-events, got %d", len(a))
	}
	// sorted chronologically
	for i := 1; i < len(a); i++ {
		if a[i].Provenance.ObservedAt.Before(a[i-1].Provenance.ObservedAt) {
			t.Fatal("EventsBySource not chronologically sorted")
		}
	}
	win := s.EventsInWindow(now.Add(-4*time.Minute), now)
	if len(win) != 3 { // excludes the +5m future event
		t.Fatalf("expected 3 in-window, got %d", len(win))
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "evidence.jsonl")

	// Write some records.
	s1, skipped, err := OpenStore(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if skipped != 0 {
		t.Fatalf("unexpected skip on empty log: %d", skipped)
	}
	e := sampleEvent("10.0.0.9")
	e.Confidence = 1.0
	if err := s1.PutEvent(e); err != nil {
		t.Fatalf("put event: %v", err)
	}
	f := NewFact("10.0.0.9 touched /.env", e.ID)
	if err := s1.PutFact(f); err != nil {
		t.Fatalf("put fact: %v", err)
	}
	h := NewHypothesis("10.0.0.9 is an automated scanner", 0.7, time.Hour)
	h.Support(EvidenceRef{ID: e.ID}, 0.4)
	if err := s1.PutHypothesis(h); err != nil {
		t.Fatalf("put hyp: %v", err)
	}
	r := NewRelationship(RelDerivedFrom, f.ID, e.ID, 1.0)
	if err := s1.PutRelationship(r); err != nil {
		t.Fatalf("put rel: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Reopen and confirm everything round-tripped.
	s2, skipped, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	if skipped != 0 {
		t.Fatalf("records skipped on reload: %d", skipped)
	}
	ev, fa, hy, re := s2.Counts()
	if ev != 1 || fa != 1 || hy != 1 || re != 1 {
		t.Fatalf("roundtrip counts wrong: ev=%d fa=%d hy=%d re=%d", ev, fa, hy, re)
	}
	got, ok := s2.Event(e.ID)
	if !ok || got.Confidence != 1.0 || got.Attributes["path"] != "/.env" {
		t.Fatalf("event did not round-trip faithfully: %+v ok=%v", got, ok)
	}
	gh, ok := s2.Hypothesis(h.ID)
	if !ok || len(gh.Supporting) != 1 {
		t.Fatalf("hypothesis support did not round-trip: %+v ok=%v", gh, ok)
	}
}

func TestUnknownSchemaIsSkippedNotFatal(t *testing.T) {
	s := NewStore()
	// A line with a future schema should be skipped, not crash.
	line := []byte(`{"schema":9999,"type":"security_event","record":{}}` + "\n")
	skipped, err := s.loadBytes(line)
	if err != nil {
		t.Fatalf("unknown schema should skip, not error: %v", err)
	}
	if skipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", skipped)
	}
}
