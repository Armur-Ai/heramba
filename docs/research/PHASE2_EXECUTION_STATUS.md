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
| R1 | Research protocol & threat model | VERIFIED | `be6db33` | `go test ./internal/research/...`; `heramba research manifest` |
| R2 | Fair coordination detection experiment | VERIFIED | `fe40522` | `go test ./internal/research/{coord,stats}/...`; `heramba research coord` |
| R3 | Deception & attack-objective experiment | VERIFIED | `679597c` | `go test ./internal/research/objective/...`; `heramba research objective` |
| R4 | Adaptation race & co-evolution lab | VERIFIED | `ec3664c` | `go test ./internal/research/coevolve/...`; `heramba research coevolve` |
| R5 | Defender prompt-injection & data-plane safety | VERIFIED | `c22524a` | `go test ./internal/taint/...` |
| R6 | Proxy correctness & resilience | VERIFIED | `1aa9b17` | `go test ./internal/edge/...` |
| R7 | Supply chain & release assurance | VERIFIED | `bfc8f36` | `make fuzz`; `make sbom`; CI |
| R8 | Scientific statistics & targets | VERIFIED | `b332f94` | `go test ./internal/research/{stats,scorecard}/...`; `heramba research stats` |
| R9 | Operations, explainability & human control | VERIFIED | `44ee931` | `go test ./internal/incident/... ./internal/alert/...` |
| R10 | Advanced training package | VERIFIED | `93557d5` | `go test ./labs/...` (fresh-machine dry run) |
| R11 | Final research & release review | VERIFIED | `46a9774` | reports + `go test -race ./...` rerun |

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

---

## R2 — Fair coordination-detection experiment — VERIFIED (2026-11-03)

**Files:** `internal/research/stats/{stats,stats_test}.go`,
`internal/research/coord/{workload,methods,run,report,coord_test}.go`,
`cmd/heramba/research_coord.go`, `docs/research/coordination-experiment.md`,
`docs/research/artifacts/coord/` (reference bundle).

**What shipped:** matched-workload generator (every cohort = 200 requests / 5m
window, so aggregate rate is not discriminative); four methods
(request-only / static-correlation / graph-correlation / full-Heramba) and four
ablations over identical cohorts; a stats package (Wilson intervals, bootstrap,
PR-AUC, confusion) with its own tests. Evaluation on **held-out** test families
(seeds 9000+, incl. the deception-aware swarm) plus rate-matched benign-control
negatives. `heramba research coord` is the single command; `--artifacts` writes
the manifest + raw + summary bundle.

**Results (revision `be6db33`, 52 cohorts, 20 coordinated):**
full-Heramba PR-AUC **0.935**, recall **1.00 [0.84,1.00]**, FPR **0.09 [0.03,0.24]**
— strictly beats request-only (0.192), static (0.192), graph (0.692). **H1 null
rejected.** Ablations: dropping cross-source → recall 0.00; dropping synchrony →
FPR 1.00; dropping decoy → PR-AUC unchanged (coordination caught without lures;
deception-aware swarm 10/10 TP).

**Commands / results:**
- `go test ./internal/research/{coord,stats}/...` → ok (incl. agreement-with-shipped-detector, determinism, H1, ablation tests)
- `go test ./...` → ALL PASS; `go test -race ./internal/research/...` → ok, no races
- metrics byte-identical across reruns (throughput excluded, it is a timing measurement)

**Limitations (reported, not hidden):** FPR 9.4% exceeds the 1% aspirational
target — benign clients that hit the same endpoints in a tight window look
synchronized+shared to the shipped detector's coordinated branch. Faithful to
CP6; enumeration-gating is the tracked mitigation. Synthetic traffic is more
separable than production.

**Next action:** R3 — deception & attack-objective experiment (5 arms,
consistency validator, deception-aware adversary, real/synthetic isolation tests).

---

## R3 — Deception & attack-objective experiment — VERIFIED (2026-11-03)

**Files:** `internal/research/objective/{consistency,experiment,run,report,objective_test}.go`,
`cmd/heramba/research_objective.go`, `docs/research/deception-experiment.md`,
`docs/research/artifacts/objective/`.

**Real, tested safety pieces:** `ValidateWorld` checks the actual
`deception.World` across identifiers / permissions / workflows / temporal state /
synthetic tag — asserted to pass on generated worlds and to catch injected
dangling refs, non-synthetic records, and real-looking email domains.
Real/synthetic isolation proven: synthetic emails use RFC-reserved domains only;
`assetgraph` real graph fails closed on `WriteAdversaryView`, synthetic exports.
`Reproducible` asserts byte-identical regeneration from a seed.

