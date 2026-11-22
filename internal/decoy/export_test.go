package decoy

import (
	"encoding/json"
	"strings"
	"testing"
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
