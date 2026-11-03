# Lab 07 — Prompt-injection-resistant defender controls

Prove that attacker-controlled text can never change a policy decision or gain an
execution privilege. You decode hostile input through the typed contract, show
the free text is quarantined, and confirm the containment decision is identical
with or without an injection.

## Objectives
- See that attacker-controlled values are never accessible on the trusted path.
- Decode untrusted input through the typed observation contract.
- Prove an injected note cannot change a containment decision.
- Watch schema confusion and off-allowlist tools fail closed.

## Prerequisites
- Go 1.25+; this repository cloned. No network or external target is needed.
- Recommended: Lab 05 (policy-constrained response) first.

## Expected duration
~20 minutes.

## Setup / reset
Nothing to install or reset — the lab builds its own inputs. Re-running is idempotent.

## Run
```bash
go test -v ./labs/07-prompt-injection-defense
```

## Tasks & expected observations
Follow the numbered steps the test prints. Each `✓` line is an expected
observation; a failed check prints `LAB CHECK FAILED` with a hint.

## Validation & grading
The test **is** the grader: a green `ok` means every objective held against the
real `internal/taint` package. No answer key ships in this public repo.

## Troubleshooting
- *Build error about `internal/taint`* — run from the repo root so the module
  resolves.
- *A step fails* — read the `LAB CHECK FAILED` message; it names the invariant
  that did not hold.

## Cleanup
None — the lab writes no files and starts no servers.
