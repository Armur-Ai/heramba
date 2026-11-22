package dashboard

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Armur-Ai/heramba/internal/event"
)

func TestExportCSVHeadersAndBody(t *testing.T) {
	h := testHandler()
	rr := get(t, h, "/api/export/events.csv")
	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Errorf("content-type = %q, want text/csv", ct)
	}
	cd := rr.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment;") || !strings.Contains(cd, ".csv") {
		t.Errorf("content-disposition = %q, want a .csv attachment", cd)
	}
	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("cache-control = %q, want no-store", rr.Header().Get("Cache-Control"))
	}

	rows, err := csv.NewReader(strings.NewReader(rr.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if len(rows) != 1+len(sampleEvents()) {
		t.Fatalf("rows = %d, want header + %d events", len(rows), len(sampleEvents()))
	}
	if rows[0][0] != "time" || rows[0][1] != "kind" || rows[0][5] != "confidence" {
		t.Errorf("header = %v, unexpected columns", rows[0])
	}
	if rows[1][1] != string(event.KindDecoyHit) {
		t.Errorf("first data row kind = %q, want decoy_hit", rows[1][1])
	}
}

func TestExportJSONHeadersAndBody(t *testing.T) {
	h := testHandler()
	rr := get(t, h, "/api/export/events.json")
	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	cd := rr.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment;") || !strings.Contains(cd, ".json") {
		t.Errorf("content-disposition = %q, want a .json attachment", cd)
	}
	var evs []event.Event
	if err := json.Unmarshal(rr.Body.Bytes(), &evs); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if len(evs) != len(sampleEvents()) {
		t.Fatalf("events = %d, want %d", len(evs), len(sampleEvents()))
	}
}

func TestExportEmptyIsValid(t *testing.T) {
	h := Handler() // no event source
	rr := get(t, h, "/api/export/events.json")
	body := strings.TrimSpace(rr.Body.String())
	if body != "[]" {
		t.Errorf("empty export = %q, want []", body)
	}
	csvrr := get(t, h, "/api/export/events.csv")
	rows, err := csv.NewReader(strings.NewReader(csvrr.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse empty CSV: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("empty CSV rows = %d, want header only", len(rows))
	}
}
