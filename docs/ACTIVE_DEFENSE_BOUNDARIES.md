# Active-Defense Boundaries

> **STATUS: DRAFT product policy (2026-10-21).** This is a product-design boundary,
> **not** a declaration that any particular use is lawful. Authority and effects
> matter more than the label "defense". Not legal advice. ⚖️ Counsel + engineering
> must tie these to actually-implemented functions before any production use.

## Principle

The engine acts **only on infrastructure the operator is authorized to control**,
with bounded, reversible effects. An "attacker" classification, a high confidence
score, an opt-in toggle, or a disclaimer **cannot** supply missing authorization.
A suspected source may be a proxy, a compromised host, an authorized tester, or an
innocent user.

## BASELINE (allowed in-product, subject to documented authority + privacy/availability review)

- **Decoys/honeypots** and fabricated application data served by the operator's
  own authorized endpoints. No real stolen data, no executable booby-traps, no
  third-party impersonation, no credential reuse.
- **Response normalization** and suppression of unnecessary stack/version signals
  on operator-controlled responses (with compatibility tests).
- **Bounded throttling / latency** on operator-controlled request handling, with
  per-session and global resource limits, a maximum duration, and rollback. The
  objective is never to bankrupt a model caller or exhaust a remote service.
- **Blocking/sinkholing egress** initiated by the operator's own workloads at an
  authorized control point. Never alter another network's routes/DNS, impersonate
  a destination you lack authority over, or interfere with third-party
  infrastructure. Prefer explicit denials where silent drops harm legitimate ops.

## ADDITIONAL REVIEW BEFORE PRODUCTION

- Passive decoy URL/token → authorized collector may record a resulting ordinary
  inbound request. Minimize metadata; never append local files/secrets; never
  execute received code; never redirect to an unapproved destination; do not
  infer personal identity from a callback. **The telemetry is real even if the
  bait is fictional.**
- Deliberately misleading decoy content must be confined to defined defensive
  surfaces and reviewed for effects on legitimate users.

## LAB-ONLY — EXCLUDED FROM PRODUCTION

- Content/instructions designed to make an **external** agent execute tools,
  disclose prompts/keys/files, modify its memory/state, attack another host,
  propagate, or incur resource exhaustion.
- Callback mechanisms that induce exfiltration or exploit external client software.
- Automatic retaliation, external scanning/exploitation, or any action based
  solely on model-generated attribution.

## Technical requirements for the engine

- **Deterministic policy enforcement outside the model** — LLM output cannot change
  allowed destinations, privacy settings, budgets, or action privileges.
- Tenant-specific authorization manifests; approved control points/collectors;
  bounded allowlisted actions; **default observe mode**; explicit activation and
  revocation.
- Isolate decoys from production data/credentials; prevent honeypots becoming open
  relays or pivots; reject arbitrary caller-supplied upstream URLs; defend against
  SSRF/redirect/DNS-boundary errors.
- Classifier failure → a documented bounded fallback (fail-open vs fail-closed
  chosen per service **with the operator**), never an invented aggressive response.
- Versioned policy changes, short-lived privileges, emergency stop, auditable
  action reason/config, and tests proving rollback and maximum effects. **Do not
  claim these are implemented until verified.**

## Interception note
TLS termination / request capture can implicate wiretap and privacy laws (US
18 USC 2511; UK interception; EU communications/employee-monitoring). Provider/
party exceptions are conditional — not blanket authority to capture or reuse all
traffic. ⚖️
