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
| 7 | Multi-sensor intelligence | VERIFIED | pending | `evidence/sensor` |
| 8 | Optimization + replay | NOT_STARTED | — | `internal/replay` |
| 9 | Conference labs | NOT_STARTED | — | `labs/` |
| 10 | Hardening + final validation | NOT_STARTED | — | — |

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