**Modeled five-arm comparison (labeled a model):** unprotected / conventional /
static-decoys / stateful / adaptive, matched depth+budget, vs naive and
deception-aware adversaries. `heramba research objective [--deception-aware]`.

**Results (fe40522, 500 trials/arm):** naive objective completion
1.00 → 1.00 → 0.88 → 0.40 → **0.00** (adaptive) — **H2 null rejected**, >99%
relative reduction, zero benign disruption for all deception arms (only the rate
limiter disrupts benign, 5%). Deception-aware: static decoys **discovered 100%,
never detected** (worthless vs a smart adversary) while stateful/adaptive are
never discovered (0.00) and keep completion at 0.40 / 0.00 — the payoff of the
consistency validator.

**Commands / results:** `go test ./internal/research/objective/...` ok (incl.
consistency-catch, isolation fail-closed, H2 naive + aware, benign-disruption
tests); `go test ./...` PASS; `-race` ok.

**Limitations:** the five-arm comparison is a behavioral model, not the live
engine; the aware adversary is a threshold heuristic, not an LLM; CPU/mem not
measured here (R6 covers proxy overhead).

**Next action:** R4 — adaptation race & co-evolution lab (event-sourced,
t_change…t_objective, adaptation latency, utility/oscillation).

---

## R4 — Adaptation race & co-evolution lab — VERIFIED (2026-11-03)

**Files:** `internal/research/coevolve/{coevolve,run,report,coevolve_test}.go`,
`cmd/heramba/research_coevolve.go`, `docs/research/coevolution-lab.md`,
`docs/research/artifacts/coevolve/` (summary + replayable event log).

**What shipped:** a strictly-local, seeded, **event-sourced** adaptation race —
no live targets, no exploitation, no propagation, only logical ticks and labeled
events. Captures t_change…t_effective/t_objective; **adaptation latency =
t_effective − t_change**, decomposed into detection and response components.
Five bounded archetypes (scripted / independent / coordinated / deception-aware /
feinting adversarial-varied) vs three defender modes (adaptive / no-adaptation /
instant). Utility = objective-prevention − update costs − oscillation penalty.
`heramba research coevolve`.

**Results (679597c, 8 rounds × 100 seeds):** adaptive counters detectable shifts
100% with **positive utility 0.93** (H3 null rejected); adaptation latency
bounded and degrades gracefully (9→14s) as detectability falls; the **response
component is a fixed 4.0s, inside the ≤5s target** (R8), detection is the variable
part. The **feinting adversary exposes the limit**: 67% countered, 50% policy
oscillation, utility 0.45 — honestly reported. Ablation: no-adaptation utility 0;
adaptive matches the instant bound for stable adversaries.

**Commands / results:** `go test ./internal/research/coevolve/...` ok (replay
determinism, pipeline-order, H3, latency decomposition, oscillation, report);
`go test ./...` PASS; `-race` ok.

**Limitations:** logical ticks (not wall-clock); seeded bounded jitter;
detectability is a parameter, not a claim about a real adversary.

**Next action:** R5 — defender prompt-injection & data-plane safety (taint
labels + typed contracts + adversarial test battery).

---

## R5 — Defender prompt-injection & data-plane safety — VERIFIED (2026-11-03)

**Files:** `internal/taint/{taint,contract,taint_test}.go`,
`docs/research/data-plane-safety.md`.

**What shipped:** a taint layer where every value carries a trust label
(Trusted/AttackerControlled/Derived) + provenance, and the only decision
accessor (`Trusted`/`RequireTrusted`) refuses untrusted content — there is no
trusted accessor that returns attacker-controlled data, so injection is a type
error. Taint propagates through derivation (no laundering). A typed
observation/action contract decodes untrusted JSON with strict decoding (rejects
unknown fields, trailing data, type mismatches = schema confusion; rejects
unknown kinds and out-of-range severity) and quarantines free text;
`AuthorizeContainment` reads only validated enum + trusted severity + tool
allowlist, never the note. Guards: fail-closed `ToolAllowlist`, `ReplayGuard`,
`SameTenant` isolation, `SanitizeForLog`, human approval for high impact.

**Battery (inert fixtures only):** `go test ./internal/taint/...` → 11 tests
cover forged roles, tool-output laundering, schema confusion, log injection,
malformed input, replay, cross-tenant contamination, privilege escalation via
tools, and taint propagation. All pass; `-race` clean.

**Verification of the core gate:** clean vs hostile-note observations produce
**identical** containment decisions — attacker text cannot change policy.
Complements `strategy.TestPromptInjectionCannotOverridePolicy` on the output
side.

**Limitations:** this is a library + battery; wiring the contract through every
live ingestion path is a tracked integration step (not yet every call site).

**Next action:** R6 — proxy correctness & resilience suite + SLOs + fault
injection.

