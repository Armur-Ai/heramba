package event

import (
	"encoding/json"
	"testing"
)

func TestSeverityString(t *testing.T) {
	cases := map[Severity]string{
		SeverityInfo: "info", SeverityLow: "low", SeverityMedium: "medium",
		SeverityHigh: "high", SeverityCritical: "critical", Severity(99): "unknown",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Fatalf("%d.String() = %q, want %q", s, got, want)
		}
	}
}

func TestDeceptionIsZeroFP(t *testing.T) {
	e := Deception(KindDecoyHit, "1.2.3.4", "/internal/admin-decoy", "GET on decoy")
	if e.Confidence != 1.0 {
		t.Fatalf("deception confidence = %v, want 1.0", e.Confidence)
	}
	if e.Severity != SeverityHigh || e.SeverityS != "high" {
		t.Fatalf("severity = %v/%q", e.Severity, e.SeverityS)
	}
	if e.Time.IsZero() {
		t.Fatal("time not stamped")
	}
}

func TestEventJSONRoundTrip(t *testing.T) {
	e := New(KindAgenticTraffic, SeverityMedium, "9.9.9.9", "/api/users", "cadence+fingerprint").
		WithLabel("classifier", "cadence")
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var got Event
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindAgenticTraffic || got.Labels["classifier"] != "cadence" || got.SeverityS != "medium" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}
