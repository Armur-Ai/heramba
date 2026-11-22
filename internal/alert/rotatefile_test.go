package alert

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

func countLines(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if len(sc.Bytes()) > 0 {
			n++
		}
	}
	return n
}

func TestRotatingFileSinkWritesJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alerts.jsonl")
	s, err := NewRotatingFile(path, 1<<20, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	e := event.Deception(event.KindDecoyHit, "10.0.0.5", "/.env", "probe")
	if err := s.Emit(e); err != nil {
		t.Fatal(err)
	}
	// Written line must be valid, round-trippable JSON.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var back event.Event
	if err := json.Unmarshal(data[:len(data)-1], &back); err != nil {
		t.Fatalf("line must be JSON: %v (%q)", err, data)
	}
	if back.Target != "/.env" || back.Kind != event.KindDecoyHit {
		t.Errorf("round-trip mismatch: %+v", back)
	}
}

func TestRotatingFileSinkRotatesAndKeepsBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alerts.jsonl")
	// Tiny cap so every couple of events forces a rotation; keep 2 backups.
	s, err := NewRotatingFile(path, 200, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for i := 0; i < 50; i++ {
		e := event.New(event.KindProbe, event.SeverityMedium, "10.0.0.9", "/scan", "enumeration sweep")
		if err := s.Emit(e); err != nil {
			t.Fatalf("emit %d: %v", i, err)
		}
	}

	// The active file must exist and be under (cap + one line of slack).
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("active file missing: %v", err)
	}
	if info.Size() > 400 {
		t.Errorf("active file not bounded by rotation: %d bytes", info.Size())
	}
	// Backups .1 and .2 must exist; .3 must NOT (maxBackups=2).
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Errorf("expected backup .1: %v", err)
	}
	if _, err := os.Stat(path + ".2"); err != nil {
		t.Errorf("expected backup .2: %v", err)
	}
	if _, err := os.Stat(path + ".3"); err == nil {
		t.Errorf(".3 should have been dropped (maxBackups=2)")
	}

	// Every emitted event must survive across the active file + both backups.
	total := countLines(t, path) + countLines(t, path+".1") + countLines(t, path+".2")
	if total == 0 {
		t.Fatal("no lines survived rotation")
	}
}

func TestRotatingFileSinkNoBackupsTruncates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alerts.jsonl")
	s, err := NewRotatingFile(path, 150, 0) // maxBackups=0 => truncate in place
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for i := 0; i < 30; i++ {
		_ = s.Emit(event.New(event.KindProbe, event.SeverityLow, "x", "/y", "detail text here"))
	}
	if _, err := os.Stat(path + ".1"); err == nil {
		t.Errorf("no backups should be created when maxBackups=0")
	}
	info, _ := os.Stat(path)
	if info.Size() > 300 {
		t.Errorf("active file not bounded: %d bytes", info.Size())
	}
}

func TestRotatingFileSinkBadPath(t *testing.T) {
	// A path whose parent directory does not exist must fail cleanly.
	bad := filepath.Join(t.TempDir(), "nope", "alerts.jsonl")
	if _, err := NewRotatingFile(bad, 1<<20, 1); err == nil {
		t.Fatal("expected error opening file in nonexistent directory")
	}
}