---

## R6 — Proxy correctness & resilience — VERIFIED (2026-11-03)

**Files:** `internal/edge/resilience_test.go`, `docs/security/proxy-resilience.md`.

**What shipped:** a correctness + resilience + SLO suite against the real
`edge.Engine` and live httptest backends. Correctness: keep-alive reuse,
chunked/streaming, SSE passthrough, 8 MiB body byte-for-byte (both modes), gzip
never corrupted, Set-Cookie/Cache-Control/redirect passthrough, POST body
forwarding, observe/protect body parity (unsafe transforms off by default; the
only protect rewrite skips any Content-Encoding and fixes Content-Length).
Resilience/fault-injection: upstream outage → 502; alert-sink (telemetry) outage
→ traffic unaffected; client cancellation → no wedge; graceful shutdown drains
in-flight. Added-latency SLO measured against a realistic backend.

**Results:** `go test ./internal/edge/...` all pass; `-race` clean. Added
latency p50 ~47µs, p95 ~62µs, p99 ~169µs (suite asserts p99 < 50ms ceiling).

**Limitations (declared):** HTTP/2 and WebSocket upgrade are carried by the
stdlib proxy but not separately asserted (no TLS/h2 fixture yet); the alert sink
is synchronous (use a buffered sink in production); CPU/mem saturation not
measured (overhead test, not a load test); latency is single-machine.

**Next action:** R7 — supply-chain & release assurance (CI, SBOM, fuzz,
dependency policy).

---

## R7 — Supply-chain & release assurance — VERIFIED (2026-11-03)

**Files:** `internal/{taint,decoy,evidence,strategy}/fuzz_test.go`,
`tools/sbom/main.go`, `sbom.cdx.json` (generated), `.github/workflows/ci.yml`
(extended), `Makefile` (fuzz/sbom/vuln targets), `docs/security/SUPPLY_CHAIN.md`.

**What shipped:** native Go fuzz targets for the four untrusted-input surfaces
(parser, decoy routing, event ingestion, policy evaluation), each asserting
no-panic plus a domain invariant (e.g. policy never allows unbounded scope). A
self-contained deterministic CycloneDX SBOM generator (`make sbom`). CI extended
with a fuzz-smoke job (`make fuzz FUZZTIME=20s`), a supply-chain job
(govulncheck, `go mod tidy` gate, SBOM freshness diff + artifact upload), on Go
1.25.x and 1.27.x. Supply-chain doc covering pinned deps, update policy,
reproducible-build guidance, secrets/access/retention/minimization/safe-defaults,
and an explicit "NOT claimed" section (no certification, no signed releases yet).

**Commands / results:**
- `make fuzz` locally: millions of execs/target, **0 crashes** (FuzzPolicyEvaluate ~3.7M execs, invariants held)
- `go run ./tools/sbom` → deterministic CycloneDX (1 runtime dep: yaml.v3 v3.0.1)
- `gofmt -l .` clean (formatted all prior Phase 2 files), `go vet` clean, `go test -race ./...` pass
- `go mod tidy` → no change

**Limitations (declared):** signed releases / SLSA provenance not yet wired;
govulncheck is advisory coverage, not a guarantee; SECURITY.md still has
owner-draft fields.

**Next action:** R8 — scientific statistics & targets (reproduce scripts,
targets-vs-observed).

---

## R8 — Scientific statistics & targets — VERIFIED (2026-11-03)

**Files:** `internal/research/stats/stats.go` (+PrecisionAtPrevalence,
RequiredNForProportionCI, tests), `internal/research/scorecard/{scorecard,report,scorecard_test}.go`,
`cmd/heramba/research_stats.go`, `docs/research/statistics.md`,
`docs/research/artifacts/scorecard/`.

**What shipped:** a target-vs-observed scorecard that reruns the three
experiments and scores them against the directive's aspirational thresholds with
Wilson/bootstrap intervals, matched-comparison effect sizes, precision at a
realistic 1% prevalence, and sample-size adequacy. `heramba research stats`.

**Reference scorecard (bfc8f36):** recall 1.00 ✅ (≥0.90); **FPR 0.094 ❌ MISS**
(≤0.01) and flagged **underpowered** (n=32 ≪ ~381 needed for ±1%); objective
reduction 1.00 ✅ (≥0.50); response latency 4.0s ✅ (≤5s); valid-run rate 1.00 ✅.
Effect size: adaptive prevents 0.884 [0.856,0.912] more objectives than static.
Precision at 1% prevalence: 0.097 (shows why 9.4% FPR fails rare-event
detection).

**Commands / results:** `go test ./internal/research/{stats,scorecard}/...` ok
(incl. honest-MISS and positive-effect assertions); full suite PASS; vet clean.

