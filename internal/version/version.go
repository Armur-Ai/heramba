// Package version carries build metadata, stamped at release time via -ldflags.
package version

import (
	"fmt"
	"runtime"
)

// These are overridden at build time:
//
//	go build -ldflags "-X github.com/Armur-Ai/heramba/internal/version.Version=v0.1.0 ..."
var (
	Version = "0.0.0-dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a one-line human-readable version banner.
func String() string {
	return fmt.Sprintf("heramba %s (commit %s, built %s, %s)", Version, Commit, Date, runtime.Version())
}
