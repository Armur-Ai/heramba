package alert

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestPagerDutyTriggersIncident(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	s := NewPagerDuty("rk-123", time.Second, event.SeverityLow)
	s.url = srv.URL
	if err := s.Emit(event.New(event.KindEgressBlocked, event.SeverityCritical, "1.2.3.4", "evil.test", "exfil")); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if got["routing_key"] != "rk-123" || got["event_action"] != "trigger" {
		t.Errorf("bad PD payload: %v", got)
	}
	pl, _ := got["payload"].(map[string]any)
	if pl == nil || pl["severity"] != "critical" {
		t.Errorf("bad PD payload severity: %v", got["payload"])
	}
}

func TestOpsgenieRaisesAlert(t *testing.T) {
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	s := NewOpsgenie("genie-1", time.Second, event.SeverityLow)
	s.url = srv.URL
	if err := s.Emit(event.Deception(event.KindCanaryTriggered, "2.2.2.2", "tok", "d")); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if auth != "GenieKey genie-1" {
		t.Errorf("bad opsgenie auth: %q", auth)
	}
	if got["priority"] != "P2" { // deception => High => P2
		t.Errorf("bad opsgenie priority: %v", got["priority"])
	}
}

func TestPagingSeverityFloors(t *testing.T) {
	var pdCalls, ogCalls int
	pd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { pdCalls++; w.WriteHeader(202) }))
	og := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { ogCalls++; w.WriteHeader(202) }))
	defer pd.Close()
	defer og.Close()

	p := NewPagerDuty("k", time.Second, event.SeverityCritical)
	p.url = pd.URL
	o := NewOpsgenie("k", time.Second, event.SeverityCritical)
	o.url = og.URL
	low := event.New(event.KindProbe, event.SeverityHigh, "x", "y", "d") // below critical floor
	_ = p.Emit(low)
	_ = o.Emit(low)
	if pdCalls != 0 || ogCalls != 0 {
		t.Fatalf("below-floor events must not page (pd=%d og=%d)", pdCalls, ogCalls)
	}
}
