package research

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDefaultProtocolIsValid(t *testing.T) {
	if err := DefaultProtocol().Validate(); err != nil {
		t.Fatalf("default protocol must validate, got: %v", err)
	}
}

func TestDefaultProtocolHasPreregisteredH1toH5(t *testing.T) {
	p := DefaultProtocol()
	want := map[string]bool{"H1": false, "H2": false, "H3": false, "H4": false, "H5": false}
	for _, h := range p.Hypotheses {
		if _, ok := want[h.ID]; ok {
			want[h.ID] = true
			if !h.Preregistered || h.Null == "" || h.Statement == "" {
				t.Errorf("%s not fully specified (prereg=%v null=%q)", h.ID, h.Preregistered, h.Null)
			}
		}
	}
	for id, found := range want {
		if !found {
			t.Errorf("missing pre-registered hypothesis %s", id)
		}
	}
}

func TestValidateCatchesSeedLeakage(t *testing.T) {
	p := DefaultProtocol()
	// Inject a test seed into a tuning family -> leakage.
	leaked := p.Families[4].Seeds[0] // a coord-test seed
	p.Families[0].Seeds = append(p.Families[0].Seeds, leaked)
	err := p.Validate()
	if !errors.Is(err, ErrSeedLeakage) {
		t.Fatalf("expected seed-leakage error, got: %v", err)
	}
}

func TestValidateCatchesMissingNull(t *testing.T) {
	p := DefaultProtocol()
	p.Hypotheses[0].Null = ""
	if err := p.Validate(); !errors.Is(err, ErrNullMissing) {
		t.Fatalf("expected missing-null error, got: %v", err)
	}
}

func TestValidateCatchesUnknownEndpoint(t *testing.T) {
	p := DefaultProtocol()
	p.Hypotheses[0].PrimaryEndpoint = "nonexistent_metric"
	if err := p.Validate(); !errors.Is(err, ErrUnknownEndpoint) {
		t.Fatalf("expected unknown-endpoint error, got: %v", err)
	}
}

func TestValidateCatchesUnknownActor(t *testing.T) {
	p := DefaultProtocol()
	p.Families[0].ActorModel = "ghost_actor"
	if err := p.Validate(); !errors.Is(err, ErrUnknownActor) {
		t.Fatalf("expected unknown-actor error, got: %v", err)
	}
}

func TestValidateRequiresHeldOut(t *testing.T) {
	p := DefaultProtocol()
	for i := range p.Families {
		p.Families[i].Partition = Train
		p.Families[i].HeldOut = false
	}
	if err := p.Validate(); !errors.Is(err, ErrNoHeldOut) {
		t.Fatalf("expected held-out error, got: %v", err)
	}
}

func TestValidateDetectsDuplicateHypothesisID(t *testing.T) {
	p := DefaultProtocol()
	dup := p.Hypotheses[0]
	p.Hypotheses = append(p.Hypotheses, dup)
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate hypothesis id") {
		t.Fatalf("expected duplicate-id error, got: %v", err)
	}
}

func TestPrimaryEndpointsExist(t *testing.T) {
	if len(DefaultProtocol().PrimaryEndpoints()) == 0 {
		t.Fatal("expected at least one primary endpoint")
	}
}

func TestTestFamiliesAreHeldOutAndNonEmpty(t *testing.T) {
	fams := DefaultProtocol().TestFamilies()
	if len(fams) == 0 {
		t.Fatal("expected held-out test families")
	}
	for _, f := range fams {
		if f.Partition != Test {
			t.Errorf("family %q in TestFamilies but partition=%s", f.Name, f.Partition)
		}
	}
}

func TestManifestRoundTrips(t *testing.T) {
	p := DefaultProtocol()
	var buf bytes.Buffer
	if err := p.WriteManifest(&buf); err != nil {
		t.Fatal(err)
	}
	var back Protocol
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatal(err)
	}
	if back.SchemaVersion != p.SchemaVersion || len(back.Hypotheses) != len(p.Hypotheses) {
		t.Fatalf("round-trip mismatch: got %d hypotheses, want %d", len(back.Hypotheses), len(p.Hypotheses))
	}
	if err := back.Validate(); err != nil {
		t.Fatalf("round-tripped protocol must still validate: %v", err)
	}
}

func TestActorByID(t *testing.T) {
	p := DefaultProtocol()
	if _, ok := p.ActorByID("coordinated_swarm"); !ok {
		t.Fatal("expected coordinated_swarm actor")
	}
	if _, ok := p.ActorByID("nope"); ok {
		t.Fatal("did not expect unknown actor")
	}
}
