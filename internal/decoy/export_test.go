package decoy

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExportIncludesDefaultsAndPreset(t *testing.T) {
	r := NewRegistry()
	p, _ := GetPreset("laravel")
	r.ApplyPreset(p)
	s := r.Export()
	if len(s.Paths) == 0 || len(s.Prefixes) == 0 {
		t.Fatalf("surface should have paths and zones: %+v", s)
	}
	b, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var back Surface
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("export JSON must round-trip: %v", err)
	}
	found := false
	for _, e := range back.Paths {
		if e.Path == "/storage/logs/laravel.log" {
			found = true
		}
	}
	if !found {
		t.Error("laravel preset path missing from export")
	}
}

func TestNginxRender(t *testing.T) {
	r := NewRegistry()
	out := r.Export().Nginx()
	if !strings.Contains(out, "location = /.env { return 404; }") {
		t.Errorf("nginx export missing exact-path block:\n%s", out)
	}
	if !strings.Contains(out, "location /.git/ { return 404; }") {
		t.Errorf("nginx export missing zone block:\n%s", out)
	}
}

func TestCaddyRender(t *testing.T) {
	r := NewRegistry()
	out := r.Export().Caddy()
	if !strings.Contains(out, "respond /.env 404") {
		t.Errorf("caddy export missing exact-path directive:\n%s", out)
	}
	// Zones render as a wildcard prefix matcher.
	if !strings.Contains(out, "respond /.git/* 404") {
		t.Errorf("caddy export missing zone directive:\n%s", out)
	}
}

func TestTraefikRender(t *testing.T) {
	r := NewRegistry()
	p, _ := GetPreset("laravel")
	r.ApplyPreset(p)
	out := r.Export().Traefik()
	// Genuine Traefik rule syntax: Path(...) for exact lures, PathPrefix(...)
	// for zones, joined by ||, around backtick-quoted values.
	if !strings.Contains(out, "Path(`/.env`)") {
		t.Errorf("traefik export missing Path matcher:\n%s", out)
	}
	if !strings.Contains(out, "PathPrefix(`/.git/`)") {
		t.Errorf("traefik export missing PathPrefix matcher:\n%s", out)
	}
	if !strings.Contains(out, "||") {
		t.Errorf("traefik export should OR matchers together:\n%s", out)
	}
	if !strings.Contains(out, "service: heramba-decoy-404") {
		t.Errorf("traefik export missing service wiring:\n%s", out)
	}
	// A preset path must be present too.
	if !strings.Contains(out, "Path(`/storage/logs/laravel.log`)") {
		t.Errorf("traefik export missing preset lure:\n%s", out)
	}
}

func TestHAProxyRender(t *testing.T) {
	r := NewRegistry()
	out := r.Export().HAProxy()
	if !strings.Contains(out, "acl heramba_decoy path ") || !strings.Contains(out, "/.env") {
		t.Errorf("haproxy export missing exact-path acl:\n%s", out)
	}
	if !strings.Contains(out, "acl heramba_decoy path_beg ") || !strings.Contains(out, "/.git/") {
		t.Errorf("haproxy export missing zone acl:\n%s", out)
	}
	if !strings.Contains(out, "http-request deny deny_status 404 if heramba_decoy") {
		t.Errorf("haproxy export missing deny rule:\n%s", out)
	}
}

func TestEnvoyRenderIsValidYAML(t *testing.T) {
	r := NewRegistry()
	p, _ := GetPreset("django")
	r.ApplyPreset(p)
	out := r.Export().Envoy()

	// The routes block must parse as YAML and carry path + prefix matches.
	var parsed struct {
		Routes []struct {
			Match          map[string]string `yaml:"match"`
			DirectResponse struct {
				Status int `yaml:"status"`
			} `yaml:"direct_response"`
		} `yaml:"routes"`
	}
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("envoy export must be valid YAML: %v\n%s", err, out)
	}
	if len(parsed.Routes) == 0 {
		t.Fatal("envoy export has no routes")
	}
	var sawPath, sawPrefix bool
	for _, rt := range parsed.Routes {
		if rt.DirectResponse.Status != 404 {
			t.Errorf("route should direct_response 404, got %d", rt.DirectResponse.Status)
		}
		if _, ok := rt.Match["path"]; ok {
			sawPath = true
		}
		if _, ok := rt.Match["prefix"]; ok {
			sawPrefix = true
		}
	}
	if !sawPath || !sawPrefix {
		t.Errorf("envoy export should have both path and prefix matches (path=%v prefix=%v)", sawPath, sawPrefix)
	}
}
