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

func TestEveryPresetHasCanarySeeds(t *testing.T) {
	for _, name := range PresetNames() {
		p, _ := GetPreset(name)
		if len(p.Seeds) == 0 {
			t.Errorf("preset %q has no canary seeds", name)
		}
		for _, s := range p.Seeds {
			if s.Plant == "" || s.Value == "" {
				t.Errorf("preset %q seed malformed: %+v", name, s)
			}
			// Seeds must be inert examples, never a real-looking live secret by
			// accident: each carries an EXAMPLE marker or a known example value.
			if !strings.Contains(s.Value, "EXAMPLE") && s.Value != "0123456789abcdef0123456789abcdef" {
				t.Errorf("preset %q seed value should be an inert example: %q", name, s.Value)
			}
		}
	}
}

func TestApplyPresetWatchesSeedsAsCanaries(t *testing.T) {
	r := NewRegistry()
	p, _ := GetPreset("go")
	r.ApplyPreset(p)
	// The AWS example key seed must now be a watched canary.
	var seeded string
	for _, s := range p.Seeds {
		if strings.HasPrefix(s.Value, "AKIA") {
			seeded = s.Value
		}
	}
	if seeded == "" {
		t.Fatal("expected an AWS-key seed in the go preset")
	}
	if _, ok := r.Canaries().Check(seeded); !ok {
		t.Errorf("seed %q should be watched as a canary after ApplyPreset", seeded)
	}
	// A value that was never seeded must not be watched (negative case).
	if _, ok := r.Canaries().Check("AKIANOTSEEDED00000000"); ok {
		t.Error("an unseeded value must not be a watched canary")
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
