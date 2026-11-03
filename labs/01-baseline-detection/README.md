# Lab 01 — Baseline telemetry and detection

See how Heramba scores a source as agentic from transparent behavioral signals (no LLM on the path), why a decoy hit is a zero-false-positive detection, and how a flat detection becomes normalized evidence with provenance.

## Objectives
- Explain an agentic-traffic verdict from its component signals.
- Show that a decoy hit carries confidence 1.0 by construction.
- Lift a detection into the evidence model and persist it.

## Prerequisites
- Go 1.25+; this repository cloned. No network or external target is needed.
- Recommended: complete the preceding labs first.

## Setup
Nothing to install. The lab generates its own deterministic synthetic data.

## Run
```bash
go test -v ./labs/01-baseline-detection
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
