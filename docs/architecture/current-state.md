# Heramba — Current State (Checkpoint 0 baseline)

_Audited 2026-11-03 against commit `3984e8a`. Go 1.27.2, module
`github.com/Armur-Ai/heramba`._

This document records what the repository **actually shipped** at the start of
the capability build-out described in `implementation-plan.md`. It is the
honest baseline the execution journal (`../engineering/EXECUTION_STATUS.md`)
measures progress against.

## Build & test baseline

| Check | Result |
|---|---|
| `go build ./...` | ✅ clean |
| `go vet ./...` | ✅ clean |
| `go test ./...` | ✅ all packages pass |
| Go files / LOC | 39 files / ~3071 LOC |

No pre-existing test failures. No pre-existing vet findings.

## Shipped packages (data plane)

| Package | Responsibility | Status |
|---|---|---|
| `internal/event` | Flat detection record (`Event`): kind, severity, source, target, confidence, labels. Deception constructor pins confidence 1.0. | shipped |
| `internal/config` | `.heramba.yml` load/validate/init; observe vs protect; decoys/egress/counter/classifier/rate-limit sub-configs. | shipped |
| `internal/classify` | Deterministic agentic-traffic classifier: tool-UA, missing-browser-signal, non-browser Accept, cadence, enumeration. Explainable `Verdict` with per-signal contributions. No LLM. | shipped |
| `internal/decoy` | Decoy path registry + canary token mint/registry; zero-FP hit detection. | shipped |
| `internal/signal` | Signal denial: response-header normalization + timing jitter (protect). | shipped |
| `internal/edge` | Reverse-proxy edge runtime; per-request pipeline classify→deception→signal-denial→proxy; counter-agent lure injection on flagged agentic sources. | shipped |
| `internal/egress` | Deny-by-default outbound/callback firewall (protect). | shipped |
| `internal/ratelimit` | Per-source request-rate budget. | shipped |
| `internal/counter` | Counter-agent prompt-decoy generator (fabricated leads + keyed canary). | shipped |
| `internal/alert` | Alert sinks (text / JSONL) + counting/shutdown summary. | shipped |
| `internal/version` | Build version string. | shipped |
| `cmd/heramba` | CLI: `serve` / `decoys` / `config` / `doctor` / `version`. | shipped |

## What the baseline does NOT yet have

Measured against the master engineering prompt's twelve capabilities, the
baseline is a strong **data plane** but has no **intelligence plane**,
**adaptive-defense plane**, or **research plane**. Specifically absent:

- No normalized evidence model beyond the flat `event.Event` (no
  Observation/Fact/Hypothesis, no provenance chain, no persistence/roundtrip
  contract).
- No temporal campaign graph; no entity resolution; IP is currently the only
  source key (the prompt explicitly forbids equating IP with attacker identity
  as a *conclusion* — baseline uses it only as a transient classifier key).
- No stateful deception (decoys are static paths/tokens, not a coherent fake
  application with session-scoped state).
- No separated real/synthetic asset graph; no business-logic / OpenAPI
  understanding.
- No strategy engine, policy-as-code, containment with approval/expiry/rollback.
- No swarm-coordination correlation; no adversarial memory with decay.
- No multi-sensor correlation; no replay/evaluation harness; no labs.

## Trust-boundary posture (baseline)

- No LLM on the enforcement path — attacker text cannot prompt-inject a
  decision. This property must be preserved.
- Observe mode never alters legitimate traffic (default, safe in front of prod).
- Counter-agent lures are bait, never payloads aimed at an agent's host.

These invariants are load-bearing and are re-asserted as acceptance gates in
every subsequent checkpoint.
