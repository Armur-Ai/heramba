# Changelog

All notable changes to Heramba are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project aims to
follow [Semantic Versioning](https://semver.org/) once it tags a release.

## [Unreleased]

### Added — intelligence, adaptive-defense & research planes (capability build-out)
- **Evidence model** (`internal/evidence`): normalized, versioned
  `SecurityEvent` + `Observation`/`Fact`/`Hypothesis` with mandatory provenance,
  validation, confidence+uncertainty, decay/expiry, and append-only JSONL
  persistence with round-trip + unknown-schema soft-skip.
- **Campaign reconstruction** (`internal/campaign`): sessionization (observed)
  and probabilistic entity resolution into actors/campaigns (inferred, never
  equating an IP with an identity), with merge/split false-merge correction, a
  temporal graph, and explainable timelines.
- **Swarm-coordination detection** (`internal/campaign/swarm`): four-class
  detector (sequential / independent / coordinated / legit-automation) keyed on
  synchrony + shared/divided targeting + enumeration — not UA/rate alone — with
  a measured four-class benchmark.
- **Active learning** (`internal/campaign/experiment`): competing hypotheses,
  Bayesian updates, expected-information-gain ranking, hard experiment budgets.
- **Stateful deception app** (`internal/deception`): seeded coherent synthetic
  world (users/projects/resources/roles), per-session state, canary tokens,
  interaction telemetry, strict production isolation.
- **Defender-controlled attack graphs** (`internal/assetgraph`): separated
  real/synthetic graphs (real fails closed on adversary export), attack-path
  hypotheses, crown-jewel risk ranking.
- **Business-logic detection** (`internal/bizlogic`): OpenAPI/native spec
  ingest, object-authorization (BOLA/IDOR) + workflow-sequence detection,
  authorized-equivalent activity allowed.
- **Strategy & adaptive containment** (`internal/strategy`): closed set of typed
  bounded actions, deterministic policy-as-code (injection-proof), shadow
  default, approval/expiry/rollback, emergency disable, audit.
- **Multi-sensor correlation** (`internal/evidence/sensor`): app/auth/network
  collectors, idempotency-keyed dedup, timestamp reconciliation, sensor health,
  missing-data safety, backpressure, cross-sensor incident timeline.
- **Replay & four-baseline evaluation** (`internal/replay`): seeded
  deterministic scenarios, four comparable defenses, measured metrics with
  uncertainty, machine+human reports, and the `heramba experiment` command.
- **Intelligence bridge** (`internal/intel`): an `alert.Sink` that lifts live
  detections into the evidence store and reconstructs campaigns; wired into
  `heramba serve` with an intelligence rollup at shutdown.
- **Six conference labs** (`labs/`): locally-gradable exercises over the real
  engine; public/private materials policy documented.
- **Engineering docs**: `docs/architecture/{current-state,implementation-plan}.md`
  and `docs/engineering/EXECUTION_STATUS.md` (12-capability matrix + research
  summary). Full suite passes under `go test -race`.

### Added — the OSS engine's first working core
- **Edge runtime** (`internal/edge`): reverse-proxy / sidecar with **observe**
  (default, detection-only passthrough) and **protect** (inline) modes.
- **Deception surface** (`internal/decoy`): default decoy endpoints + a
  registry, and cryptographic **canary tokens** — zero-false-positive detection.
- **Canary detection in the edge**: watched canary tokens (config
  `decoys.canaries`) are detected in request path/query/headers **and a bounded prefix of the body**;
  protect mode denies the request.
- **Agentic-traffic classifier** (`internal/classify`): deterministic,
  explainable scoring (cadence, client fingerprint, missing browser signals,
  enumeration). No LLM on the detection path.
- **Signal denial** (`internal/signal`): response-header normalization + bounded
  timing jitter.
- **Egress firewall** (`internal/egress`): deny-by-default outbound allowlist
  (protect mode) that cuts SSRF/OOB callbacks and exfil.
- **Per-source rate limiting** (`internal/ratelimit`): token-bucket throttle in
  protect mode (429 + `rate_limited` event); never engaged in observe mode.
- **Counter-agent prompt decoys** (`internal/counter`): inject fabricated vuln
  leads into HTML/JS for classifier-flagged agentic sources (protect mode); the
  fake paths become decoys and the key a canary — closed-loop deception.
- **Alerting** (`internal/alert`): text + JSONL sinks, fan-out.
- **CLI** (`cmd/heramba`): `serve`, `decoys` (list / canary), `config`
  (init / show), `doctor`, `version`.

### Status
Pre-release. Observe mode is safe to run in front of production first. See the
README for the honest shipped-vs-planned breakdown; richer decoy *content*,
AI-augmented classification, and managed cloud intelligence are the commercial
tier, not in this engine.
