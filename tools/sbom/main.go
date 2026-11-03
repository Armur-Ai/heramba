// Command sbom generates a minimal CycloneDX 1.5 software bill of materials for
// the Heramba module from `go list -m -json all`. It is self-contained (no
// external tooling) and deterministic (components sorted), so the committed SBOM
// is reproducible and verifiable: regenerate with `make sbom` and diff.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"
)

type goMod struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Main    bool   `json:"Main"`
}

type component struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
	PURL    string `json:"purl,omitempty"`
}

type bom struct {
	BOMFormat   string         `json:"bomFormat"`
	SpecVersion string         `json:"specVersion"`
	Version     int            `json:"version"`
	Metadata    map[string]any `json:"metadata"`
	Components  []component    `json:"components"`
}

func main() {
	out, err := exec.Command("go", "list", "-m", "-json", "all").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sbom: go list failed: %v\n", err)
		os.Exit(1)
	}
	dec := json.NewDecoder(bufio.NewReader(bytes.NewReader(out)))
	var mainMod string
	var comps []component
	for dec.More() {
		var m goMod
		if err := dec.Decode(&m); err != nil {
			fmt.Fprintf(os.Stderr, "sbom: decode: %v\n", err)
			os.Exit(1)
		}
		if m.Main {
			mainMod = m.Path
			continue
		}
		c := component{Type: "library", Name: m.Path, Version: m.Version}
		if m.Version != "" {
			c.PURL = "pkg:golang/" + m.Path + "@" + m.Version
		}
		comps = append(comps, c)
	}
	sort.Slice(comps, func(i, j int) bool {
		if comps[i].Name != comps[j].Name {
			return comps[i].Name < comps[j].Name
		}
		return comps[i].Version < comps[j].Version
	})

	// Deterministic timestamp: honor SOURCE_DATE_EPOCH if set, else omit.
	meta := map[string]any{
		"component": map[string]any{"type": "application", "name": mainMod},
		"tools":     []map[string]any{{"name": "heramba-sbom", "vendor": "Armur"}},
	}
	if sde := os.Getenv("SOURCE_DATE_EPOCH"); sde != "" {
		meta["timestamp"] = sde
	} else {
		_ = time.Now // intentionally omitted for reproducibility
	}

	b := bom{
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.5",
		Version:     1,
		Metadata:    meta,
		Components:  comps,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(b); err != nil {
		fmt.Fprintf(os.Stderr, "sbom: encode: %v\n", err)
		os.Exit(1)
	}
}
