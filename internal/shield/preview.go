package shield

import (
	"context"
	"encoding/json"
	"os/exec"
)

// previewScanner is the shared argv adapter for defense tools whose engines are
// still in development (byakugan, sharingan) or not yet built (skillscan). The
// invocation seam is real now so these compose into Heramba the moment their
// binaries ship: when present, Heramba runs `<bin> scan <path> --json` and
// counts the top-level findings; when absent, it reports an honest gap with a
// pointer to what the tool will do.
type previewScanner struct {
	name    string
	bin     string // override; empty resolves name on PATH
	planned string // honest "not yet shipped" detail
}

func (p previewScanner) Name() string { return p.name }

func (p previewScanner) Scan(ctx context.Context, t Target) Result {
	bin := resolveBin(p.bin, p.name)
	if bin == "" {
		return Result{Tool: p.name, Status: StatusUnavailable, Detail: p.planned}
	}
	// #nosec G204 — bin is PATH-resolved; t.Path is an operator-supplied scan target.
	out, err := exec.CommandContext(ctx, bin, "scan", t.Path, "--json").Output()
	n, ok := countJSONArray(out)
	if !ok {
		detail := p.name + " produced no parseable output"
		if err != nil {
			detail = p.name + " run failed: " + err.Error()
		}
		return Result{Tool: p.name, Status: StatusError, Detail: detail, Raw: out}
	}
	st := StatusClean
	if n > 0 {
		st = StatusFindings
	}
	return Result{Tool: p.name, Status: st, Findings: n, Raw: out}
}

// countJSONArray counts the entries of a top-level JSON array. ok is false when
// the bytes are not a JSON array (so the caller can distinguish a failure from a
// clean empty result).
func countJSONArray(b []byte) (int, bool) {
	if len(b) == 0 {
		return 0, false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(b, &arr); err != nil {
		return 0, false
	}
	return len(arr), true
}

// Byakugan is the adapter for the ML model-artifact backdoor scanner.
func Byakugan(bin string) Scanner {
	return previewScanner{
		name:    "byakugan",
		bin:     bin,
		planned: "byakugan (ML model-artifact backdoor scanner) is a design preview — wire it in once its engine ships (github.com/Armur-Ai/byakugan)",
	}
}

// Sharingan is the adapter for the rogue / embedded-LLM footprint detector.
func Sharingan(bin string) Scanner {
	return previewScanner{
		name:    "sharingan",
		bin:     bin,
		planned: "sharingan (rogue / embedded-LLM footprint detector) is a design preview — wire it in once its engine ships (github.com/Armur-Ai/sharingan)",
	}
}

// SkillScan is the adapter for Claude skill safety scanning.
func SkillScan(bin string) Scanner {
	return previewScanner{
		name:    "skillscan",
		bin:     bin,
		planned: "skillscan (Claude skill safety) is planned — not yet available",
	}
}
