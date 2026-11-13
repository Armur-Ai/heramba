package shield

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// AgatScanner is the thin argv adapter for agat — an artificial immune system
// that catches polymorphic / metamorphic / AI-rewritten malware by capability,
// not signature. Heramba invokes `agat scan <path> --json` and counts the
// malicious/suspicious verdicts; agat exits non-zero when it finds malware,
// which is a signal, not an adapter error.
type AgatScanner struct {
	Bin string // override; empty resolves "agat" on PATH
}

func (AgatScanner) Name() string { return "agat" }

func (a AgatScanner) Scan(ctx context.Context, t Target) Result {
	bin := resolveBin(a.Bin, "agat")
	if bin == "" {
		return Result{
			Tool:   "agat",
			Status: StatusUnavailable,
			Detail: "agat not found on PATH — install it to scan binaries for malware (github.com/Armur-Ai/agat)",
		}
	}
	// #nosec G204 — bin is PATH-resolved; t.Path is an operator-supplied scan target.
	out, runErr := exec.CommandContext(ctx, bin, "scan", t.Path, "--json").Output()
	verdicts, parseErr := parseAgatJSON(out)
	if parseErr != nil {
		// No parseable output — a real failure, not a verdict.
		detail := "agat produced no parseable output"
		if runErr != nil {
			detail = "agat run failed: " + runErr.Error()
		}
		return Result{Tool: "agat", Status: StatusError, Detail: detail, Raw: out}
	}
	flagged := 0
	for _, v := range verdicts {
		switch v.Verdict.Severity {
		case "malicious", "suspicious":
			flagged++
		}
	}
	st := StatusClean
	if flagged > 0 {
		st = StatusFindings
	}
	return Result{
		Tool:     "agat",
		Status:   st,
		Findings: flagged,
		Detail:   fmt.Sprintf("%d of %d sample(s) flagged malicious/suspicious", flagged, len(verdicts)),
		Raw:      out,
	}
}

// agatVerdict is the subset of agat's --json output we consume.
type agatVerdict struct {
	Verdict struct {
		Severity string `json:"severity"` // malicious|suspicious|clean
	} `json:"verdict"`
}

func parseAgatJSON(b []byte) ([]agatVerdict, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("empty output")
	}
	var vs []agatVerdict
	if err := json.Unmarshal(b, &vs); err != nil {
		return nil, err
	}
	return vs, nil
}
