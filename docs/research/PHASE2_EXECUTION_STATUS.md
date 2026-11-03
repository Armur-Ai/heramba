# Heramba — Phase 2 Execution Status

Authoritative journal for the **Phase 2 research, adversarial validation, and
production assurance** directive (checkpoints R0–R11). It layers scientific
controls, robustness, reliability engineering, and an advanced teaching platform
on top of the verified 12-capability foundation (CP0–CP10).

> **Repository state and executed tests supersede this journal.** A checkpoint
> is `VERIFIED` only once its acceptance tests pass in-tree. Nothing here claims
> a benchmark that is not produced by a committed, runnable harness.

Status legend: `NOT_STARTED` · `IN_PROGRESS` · `BLOCKED` · `PARTIAL` · `VERIFIED`.

| R | Title | Status | Commit | Evidence |
|---|---|---|---|---|
| R0 | Safe handoff & gap analysis | VERIFIED | `38f533d` | baseline below |
| R1 | Research protocol & threat model | VERIFIED | this commit | `go test ./internal/research/...`; `heramba research manifest` |
| R2 | Fair coordination detection experiment | NOT_STARTED | — | — |
| R3 | Deception & attack-objective experiment | NOT_STARTED | — | — |
| R4 | Adaptation race & co-evolution lab | NOT_STARTED | — | — |
| R5 | Defender prompt-injection & data-plane safety | NOT_STARTED | — | — |
| R6 | Proxy correctness & resilience | NOT_STARTED | — | — |
| R7 | Supply chain & release assurance | NOT_STARTED | — | — |
| R8 | Scientific statistics & targets | NOT_STARTED | — | — |
| R9 | Operations, explainability & human control | NOT_STARTED | — | — |
| R10 | Advanced training package | NOT_STARTED | — | — |
| R11 | Final research & release review | NOT_STARTED | — | — |

---

## R0 — Safe handoff & gap analysis — VERIFIED (2026-11-03)

### Concurrency / handoff check

The original master-prompt session has **completed and committed** all of
CP0–CP10. At handoff:

- `git status` → `On branch main`, up to date with `origin/main`, **working
  tree clean** (no concurrent writer).
