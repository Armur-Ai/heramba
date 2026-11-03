# Lab 03 — Stateful deception engineering

Stand up a coherent, seeded synthetic application, watch interactions get recorded as evidence, and confirm the deception world holds no production data to leak.

## Objectives
- Generate a deterministic synthetic world and verify every record is synthetic.
- Serve the deception app and confirm interaction telemetry is wired.
- Understand session-scoped state and production isolation.

## Prerequisites
- Go 1.25+; this repository cloned. No network or external target is needed.
- Recommended: complete the preceding labs first.

## Setup
Nothing to install. The lab generates its own deterministic synthetic data.

## Run
```bash
go test -v ./labs/03-stateful-deception
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
