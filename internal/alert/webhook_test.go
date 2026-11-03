package alert

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestWebhookSinkPostsJSON(t *testing.T) {
	var mu sync.Mutex
	var got []event.Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected JSON content-type, got %q", r.Header.Get("Content-Type"))
		}
		b, _ := io.ReadAll(r.Body)
		var e event.Event
		if err := json.Unmarshal(b, &e); err != nil {
			t.Errorf("bad JSON body: %v", err)
		}
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := NewWebhook(srv.URL, time.Second, event.SeverityMedium)
	// Below the floor: not sent.
	if err := sink.Emit(event.New(event.KindAgenticTraffic, event.SeverityInfo, "x", "/", "info")); err != nil {
		t.Fatal(err)
	}
	// At/above the floor: sent.
	if err := sink.Emit(event.Deception(event.KindDecoyHit, "1.2.3.4", "/.env", "decoy")); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 delivered event (severity floor), got %d", len(got))
	}
	if got[0].Kind != event.KindDecoyHit {
		t.Fatalf("delivered wrong event: %s", got[0].Kind)
	}
}

func TestWebhookSinkReturnsErrorOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	sink := NewWebhook(srv.URL, time.Second, event.SeverityInfo)
	if err := sink.Emit(event.Deception(event.KindDecoyHit, "x", "/.env", "d")); err == nil {
		t.Fatal("expected an error on a 500 response")
	}
}
