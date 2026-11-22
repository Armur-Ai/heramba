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

func TestDatadogSinkPostsLogArray(t *testing.T) {
	var got []map[string]any
	var key string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("DD-API-KEY")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	s := NewDatadog(srv.URL, "ddkey", time.Second, event.SeverityLow)
	if err := s.Emit(event.New(event.KindEgressBlocked, event.SeverityCritical, "1.2.3.4", "evil.test", "d")); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if key != "ddkey" {
		t.Errorf("missing DD-API-KEY, got %q", key)
	}
	if len(got) != 1 || got[0]["ddsource"] != "heramba" || got[0]["status"] != "critical" {
		t.Errorf("unexpected datadog payload: %v", got)
	}
}

func TestDatadogDefaultURL(t *testing.T) {
	if NewDatadog("", "k", 0, event.SeverityInfo).url != defaultDatadogURL {
		t.Fatal("empty url should default to the US1 intake")
	}
}

func TestDatadogStatusMapping(t *testing.T) {
	cases := map[event.Severity]string{
		event.SeverityCritical: "critical",
		event.SeverityHigh:     "error",
		event.SeverityMedium:   "warning",
		event.SeverityLow:      "info",
		event.SeverityInfo:     "notice",
	}
	for sev, want := range cases {
		if got := ddStatus(sev); got != want {
			t.Errorf("ddStatus(%v)=%s want %s", sev, got, want)
		}
	}
}
