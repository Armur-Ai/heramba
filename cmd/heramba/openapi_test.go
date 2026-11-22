package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Armur-Ai/heramba/internal/config"
	"github.com/Armur-Ai/heramba/internal/decoy"
	"github.com/Armur-Ai/heramba/internal/edge"
	"github.com/Armur-Ai/heramba/internal/event"
)

const testSpec = `{"openapi":"3.0.0","paths":{"/login":{"post":{"requestBody":{"content":{"application/json":{"schema":{"type":"object","properties":{"username":{"type":"string"},"password":{"type":"string"}}}}}}}}}}`

func writeSpec(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(p, []byte(testSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestApplyOpenAPICanariesAppendsTokens(t *testing.T) {
	cfg := config.Default()
	cfg.Decoys.OpenAPISpec = writeSpec(t)
	cfg.Decoys.CanarySalt = "s"

	aug, n, err := applyOpenAPICanaries(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 { // only "password" is sensitive
		t.Fatalf("derived %d canaries, want 1", n)
	}
	want := decoy.DeriveBodyCanary("password", "s")
	var found bool
	for _, c := range aug.Decoys.Canaries {
		if c == want {
			found = true
		}
	}
	if !found {
		t.Errorf("augmented canaries %v missing derived token %q", aug.Decoys.Canaries, want)
	}
}

func TestApplyOpenAPICanariesNoSpecIsNoop(t *testing.T) {
	cfg := config.Default()
	aug, n, err := applyOpenAPICanaries(cfg)
	if err != nil || n != 0 {
		t.Fatalf("no spec should be a no-op: n=%d err=%v", n, err)
	}
	if len(aug.Decoys.Canaries) != 0 {
		t.Errorf("no spec should add no canaries, got %v", aug.Decoys.Canaries)
	}
}

func TestApplyOpenAPICanariesMissingFileErrors(t *testing.T) {
	cfg := config.Default()
	cfg.Decoys.OpenAPISpec = filepath.Join(t.TempDir(), "nope.json")
	if _, _, err := applyOpenAPICanaries(cfg); err == nil {
		t.Fatal("missing spec file should error")
	}
}

func TestOpenAPIBodyCanaryDetectedEndToEnd(t *testing.T) {
	cfg := config.Default()
	cfg.Decoys.OpenAPISpec = writeSpec(t)
	cfg.Decoys.CanarySalt = "secret-salt"
	aug, _, err := applyOpenAPICanaries(cfg)
	if err != nil {
		t.Fatal(err)
	}

	sink := &capSinkMain{}
	engine, err := edge.New(aug, sink)
	if err != nil {
		t.Fatal(err)
	}

	// An attacker who scraped the spec replays the planted canary in a body.
	tok := decoy.DeriveBodyCanary("password", "secret-salt")
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"password":"`+tok+`"}`))
	req.RemoteAddr = "203.0.113.7:4444"
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(httptest.NewRecorder(), req)

	var canary int
	for _, e := range sink.events {
		if e.Kind == event.KindCanaryTriggered {
			canary++
		}
	}
	if canary != 1 {
		t.Errorf("expected 1 canary_triggered from the body canary, got %d (events=%d)", canary, len(sink.events))
	}
}

type capSinkMain struct{ events []event.Event }

func (c *capSinkMain) Emit(e event.Event) error { c.events = append(c.events, e); return nil }