**Honesty rules enforced:** a MISS is reported as a MISS; no zero-substitution;
no unstratified aggregate; observed explicitly separated from target.

**Next action:** R9 — operations, explainability & human control (investigation
timeline, incident reports).

---

## R9 — Operations, explainability & human control — VERIFIED (2026-11-03)

**Files:** `internal/incident/{incident,report,incident_test}.go`,
`internal/alert/{webhook,webhook_test}.go`,
`docs/research/operations-explainability.md`.

**What shipped:** an `incident.Builder` that links the full causal chain
(observation → fact → hypothesis → proposed → approved → outcome) from the
existing typed records, with observed-vs-inferred marking, confidence,
uncertainty, contradicting-evidence counts, source health, and causal caveats.
`Explain(actionID)` returns a single action's chain back to its evidence.
Human-control gates are checked and tested: `AllConsequentialReversible` (an
enforced action needs a rollback path) and `AllConsequentialHaveEvidence` (a
state-changing action must cite evidence); per-action blast radius (bounded
scope, legit-impact flag). Retention policy with `Prune`. Markdown/JSON incident
export. A **functional, tested webhook alert sink** (verified against httptest:
delivery, severity floor, error-on-500) — no fabricated integrations.

**Commands / results:** `go test ./internal/incident/... ./internal/alert/...`
ok (+race); full suite PASS; vet clean. Gates proven: enforced-without-token →
not reversible; no-evidence action → evidence gate fails.

**Limitations:** retention windows are a deployment choice; only
text/JSONL/webhook sinks exist (Slack/SIEM would each need the same functional
test first).

**Next action:** R10 — advanced training package (prompt-injection + adaptation
labs, course map).

---

## R10 — Advanced training package — VERIFIED (2026-11-03)

**Files:** `labs/07-prompt-injection-defense/{lab_test.go,README.md}`,
`labs/08-adaptation-race/{lab_test.go,README.md}`, `labs/README.md` (updated to
eight labs), `docs/training/{COURSE.md,README.md}`.

**What shipped:** two new locally-gradable labs reusing real Phase 2 packages —
Lab 07 (prompt-injection-resistant controls, `internal/taint`) and Lab 08
(adaptation race & ablation, `internal/research/coevolve`). The lab set now
covers all directive-required topics: campaign reconstruction (02),
coordination-vs-automation (06), stateful-deception consistency (03),
business-logic invariants (04), prompt-injection-resistant controls (07), and
adaptation race + ablation (08). Each lab has objectives, prerequisites, expected
duration, one-command run/reset, expected observations, automatic grading (the
test is the grader), troubleshooting, and cleanup. Added a 2-day core + optional
4-day course map in `docs/training/COURSE.md` with per-block hands-on time and
instructor dependencies, plus the public/private materials policy.

**Fresh-machine dry run:** `go test ./labs/...` → all 8 labs `ok`, with **no
instructor-only secrets and no external targets**. Full suite + vet + gofmt clean.

**Limitations:** instructor solution keys / challenge flags / grading secrets are
intentionally kept in a separate private repo (not committed here).

**Next action:** R11 — final research & release review (reports + capability
matrix).

---

## R11 — Final research & release review — VERIFIED (2026-11-03)

**Files:** `docs/research/{research-report,reproducibility,capability-matrix}.md`,
`docs/security/assurance-report.md`.

**What shipped:** the research report (threat model, protocol, H1–H5 results with
uncertainty, ablations, limitations, ethics/safety), the reproducibility guide
(every figure → a committed command, with determinism checks), the security &
release assurance report (proxy correctness, threat-model coverage, supply-chain,
fuzz, failure injection, known risks, release readiness), and a combined
capability matrix for the original 12 + Phase 2 with honest VERIFIED / PARTIAL /
NOT STARTED verdicts.

**Independent rerun (93557d5):** `go build`/`vet`/`gofmt` clean; `go test -race
./...` → **38 packages pass, no races**; coordination metrics hash stable
(`c24eb0fdd001c864`); `heramba experiment --json` byte-identical
(`1387ba652109a167`); fuzz smoke clean. Codebase grew 93→136 Go files,
10,647→16,460 LOC across 11 Phase 2 commits.

**Honest blockers carried forward:** coordination FPR (9.4%) exceeds the 1%
target on underpowered synthetic benign; H4 (missing-sensor degradation) is
PARTIAL; no signed releases/SLSA yet; SECURITY.md owner-draft fields unfilled;
deception app / strategy engine not auto-mounted into live serve.

## Phase 2 — complete

R0–R11 VERIFIED (H4 PARTIAL, disclosed). All critical tests pass under
`go test -race ./...`; every experiment reproduces from a committed command.
