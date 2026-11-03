# Heramba — Security & release assurance report

What is actually verified in-tree, and the honest limits. **No certification,
accreditation, or "military-grade" claim is made** — only what committed code and
CI demonstrate. Code revision: `93557d5`.

## Summary

| Area | State | Evidence |
|---|---|---|
| Proxy correctness & resilience | VERIFIED | `go test ./internal/edge/...` (+race) |
| Defender prompt-injection resistance | VERIFIED | `go test ./internal/taint/...`; ~3.7M-exec policy fuzz |
| Supply-chain checks | VERIFIED | CI: govulncheck, gofmt, tidy gate, SBOM diff |
| Fuzzing (parsers/routing/ingestion/policy) | VERIFIED | `make fuzz`, 0 crashes |
| Failure injection | VERIFIED | upstream/sensor-outage, cancellation, shutdown tests |
| Operator control & auditability | VERIFIED | `go test ./internal/incident/...` |
| Signed releases / SLSA provenance | NOT STARTED | release-time work, not claimed |

## Proxy correctness & resilience (R6)

Against live backends: keep-alive, chunked/streaming, SSE, 8 MiB body byte-for-
byte (both modes), gzip never corrupted, cookie/cache/redirect passthrough, POST
body forwarding, observe/protect body parity. **No unsafe transforms by
default** — the only protect-mode rewrite skips any `Content-Encoding` and fixes
`Content-Length`. Fault injection with documented safe behavior: upstream outage
→ 502; telemetry/sensor outage (alert sink errors) → traffic unaffected; client
cancellation → no wedge; graceful shutdown drains in-flight. Added latency p50
~47µs / p99 ~169µs (regression ceiling p99 < 50ms). See
[proxy-resilience.md](proxy-resilience.md).

## Threat-model coverage (R5)

Attacker-controlled input (URLs, headers, logs, telemetry, decoy content, tool
outputs) is assumed hostile and handled by the taint layer: a value carries a
trust label and only `Trusted()`/`RequireTrusted()` reach the decision path, so
injection is a type error. A typed contract rejects schema confusion, unknown
kinds, and out-of-range values, and quarantines free text. Battery (inert
fixtures): forged roles, tool-output laundering, schema confusion, log injection,
malformed input, replay, cross-tenant contamination, privilege escalation via
tools, taint laundering — all covered. The core gate (attacker text cannot change
policy) is proven structurally and by fuzzing. See
[data-plane-safety.md](../research/data-plane-safety.md).

## Supply-chain checks (R7)

CI runs `govulncheck ./...` (dependency vulnerability scan), a `go mod tidy`
cleanliness gate, and an SBOM freshness diff; it uploads the CycloneDX SBOM as an
artifact. The module has **one** pinned runtime dependency (`gopkg.in/yaml.v3
v3.0.1`). Reproducible-build guidance (`-trimpath`, `SOURCE_DATE_EPOCH`,
`CGO_ENABLED=0`) and the dependency-update policy are in
[SUPPLY_CHAIN.md](SUPPLY_CHAIN.md).

## Fuzz results (R7)

Four targets — parser (`taint.DecodeObservation`), decoy routing, event ingestion
(`evidence` store loader), and policy evaluation — each asserting no-panic plus a
domain invariant. Local reference runs reached millions of executions per target
with **zero crashes**; the policy fuzzer (~3.7M execs) found no case where an
unbounded scope or disallowed type was allowed.

## Failure injection (R6)

Upstream dead → 502 (no hang/panic); alert sink erroring → served traffic
unaffected (the sink is the only hot-path downstream, and its failure is
swallowed); client cancellation → edge keeps serving; graceful shutdown drains
in-flight requests. **Documented hardening note:** the alert sink is synchronous,
so a *slow* (not failing) sink would add latency — production should use a
buffered/async sink.

## Operator control & auditability (R9)

Every consequential action is scoped (bounded to one subject), time-limited
(expiry/auto-rollback), reversible (rollback token; checked by
`AllConsequentialReversible`), and evidence-backed (checked by
`AllConsequentialHaveEvidence`). Shadow mode is the default; medium+ impact needs
human approval; an emergency kill-switch exists. The incident timeline links
observation→fact→hypothesis→action→outcome and exports a report. See
[operations-explainability.md](../research/operations-explainability.md).

## Known risks / open items

- **Coordination FPR (9.4%) exceeds the 1% target** on hard synthetic benign, and
  the benign sample is underpowered; not production-tuned.
- **H4 (missing-sensor degradation) is PARTIAL** — not a full live-sensor sweep.
- **No signed releases / SLSA provenance yet.**
- **SECURITY.md has owner-draft fields** (security contact, supported versions)
  that must be filled before publication.
- The synthetic deception app and strategy engine are **not auto-mounted into
  live `serve`** by default (opt-in).

## Release readiness

**Observe mode is safe to run in front of production** (it only watches; proxy
correctness and fail-safe behavior are tested). Inline protect-mode actions are
bounded, audited, reversible, and opt-in. The research/intelligence planes are
**research-grade and library-complete**, not a turnkey managed service. This is a
pre-release engine with honestly-disclosed limitations, not a certified product.
