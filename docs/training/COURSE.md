# Heramba training course map

A hands-on course on defending the application/API layer against coordinated
autonomous attacks, built entirely on the public [labs](../../labs/) and the
reproducible [research experiments](../research/). Everything runs locally
against the real engine — **no external targets, no network, no instructor-only
secrets** are required to complete any student lab.

> Instructor solution keys, private challenge flags, grading secrets, and
> confidential proposals live in a **separate private repository** and are never
> committed here. The `lab_test.go` graders assess understanding against real
> engine behavior without revealing private material.

## Who it is for

Security engineers, detection engineers, and researchers comfortable reading Go
and running `go test`. No prior Heramba knowledge assumed.

## Prerequisites (one-time, ~10 min)

- Go 1.25+ and this repository cloned.
- Verify the environment: `go test ./labs/...` should print `ok` for every lab.

That single command is the **fresh-machine dry run**: if it passes, every
student lab is runnable with no further setup.

## 2-day core course

Each lab is a `go test` that narrates the exercise and grades itself.

### Day 1 — Detection & intelligence (~5.5h hands-on)

| Block | Lab | Hands-on | Instructor dependency |
|---|---|---|---|
| 1 | 01 Baseline telemetry & detection | 45m | intro slides on the evidence model |
| 2 | 02 Distributed campaign reconstruction | 75m | walkthrough of observed-vs-inferred |
| 3 | 06 Coordination vs. automation (swarm) | 75m | discuss the confusion matrix |
| 4 | R2 fair coordination experiment (`heramba research coord`) | 60m | discuss matched workloads & ablations |

### Day 2 — Deception, response & robustness (~5.5h hands-on)

| Block | Lab | Hands-on | Instructor dependency |
|---|---|---|---|
| 1 | 03 Stateful deception engineering | 60m | consistency-validator discussion |
| 2 | 04 Business-logic attack detection | 60m | OpenAPI invariant modeling |
| 3 | 05 Policy-constrained adaptive response | 60m | threat-model the action set |
| 4 | 07 Prompt-injection-resistant controls | 45m | red-team the typed contract |
| 5 | R3 deception objective experiment (`heramba research objective`) | 45m | interpret the five-arm table |

## Optional 4-day extension

Days 1–2 as above, then:

### Day 3 — Co-evolution & the scientific method (~5h)

| Block | Activity | Hands-on |
|---|---|---|
| 1 | 08 Adaptation race & ablation study | 60m |
| 2 | R4 co-evolution lab (`heramba research coevolve`) + event-log reading | 75m |
| 3 | R8 targets-vs-observed scorecard (`heramba research stats`) | 60m |
| 4 | Pre-registration & leakage: the protocol manifest (`heramba research manifest`) | 60m |

### Day 4 — Production assurance & operations (~5h)

| Block | Activity | Hands-on |
|---|---|---|
| 1 | Proxy correctness & resilience (R6 suite, SLOs, fault injection) | 75m |
| 2 | Supply-chain assurance: fuzzing, SBOM, govulncheck (`make fuzz`, `make sbom`) | 60m |
| 3 | Operator investigation timeline & incident export (R9) | 75m |
| 4 | Capstone: run the full research suite and defend the honest limitations | 60m |

## Grading

Every lab is self-grading (`go test`). A module is "passed" when its labs print
`ok`. The research blocks are assessed by **interpretation**: students explain
the measured numbers *and* the declared limitations (a correct read of a MISS or
an underpowered interval counts as mastery, not a failure).

## Honesty note for instructors

Conference acceptance is not an engineering acceptance test. These materials
teach a real, tested system with honestly-reported limitations; do not present
synthetic-benchmark numbers as field performance.
