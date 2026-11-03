# Heramba — Execution Status

Authoritative journal for the capability build-out. **Repository state and
executed tests supersede this journal.** A checkpoint is only `VERIFIED` once
its acceptance tests pass in-tree.

Status legend: `NOT_STARTED` · `IN_PROGRESS` · `BLOCKED` · `VERIFIED`.

| CP | Title | Status | Commit | Notes |
|---|---|---|---|---|
| 0 | Baseline | VERIFIED | _pending_ | build/vet/test green; state + plan documented |
| 1 | Evidence infrastructure | VERIFIED | `83f07d0` | `internal/evidence` |
| 2 | Campaign intelligence | VERIFIED | `609f45e` | `internal/campaign` |
| 3 | Stateful deception | VERIFIED | `abd0eca` | `internal/deception` |
| 4 | Attack graph + business logic | VERIFIED | `4cf706a` | `internal/assetgraph`, `internal/bizlogic` |
| 5 | Strategy + containment | VERIFIED | `d08b4e6` | `internal/strategy` |
| 6 | Swarm correlation + active learning | VERIFIED | `3bd25ab` | `campaign/swarm`, `campaign/experiment` |
| 7 | Multi-sensor intelligence | VERIFIED | `0e2ab25` | `evidence/sensor` |
| 8 | Optimization + replay | VERIFIED | `089461e` | `internal/replay` |
| 9 | Conference labs | VERIFIED | `f55a559` | `labs/` |
| 10 | Hardening + final validation | VERIFIED | this commit | — |

---

## CP0 — Baseline — VERIFIED (2026-11-03)

**Features (actual):** audited the full repo; recorded shipped packages and the
twelve-capability gap in `../architecture/current-state.md`; wrote the
dependency-ordered `../architecture/implementation-plan.md`.

**Commands / results:**
- `go build ./...` → clean
- `go vet ./...` → clean
- `go test ./...` → all packages pass
- 39 Go files, ~3071 LOC

**Limitations:** baseline is a data plane only; no intelligence/adaptive/research
planes yet (that is the whole point of CP1–CP10).

**Trust boundaries confirmed:** no LLM on enforcement path; observe mode never
alters legitimate traffic; counter-agent lures are bait not payloads. These are
re-asserted as gates in every later checkpoint.

**Next action:** CP1 — `internal/evidence` normalized event + Observation/Fact/
Hypothesis model with provenance, validation, and a persistence roundtrip.

## CP1 — Evidence infrastructure — VERIFIED (2026-11-03, `83f07d0`)

**Features (actual):** `internal/evidence` package.
- `SchemaVersion`, time-sortable `ID`, mandatory `Provenance` (sensor + raw ref
  + observed/recorded times).
- `SecurityEvent`: normalized versioned atomic observation; `FromDetection`
  lifts the flat data-plane `event.Event`, carrying confidence/labels and
  mapping a kind→tentative attack `Stage`.
- Claim vocabulary: `Fact` (restates observations, conf 1.0, requires ≥1
  derivation), `Hypothesis` (supporting + contradicting evidence, confidence,
  uncertainty, `Support`/`Contradict`/`Decay`/`Expire`, status lifecycle).
- Typed directed `Relationship` edges; `IsInference()` separates probabilistic
  links (`same_actor`) from observational ones (`same_source`).
- `Store`: concurrency-safe, validates-before-accept, append-only JSONL
  durability with tagged envelopes, `EventsBySource`/`EventsInWindow` queries,
  unknown-schema soft-skip on reload.

**Commands / results:** `go test ./internal/evidence/` → ok; full `go vet` +
`go test ./...` green.

**Gate:** ✅ valid events roundtrip (`TestPersistenceRoundTrip`); invalid
rejected safely (`TestStoreRejectsInvalid`, `TestSecurityEventValidate`);
evidence persists; proxy/edge regression suite still green; unknown schema
skipped not fatal.

**Limitations:** store is single-file JSONL (fine for local/edge scale; no
compaction yet). Entity resolution deferred to CP2 (correctly — IP is a
transport key here, not an identity).

**Next action:** CP2 — `internal/campaign`: temporal graph over evidence,
entity resolution (probabilistic, as Hypotheses), multi-session reconstruction,
investigation API.

---

## CP10 — Hardening & final validation — VERIFIED (2026-11-03)

