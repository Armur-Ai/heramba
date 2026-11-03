# Lab 02 — Distributed campaign reconstruction

Reconstruct a coordinated multi-source campaign from raw evidence, and see how Heramba keeps observed facts (sessions) distinct from inferred links (actors, campaigns) — never equating an IP with an identity.

## Objectives
- Reconstruct sessions, actors, and a campaign from an evidence stream.
- Confirm distinct IPs are not collapsed into one actor by a shared generic tool.
- Read an explanation that separates facts from hypotheses.

## Prerequisites
- Go 1.25+; this repository cloned. No network or external target is needed.
- Recommended: complete the preceding labs first.

## Setup
Nothing to install. The lab generates its own deterministic synthetic data.

## Run
```bash
go test -v ./labs/02-campaign-reconstruction
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