- `HEAD` = `bc61202` ("docs: deep-dive guides for evidence, campaign
  intelligence, strategy").

No conflict with concurrent work. Safe to proceed on `main` with focused,
additive commits (directive permits this once the prior session has committed
its verified work and the tree is clean).

### Baseline build / test performance

All commands run with `go1.27.2 darwin/arm64`.

| Command | Result |
|---|---|
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `go test -race ./...` (cold cache) | **28 packages pass, 0 fail, 0 races**, 13.18s wall |
| Codebase | 93 Go files, 10,647 LOC, 31 packages |

No pre-existing test failures. No known flakes observed across repeated runs.

### Gap matrix — original 12 capabilities (foundation)

All verified in CP0–CP10 and re-confirmed green at R0; Phase 2 does **not**
reimplement them, it adds controls and evaluation around them.

| # | Capability | Package | State |
|---|---|---|---|
| 01 | Edge runtime / reverse proxy | `internal/edge` | EXISTS (verified) |
| 02 | Decoys + canaries | `internal/decoy`, `internal/counter` | EXISTS (verified) |
| 03 | Agentic classifier | `internal/classify` | EXISTS (verified) |
| 04 | Signal denial | `internal/signal` | EXISTS (verified) |
| 05 | Egress firewall + rate limit | `internal/egress`, `internal/ratelimit` | EXISTS (verified) |
| 06 | Evidence model | `internal/evidence` | EXISTS (verified) |
| 07 | Campaign reconstruction | `internal/campaign` | EXISTS (verified) |
| 08 | Swarm + active learning | `internal/campaign/{swarm,experiment}` | EXISTS (verified) |
| 09 | Multi-sensor correlation | `internal/evidence/sensor` | EXISTS (verified) |
| 10 | Strategy + containment | `internal/strategy` | EXISTS (verified) |
| 11 | Deception app + asset/bizlogic | `internal/{deception,assetgraph,bizlogic}` | EXISTS (verified) |
| 12 | Replay evaluation + labs | `internal/replay`, `labs/` | EXISTS (verified) |

### Gap matrix — Phase 2 additions (this directive)

| R | Deliverable | Existing leverage | Gap | Verify command (target) |
|---|---|---|---|---|
| R1 | Research protocol, threat model, pre-registered H1–H5, scenario/hypothesis manifests, schema validation | `replay.Scenario`, evidence vocab | **MISSING** — no protocol package or manifests | `go test ./internal/research/...` |
| R2 | Fair coordination experiment: matched workloads, 4 methods (request-only/static/graph/full), ablations, CIs, one command | `swarm`, `campaign`, `replay` | **MISSING** — need matched-workload generator + method harness + ablation runner | `heramba research coord` |
| R3 | Deception objective experiment: 5 arms, consistency validator, deception-aware agents, isolation tests | `deception`, `replay`, `assetgraph` | **PARTIAL** — 4-baseline exists; need unprotected/conventional arms, consistency validator, deception-aware adversary, isolation proof tests | `heramba research objective` |
| R4 | Adaptation-race lab: local simulator, event sourcing, t_change…t_objective, adaptation latency, utility/oscillation | `strategy`, `replay` | **MISSING** — need co-evolution loop + event log + latency decomposition | `heramba research coevolve` |
| R5 | Taint/provenance labels, typed contracts, adversarial defender-safety tests (forged roles, laundering, log injection, replay, cross-tenant) | `strategy` (injection-proof policy), `evidence.Provenance` | **PARTIAL** — policy is injection-proof; need taint labels + a dedicated adversarial test battery | `go test ./internal/taint/... ./internal/strategy/...` |
| R6 | Proxy correctness & resilience suite + SLOs + fault injection | `internal/edge` | **PARTIAL** — basic edge tests exist; need chunked/streaming/gzip/keep-alive/timeout/upstream-outage/cancellation/shutdown + latency SLO measurement | `go test ./internal/edge/...` |
| R7 | CI, dependency scan, SBOM, fuzz, SECURITY policy, supported versions, dep update policy | `CHANGELOG.md`, `SECURITY.md` | **PARTIAL** — SECURITY/CHANGELOG exist; need CI workflow, SBOM generator, fuzz targets, SBOM artifact | `go test -run Fuzz`, CI run, `make sbom` |
| R8 | Stats: Wilson/bootstrap CIs, stratification, effect sizes, reproducible table scripts, targets-vs-observed | `replay.Aggregate` | **MISSING** — need a stats package + reproduce scripts | `go test ./internal/research/stats/...` |
| R9 | Operator investigation timeline (obs→fact→hyp→proposed→approved→outcome), exportable incident reports, retention | `intel`, `strategy` audit, `sensor.IncidentTimeline` | **PARTIAL** — timeline + audit exist; need full causal chain export + incident report + retention policy | `go test ./internal/incident/...` |
| R10 | 6+ labs incl. prompt-injection-resistant controls + adaptation-race; 2-day/4-day course map | `labs/01..06` | **PARTIAL** — 6 labs exist; need prompt-injection lab, adaptation-race lab, course map | fresh-machine `go test ./labs/...` |
| R11 | research-report.md, reproducibility.md, assurance-report.md, 12+Phase2 capability matrix | all above | **MISSING** — produced last from real artifacts | independent rerun |

### Principles carried forward (non-negotiable)

- No LLM on the enforcement path; attacker-controlled bytes never reach a
  trusted field (R5 hardens this with explicit taint labels).
- No fabricated tests or benchmarks; metrics the harness cannot honestly measure
  are marked "not measured".
- Synthetic-only adversarial activity; strictly local, isolated, authorized; no
  real-world/third-party targeting, no deployable offensive payloads.
- Observed (sessions/facts) vs inferred (actors/campaigns/hypotheses) stays
  structural; detection of coordination ≠ attribution to an LLM.
- Targets (R8) are aspirational evaluation thresholds, never claims.

### Next action

Proceed to **R1**: build `internal/research` with a schema-validated protocol,
threat-model manifest, and pre-registered hypotheses H1–H5.

---

## R1 — Research protocol & threat model — VERIFIED (2026-11-03)

**Files:** `internal/research/{protocol,default,protocol_test}.go`,
`cmd/heramba/research.go` (+`main.go` wiring), `docs/research/{protocol,threat-model}.md`,
`docs/research/protocol.manifest.json` (generated).

**What shipped:** the protocol is **code, not prose** — a schema-validated
`research.Protocol` with pre-registered H1-H5 (each with an explicit null),
six threat-model actor families (scripted / independent / coordinated /
legit-automation / human / deception-aware) with explicit capabilities, request
and time budgets, objectives, and defender observability; 13 endpoints (5
primary); leakage-free train/tune/validation/test partitions; stat methods,
stopping and leakage rules. `Protocol.Validate()` rejects duplicate IDs,
non-falsifiable hypotheses, unknown endpoints/actors, missing held-out families,
and **train/test seed leakage** — each proved by a negative test.

**Commands / results:**
- `go test ./internal/research/...` → `ok` (12 tests incl. leakage/null/dup/held-out negatives)
- `go vet ./...` → clean; `go test -race ./...` → research `ok`, full suite PASS
- `heramba research manifest` → validated JSON; `--out docs/research/protocol.manifest.json` (464 lines)

**Limitations:** the deception-aware actor is a modeled heuristic, not a live
LLM; synthetic traffic is more separable than production (declared in
`threats_to_validity`).

**Next action:** R2 — the fair coordination-detection experiment on these
families (4 methods + ablations, CIs, one command).
