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

// BuildInfo is the machine-readable build metadata, for `heramba version --json`
// and any tooling that wants to parse it rather than the banner string.
type BuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Go      string `json:"go"`
}

// Info returns the current build metadata.
func Info() BuildInfo {
	return BuildInfo{Version: Version, Commit: Commit, Date: Date, Go: runtime.Version()}
}
