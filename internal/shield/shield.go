// Package shield is Heramba's defense-tool orchestration seam. Just as Pentest
// Swarm is the centerpiece that coordinates offensive tools, Heramba is the
// centerpiece that coordinates a company's *defensive* tools:
//
//   - agat      — an artificial immune system for binaries (polymorphic /
//     metamorphic / AI-rewritten malware)
//   - byakugan  — a static scanner for ML model artifacts (backdoors on load)
//   - sharingan — a rogue / embedded-LLM footprint detector
//   - skillscan — Claude skill safety (planned)
//
// Each tool is ALSO a standalone CLI; Heramba never reimplements one. It invokes
// each through a thin argv adapter and aggregates the results, so the tools can
// be used independently OR composed into one defensive assessment. A tool that
// isn't installed is reported as an explicit, honest gap — never a silent clean
// result (Heramba's "shipped vs. planned" discipline).
package shield

import (
	"context"
	"os"
	"os/exec"
)

// Target is what a defensive scan assesses: a path to a directory, build
// artifact, container-export, model file, or repository on disk.
type Target struct {
	Path string
	Kind string // optional hint: dir|artifact|model|repo
}

// Status is a scanner's normalized outcome.
type Status string

const (
	StatusClean       Status = "clean"       // ran, found nothing
	StatusFindings    Status = "findings"    // ran, found something
	StatusUnavailable Status = "unavailable" // tool not installed / not yet shipped
	StatusError       Status = "error"       // tool ran but failed
)

// Result is one tool's normalized outcome. Raw holds the tool's own output so a
// later step can wrap it in an armur-contracts envelope without re-running it.
type Result struct {
	Tool     string `json:"tool"`
	Status   Status `json:"status"`
	Findings int    `json:"findings"`
	Detail   string `json:"detail,omitempty"`
	Raw      []byte `json:"-"`
}

// Scanner is one defensive capability Heramba orchestrates.
type Scanner interface {
	Name() string
	Scan(ctx context.Context, t Target) Result
}

// Registry runs a set of scanners over a target.
type Registry struct {
	Scanners []Scanner
}

// New builds a Registry from zero or more scanners.
func New(s ...Scanner) *Registry { return &Registry{Scanners: s} }

// Default is Heramba's standard defensive suite. Binary paths may be overridden
// per tool via HERAMBA_<TOOL>_BIN (e.g. HERAMBA_AGAT_BIN=/opt/agat).
func Default() *Registry {
	return New(
		AgatScanner{Bin: os.Getenv("HERAMBA_AGAT_BIN")},
		Byakugan(os.Getenv("HERAMBA_BYAKUGAN_BIN")),
		Sharingan(os.Getenv("HERAMBA_SHARINGAN_BIN")),
		SkillScan(os.Getenv("HERAMBA_SKILLSCAN_BIN")),
	)
}

// Run executes every scanner over the target and collects their results. A
// scanner that is unavailable or errors is recorded; it never aborts the run.
func (r *Registry) Run(ctx context.Context, t Target) []Result {
	out := make([]Result, 0, len(r.Scanners))
	for _, s := range r.Scanners {
		if err := ctx.Err(); err != nil {
			break
		}
		out = append(out, s.Scan(ctx, t))
	}
	return out
}

// resolveBin returns the path to a tool binary: an explicit override if set and
// executable, else a PATH lookup of name, else "".
func resolveBin(override, name string) string {
	if override != "" {
		if p, err := exec.LookPath(override); err == nil {
			return p
		}
		return ""
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}
