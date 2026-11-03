# Lab 08 — Adaptation race & ablation study

Measure the defender's adaptation latency against a bounded attacker strategy
shift, decompose it into detection and response components, and run the
no-adaptation / adaptive / instant ablation to see where adaptation helps — and,
against a feinting adversary, where it hurts.

## Objectives
- Measure adaptation latency and confirm positive defender utility on a
  detectable shift.
- Decompose latency into detection + response components.
- See detection latency grow (gracefully) as the adversary gets stealthier.
- Run the ablation and observe policy oscillation against a feinting adversary.

## Prerequisites
- Go 1.25+; this repository cloned. No network or external target is needed.
- Recommended: Labs 02 and 06 (campaign + swarm) first.

## Expected duration
~25 minutes.

## Setup / reset
Nothing to install. The simulation is seeded and event-sourced, so runs are
deterministic and need no reset.

## Run
```bash
go test -v ./labs/08-adaptation-race
```

## Tasks & expected observations
Follow the numbered steps; each `✓` is an expected observation. The printed
latency/utility numbers are measured from the real `internal/research/coevolve`
lab, not hard-coded.

## Validation & grading
The test **is** the grader. A green `ok` means the adaptation and ablation
properties held. No answer key ships in this public repo.

## Troubleshooting
- *A utility/oscillation check fails* — ensure you did not change
  `coevolve.DefaultParams`; the thresholds assume the shipped defaults.

## Cleanup
None.
