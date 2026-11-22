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

func TestOpenObserveSinkPostsRecords(t *testing.T) {
	var got []map[string]any
	var user, pass, path string
	var hasBasic bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, hasBasic = r.BasicAuth()
		path = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewOpenObserve(srv.URL, "", "", "root@example.com", "pw", time.Second, event.SeverityLow)
	e := event.Deception(event.KindDecoyHit, "10.0.0.1", "/.env", "probe")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if !hasBasic || user != "root@example.com" || pass != "pw" {
		t.Errorf("basic auth = %q/%q (ok=%v), want root@example.com/pw", user, pass, hasBasic)
	}
	// Defaults: org=default, stream=heramba.
	if path != "/api/default/heramba/_json" {
		t.Errorf("ingest path = %q, want /api/default/heramba/_json", path)
	}
	if len(got) != 1 {
		t.Fatalf("expected one record, got %v", got)
	}
	blob, _ := json.Marshal(got)
	for _, want := range []string{"decoy_hit", "10.0.0.1", "_timestamp"} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("payload missing %q: %s", want, blob)
		}
	}
}

func TestOpenObserveCustomOrgStream(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s := NewOpenObserve(srv.URL, "secops", "detections", "u", "p", time.Second, event.SeverityInfo)
	_ = s.Emit(event.New(event.KindProbe, event.SeverityHigh, "x", "y", "d"))
	if path != "/api/secops/detections/_json" {
		t.Errorf("path = %q, want /api/secops/detections/_json", path)
	}
}

func TestOpenObserveSinkFloorAndError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s := NewOpenObserve(srv.URL, "", "", "u", "p", time.Second, event.SeverityHigh)
	if err := s.Emit(event.New(event.KindProbe, event.SeverityLow, "x", "/x", "d")); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("below-floor event should be dropped, got %d", calls)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	s2 := NewOpenObserve(bad.URL, "", "", "u", "p", time.Second, event.SeverityInfo)
	if err := s2.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 500")
	}
}
