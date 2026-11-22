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

func TestElasticSinkIndexesDoc(t *testing.T) {
	var path, auth string
	var doc map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		auth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &doc)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	s := NewElastic(srv.URL, "heramba-test", "abc", time.Second, event.SeverityLow)
	if err := s.Emit(event.Deception(event.KindCanaryTriggered, "1.2.3.4", "tok", "d")); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if !strings.HasSuffix(path, "/heramba-test/_doc") {
		t.Errorf("unexpected index path: %s", path)
	}
	if auth != "ApiKey abc" {
		t.Errorf("missing ApiKey auth: %q", auth)
	}
	if doc["@timestamp"] == nil || doc["kind"] != "canary_triggered" {
		t.Errorf("unexpected doc: %v", doc)
	}
}

func TestElasticDefaultIndexAndNoAuth(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s := NewElastic(srv.URL, "", "", time.Second, event.SeverityInfo)
	if s.index != "heramba-detections" {
		t.Errorf("default index wrong: %s", s.index)
	}
	_ = s.Emit(event.New(event.KindProbe, event.SeverityHigh, "x", "y", "d"))
	if auth != "" {
		t.Error("no api key => no Authorization header")
	}
}

func TestElasticErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	s := NewElastic(srv.URL, "i", "", time.Second, event.SeverityInfo)
	if err := s.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 400")
	}
}
