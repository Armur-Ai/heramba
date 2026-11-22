package alert

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestAxiomSinkPostsIngest(t *testing.T) {
	var got []map[string]any
	var auth, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		path = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewAxiom(srv.URL, "heramba-detections", "ax-token", time.Second, event.SeverityLow)
	e := event.Deception(event.KindDecoyHit, "10.0.0.1", "/.env", "probe")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if auth != "Bearer ax-token" {
		t.Errorf("Authorization = %q, want Bearer ax-token", auth)
	}
	if path != "/v1/datasets/heramba-detections/ingest" {
		t.Errorf("ingest path = %q, want /v1/datasets/heramba-detections/ingest", path)
	}
	if len(got) != 1 {
		t.Fatalf("expected one record, got %v", got)
	}
	blob, _ := json.Marshal(got)
	for _, want := range []string{"decoy_hit", "10.0.0.1", "_time"} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("payload missing %q: %s", want, blob)
		}
	}
}

func TestAxiomDefaultBase(t *testing.T) {
	s := NewAxiom("", "ds", "t", time.Second, event.SeverityInfo)
	if !strings.HasPrefix(s.endpoint, defaultAxiomURL+"/v1/datasets/") {
		t.Errorf("empty base should default to Axiom Cloud, got %q", s.endpoint)
	}
}

func TestAxiomSinkFloorAndError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s := NewAxiom(srv.URL, "ds", "t", time.Second, event.SeverityHigh)
	if err := s.Emit(event.New(event.KindProbe, event.SeverityLow, "x", "/x", "d")); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("below-floor event should be dropped, got %d", calls)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer bad.Close()
	s2 := NewAxiom(bad.URL, "ds", "t", time.Second, event.SeverityInfo)
	if err := s2.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 403")
	}
}
