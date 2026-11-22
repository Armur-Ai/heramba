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

func TestLokiSinkPostsStream(t *testing.T) {
	var got map[string]any
	var tenant string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant = r.Header.Get("X-Scope-OrgID")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	s := NewLoki(srv.URL, "tenant-7", time.Second, event.SeverityLow)
	e := event.Deception(event.KindDecoyHit, "10.0.0.1", "/.env", "probe")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if tenant != "tenant-7" {
		t.Errorf("tenant header = %q, want tenant-7", tenant)
	}
	streams, ok := got["streams"].([]any)
	if !ok || len(streams) != 1 {
		t.Fatalf("expected one stream, got %v", got["streams"])
	}
	blob, _ := json.Marshal(got)
	for _, want := range []string{"decoy_hit", "high", `"app":"heramba"`, "10.0.0.1"} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("payload missing %q: %s", want, blob)
		}
	}
}

func TestLokiSinkRespectsFloorAndErrors(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s := NewLoki(srv.URL, "", time.Second, event.SeverityHigh)
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
	s2 := NewLoki(bad.URL, "", time.Second, event.SeverityInfo)
	if err := s2.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 500")
	}
}
