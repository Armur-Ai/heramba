# Heramba Labs

Six progressive, self-contained exercises that teach Heramba's defense model
against autonomous attack agents. Every lab runs, resets, and grades **locally**
against the real engine packages — no external target, no network, no cloud.

## Running a lab

Each lab is a Go test that doubles as its automated grader. Run one with `-v` to
see the exercise narrative and the checks:

```bash
go test -v ./labs/01-baseline-detection
```

Run them all (the full grade):

```bash
go test ./labs/...
```

Labs are deterministic (fixed seeds), so **resetting a lab is just re-running
it** — there is no state to clean up.

## The six labs

| # | Lab | Teaches | Engine package |
|---|-----|---------|----------------|
| 01 | [Baseline telemetry & detection](01-baseline-detection) | deterministic agentic classification, zero-FP decoy hits, the evidence model | `classify`, `event`, `evidence` |
| 02 | [Distributed campaign reconstruction](02-campaign-reconstruction) | sessionization, entity resolution, observed-vs-inferred | `campaign` |
| 03 | [Stateful deception engineering](03-stateful-deception) | coherent synthetic worlds, session state, production isolation | `deception` |
| 04 | [Business-logic attack detection](04-business-logic) | BOLA/IDOR, workflow-sequence abuse, authorized-equivalent allowance | `bizlogic` |
| 05 | [Policy-constrained adaptive response](05-adaptive-response) | shadow mode, approval/expiry/rollback, injection-proof policy | `strategy` |
| 06 | [Swarm-versus-defense benchmarking](06-swarm-benchmark) | the reproducible four-baseline evaluation | `replay`, `campaign/swarm` |

A suggested order is 01 → 06; each builds intuition the next relies on.

## Public vs. private materials

This directory is **student-safe** infrastructure and fixtures, published in the
open-source repo. The following are **deliberately not here** and live in a
separate private instructor repository:

- instructor solution keys and model answers,
- private challenge flags / scoring secrets,
- unpublished research artifacts and confidential submissions.

The automated checks in each `lab_test.go` grade a student's understanding
against the real engine behavior without revealing any private material.