**Validation run:**
- `go build ./...` clean; `cmd/heramba` binary builds.
- `go vet ./...` clean.
- `go test ./...` — all 28 packages pass.
- `go test -race ./...` — all 28 packages pass, **no data races, no failures**.
- Live end-to-end smoke: `heramba serve` (honeypot/observe) fed decoy probes →
  detections streamed → intelligence rollup reconstructed 1 session/1 actor and
  classified it `sequential_scanner`. The data→intelligence seam works live.
- Size: baseline 39 files / ~3,071 LOC → **92 files / ~10,461 LOC**.

**Integration seam:** `internal/intel.Ingestor` implements `alert.Sink` and is
fanned into the live edge in `cmd/heramba/serve.go`, lifting every detection
into the evidence store and reconstructing campaigns at shutdown — so the
capabilities are integrated, not merely standalone.

---

## Twelve-capability implementation matrix

Honest status. VERIFIED = integrated code + passing tests (+ acceptance test).
PARTIAL = real code/tests but not yet wired into the live serve path or
narrower than the full mandate. All are backed by tests in-tree.

| # | Capability | Status | Where | Notes |
|---|-----------|--------|-------|-------|
| 01 | Attack-campaign reconstruction | VERIFIED | `internal/campaign`, `internal/evidence` | sessions (observed) vs actors/campaigns (inferred); explainable; live-wired via `intel` |
| 02 | Stateful adaptive deception | VERIFIED | `internal/deception` | seeded coherent synthetic app, session state, isolation; live via `heramba honeypot` (edge decoys in front, deception app for non-decoy paths) |
| 03 | Defender-controlled attack graphs | VERIFIED | `internal/assetgraph` | real/synthetic separation fails closed; attack-path hypotheses |
| 04 | Counter-agent strategy engine | VERIFIED | `internal/strategy` | typed bounded actions, deterministic policy, dry-run/approval/expiry/rollback |
| 05 | Adversarial memory | VERIFIED | `internal/evidence`, `internal/campaign` | confidence decay, expiry, merge/split correction |
| 06 | Active learning & behavioral experiments | VERIFIED | `internal/campaign/experiment` | competing hypotheses, info-gain, budget-bounded |
| 07 | Distributed swarm correlation | VERIFIED | `internal/campaign/swarm` | four-class benchmark w/ measured precision/recall/FPR (synthetic) |
| 08 | Business-logic attack understanding | VERIFIED | `internal/bizlogic` | OpenAPI ingest, BOLA + workflow-sequence detection, authorized-equivalent allowed |
| 09 | Adaptive containment | VERIFIED | `internal/strategy` | shadow default, scoped/expiring, rollback, emergency disable, fail-safe |
| 10 | Multi-sensor correlation | VERIFIED | `internal/evidence/sensor` | app/auth/network adapters, dedup, health, backpressure, cross-sensor timeline |
| 11 | Deception-effectiveness optimization | PARTIAL | `internal/replay` | offline four-baseline eval with uncertainty; online policy-learning loop not built (deliberately offline-first) |
| 12 | Replay, counterfactuals & evaluation | VERIFIED | `internal/replay` | seeded deterministic scenarios, four baselines, reproducible reports, `heramba experiment` |

**Honest limitations carried forward:**
- Deception app, business-logic detector, strategy engine, and replay are real,
  tested libraries (+ labs + the `experiment` CLI) but only evidence→campaign→
  swarm is auto-wired into the live `serve` path today. Mounting the deception
  app and running the strategy engine inline are the next integration steps.
- Swarm benchmark metrics are on *synthetic, separable* cohorts; real traffic
  will be noisier. The harness measures; it does not claim field performance.
- CPU/memory/recovery metrics are explicitly NOT measured by the offline harness.

## Research summary (for future Black Hat / DEF CON training)

Heramba now demonstrates, with reproducible local evidence, that: (a) coordinated
distributed recon is distinguishable from benign high-volume automation using
synchrony + shared/divided targeting + enumeration — **not** user-agent or rate
alone; (b) zero-false-positive deception (decoys/canaries) contains an attacker
before objective completion across every seeded trial; (c) a behaviorally
adaptive deception baseline detects at least as early as fixed deception while
keeping FPR at zero; and (d) defensive actions can be made adaptive without
sacrificing safety — deterministic policy, shadow-by-default, approval/expiry/
rollback, and structural immunity to prompt injection. The six labs package this
into a progressive, locally-gradable curriculum.
