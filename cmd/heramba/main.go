// Command heramba is the CLI for the Heramba active-deception defense engine.
//
// Heramba defends the application/API layer that autonomous attack swarms
// target, by denying the attacker clean ground-truth signal and trapping it in
// deception — rather than matching payloads (WAF) or watching hosts (EDR).
//
// Safety: the engine defaults to OBSERVE mode (detection only, nothing inline).
// Inline PROTECT mode is explicit opt-in.
package main

import (
	"fmt"
	"os"

	"github.com/Armur-Ai/heramba/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		runServe(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println(version.String())
	case "help", "-h", "--help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "heramba: unknown command %q\n\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(2)
	}
}

func usage(w *os.File) {
	fmt.Fprint(w, `heramba — active-deception defense for the app/API layer

usage:
  heramba <command> [flags]

commands:
  serve        run the edge runtime (observe mode by default)
  decoys       generate and list decoys / canary tokens
  config       create or inspect configuration
  doctor       check the environment and configuration
  version      print version
  help         show this help

Heramba starts in OBSERVE mode (detection only). Inline PROTECT mode is opt-in.
Docs: https://github.com/Armur-Ai/heramba
`)
}
