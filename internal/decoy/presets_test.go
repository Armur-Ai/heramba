package decoy

import (
	"strings"
	"testing"
)

func TestPresetNamesSorted(t *testing.T) {
	names := PresetNames()
	if len(names) == 0 {
		t.Fatal("expected presets")
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("PresetNames not sorted: %v", names)
		}
	}
	// Spot-check the stacks the plan calls out.
	for _, want := range []string{"node", "rails", "django", "laravel", "wordpress", "spring"} {
		if _, ok := GetPreset(want); !ok {
			t.Errorf("missing preset %q", want)
		}
	}
}

func TestGetPresetUnknown(t *testing.T) {
	if _, ok := GetPreset("cobol"); ok {
		t.Fatal("unknown preset should return ok=false")
	}
}

func TestApplyPresetRegistersPathsAndZones(t *testing.T) {
	r := NewRegistry()
	p, _ := GetPreset("laravel")
	r.ApplyPreset(p)

	// An exact-path lure is now a decoy.
	if _, ok := r.IsDecoy("/storage/logs/laravel.log"); !ok {
		t.Error("laravel log path should be a decoy after ApplyPreset")
	}
	// A path under the storage/ zone is a decoy even though it wasn't listed.
	if _, ok := r.IsDecoy("/storage/framework/sessions/abc123"); !ok {
		t.Error("path under the laravel /storage/ zone should be a decoy")
	}
}

func TestApplyPresetNoteCarriesTriageContext(t *testing.T) {
	r := NewRegistry()
	p, _ := GetPreset("rails")
	r.ApplyPreset(p)
	e, ok := r.IsDecoy("/config/master.key")
	if !ok {
		t.Fatal("expected /config/master.key decoy")
	}
	if !strings.Contains(strings.ToLower(e.Note), "rails") {
		t.Errorf("note should explain the lure, got %q", e.Note)
	}
}

func TestEveryPresetHasPaths(t *testing.T) {
	for _, name := range PresetNames() {
		p, _ := GetPreset(name)
		if len(p.Paths) == 0 {
			t.Errorf("preset %q has no paths", name)
		}
		for _, e := range p.Paths {
			if !strings.HasPrefix(e.Path, "/") || e.Note == "" {
				t.Errorf("preset %q endpoint malformed: %+v", name, e)
			}
		}
		for _, e := range p.Prefixes {
			if !strings.HasSuffix(e.Path, "/") {
				t.Errorf("preset %q zone %q must end in '/'", name, e.Path)
			}
		}
	}
}
