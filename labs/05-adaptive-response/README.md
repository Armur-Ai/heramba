# Lab 05 — Policy-constrained adaptive response

See that defensive actions are bounded and deterministic: shadow mode never enforces, high-impact actions need approval, everything expires and rolls back, and no attacker text can change a policy decision.

## Objectives
- Observe shadow mode validating without enforcing.
- Prove injected text cannot flip a policy verdict.
- Walk an action through approval → expiry → rollback.

## Prerequisites
- Go 1.25+; this repository cloned. No network or external target is needed.
- Recommended: complete the preceding labs first.

## Setup
Nothing to install. The lab generates its own deterministic synthetic data.

## Run
```bash
go test -v ./labs/05-adaptive-response
```

## Tasks & expected observations
Follow the numbered steps printed by the test. Each `✓` line is an expected
observation; each check that fails prints a remediation hint prefixed
`LAB CHECK FAILED`.

## Validation & grading
The test **is** the automated grader: a green `ok` line means every objective
was met against the real engine. There is no separate answer key in this public
repo (see [../README.md](../README.md) on public vs. private materials).

## Reset
Re-run the command above. The lab is deterministic (fixed seeds); there is no
leftover state.

## Instructor notes
Model answers, scoring rubrics, and any private challenge variants are kept in
the separate private instructor repository, not here.
