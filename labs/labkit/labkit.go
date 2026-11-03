// Package labkit holds shared helpers for Heramba's hands-on labs. Each lab is
// a self-contained, locally-gradable exercise that runs against the real engine
// packages — no external target, no network, deterministic seeds so a lab
// resets simply by re-running it.
//
// Public vs private: this package and the labs under labs/ are student-safe
// infrastructure and fixtures. Instructor solution keys, private challenge
// flags, and unpublished research live in a SEPARATE private repository, never
// in this public tree. See labs/README.md.
package labkit

import (
	"fmt"
	"strings"
	"testing"
)

// Step prints a numbered lab step so a student running `go test -v` sees the
// exercise narrative alongside the automated checks.
func Step(t *testing.T, n int, msg string) {
	t.Helper()
	t.Logf("── step %d: %s", n, msg)
}

// Observe prints an expected observation (the "what you should see" line).
func Observe(t *testing.T, msg string, args ...any) {
	t.Helper()
	t.Logf("   ✓ %s", fmt.Sprintf(msg, args...))
}

// Require fails the lab with a clear remediation hint if a condition is false.
func Require(t *testing.T, cond bool, whatFailed string) {
	t.Helper()
	if !cond {
		t.Fatalf("LAB CHECK FAILED: %s", whatFailed)
	}
}

// Banner prints a lab title block.
func Banner(t *testing.T, title string) {
	t.Helper()
	t.Logf("\n%s\n%s", title, strings.Repeat("─", len(title)))
}
