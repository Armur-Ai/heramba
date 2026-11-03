# Defender prompt-injection & data-plane safety (R5)

Tests **H5** (the safety half): *attacker-controlled text can never change a
policy decision or gain an execution privilege.* The premise is adversarial —
every URL, header, log, telemetry field, retrieved page, decoy content, and tool
output is assumed hostile.

Code: [`internal/taint`](../../internal/taint). Battery:
`go test ./internal/taint/...` (11 adversarial tests, inert fixtures only — no
usable payloads).

## The mechanism: taint as a type, not a judgment call

Every value carries a trust **label** (`Trusted` / `AttackerControlled` /
`Derived`) and **provenance**. A decision may read a value only through
`Trusted()` / `RequireTrusted()`, which return nothing for untrusted content.
There is **no trusted accessor that hands out attacker-controlled data** — the
only way to reach it is `Expose()`, named loudly and reserved for sanitized
logging. So forgetting to check is not possible; injection becomes a type error.

Taint **propagates through derivation**: `DeriveString` of an attacker value is
`Derived` (untrusted). A transformation cannot launder taint away.

## Typed observation/action contract

External input is decoded (`DecodeObservation`) into a struct with a **closed
enum** of kinds and quarantined free text. Decoding:

- rejects **unknown fields**, **trailing data**, and **type mismatches** (schema
  confusion) via strict JSON decoding — it never coerces;
- rejects **unknown kinds** and **out-of-range** severity;
- quarantines the free-text `note` and the attacker-chosen `source_key` as
  `AttackerControlled`.

`AuthorizeContainment` reads **only** the validated enum and the trusted numeric
severity plus the tool allowlist — it never consults the note or source-key
content. An injected instruction in `note` therefore **cannot change the
decision** (proven: clean vs. hostile-note observations produce identical
decisions).

## The rest of the battery

| Threat | Guard | Test |
|---|---|---|
| Forged role / tool-output laundering | decision ignores free text | `TestForgedRoleCannotChangeDecision` |
| Schema confusion | strict decode, no unknown fields/trailing data | `TestSchemaConfusionRejected` |
| Log injection (CR/LF, ANSI, control chars) | `SanitizeForLog` | `TestLogSanitization` |
| Malformed / empty input | fail to typed error, never partial object | `TestMalformedInputRejected` |
| Replay | `ReplayGuard` nonce dedup | `TestReplayGuard` |
| Cross-tenant contamination | `SameTenant` (fail closed on empty) | `TestCrossTenantIsolation` |
| Privilege escalation via tools | explicit fail-closed `ToolAllowlist` | `TestToolAllowlistFailsClosed` |
| High-impact auto-action | human approval required | `TestHighImpactRequiresApproval` |
| Taint laundering | propagation through `DeriveString` | `TestTaintPropagatesThroughDerivation` |

## Relationship to the strategy engine

This layer generalizes the guarantee already structural in
[`internal/strategy`](../strategy.md): `Policy.Evaluate` reads only an action's
typed fields and never free text, so a compromised LLM proposal takes the same
gate as any other (`strategy.TestPromptInjectionCannotOverridePolicy`). R5 makes
the **input** side symmetric: by the time anything reaches the policy, its
attacker-controlled parts are labeled and structurally excluded from the trusted
path.

## Boundaries

- No offensive payloads live in the repo; fixtures model the *shape* of an
  attack (an injected instruction string, a forged-role object) without being
  usable.
- This is a library + battery. Wiring the taint contract through every live
  ingestion path is a tracked integration step (R9/future), not a claim that
  every call site already uses it today.
