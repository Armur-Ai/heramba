package alert

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestTemplateWebhookRendersAndPosts(t *testing.T) {
	var body, ctype string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctype = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpl := `{"kind":"{{.Kind}}","src":"{{.Source}}","sev":"{{.SeverityS}}","conf":{{.Confidence}}}`
	s, err := NewTemplateWebhook(srv.URL, tmpl, "application/json", time.Second, event.SeverityLow)
	if err != nil {
		t.Fatal(err)
	}
	e := event.Deception(event.KindDecoyHit, "10.0.0.5", "/.env", "probe")
	if err := s.Emit(e); err != nil {
		t.Fatalf("emit: %v", err)
	}
	want := `{"kind":"decoy_hit","src":"10.0.0.5","sev":"high","conf":1}`
	if body != want {
		t.Errorf("rendered body = %q, want %q", body, want)
	}
	if !strings.Contains(ctype, "application/json") {
		t.Errorf("content-type = %q", ctype)
	}
}

func TestTemplateWebhookCustomContentType(t *testing.T) {
	s, err := NewTemplateWebhook("http://x", "kind={{.Kind}}", "text/plain", time.Second, event.SeverityInfo)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.Render(event.New(event.KindProbe, event.SeverityLow, "a", "/b", "d"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "kind=probe" {
		t.Errorf("render = %q, want kind=probe", out)
	}
}

func TestTemplateWebhookBadTemplateRejected(t *testing.T) {
	if _, err := NewTemplateWebhook("http://x", "{{.Kind", "", time.Second, event.SeverityInfo); err == nil {
		t.Fatal("expected error parsing malformed template")
	}
	if _, err := NewTemplateWebhook("http://x", "", "", time.Second, event.SeverityInfo); err == nil {
		t.Fatal("expected error on empty template")
	}
}

func TestTemplateWebhookFloorAndError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s, _ := NewTemplateWebhook(srv.URL, "{{.Kind}}", "", time.Second, event.SeverityHigh)
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
	s2, _ := NewTemplateWebhook(bad.URL, "{{.Kind}}", "", time.Second, event.SeverityInfo)
	if err := s2.Emit(event.New(event.KindDecoyHit, event.SeverityHigh, "x", "y", "d")); err == nil {
		t.Fatal("expected error on 500")
	}
}
