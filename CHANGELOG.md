# Changelog

All notable changes to Heramba are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project aims to
follow [Semantic Versioning](https://semver.org/) once it tags a release.

## [Unreleased]

### Added — the OSS engine's first working core
- **Edge runtime** (`internal/edge`): reverse-proxy / sidecar with **observe**
  (default, detection-only passthrough) and **protect** (inline) modes.
- **Deception surface** (`internal/decoy`): default decoy endpoints + a
  registry, and cryptographic **canary tokens** — zero-false-positive detection.
- **Canary detection in the edge**: watched canary tokens (config
  `decoys.canaries`) are detected in request path/query/headers; protect mode
  denies the request.
- **Agentic-traffic classifier** (`internal/classify`): deterministic,
  explainable scoring (cadence, client fingerprint, missing browser signals,
  enumeration). No LLM on the detection path.
- **Signal denial** (`internal/signal`): response-header normalization + bounded
  timing jitter.
- **Egress firewall** (`internal/egress`): deny-by-default outbound allowlist
  (protect mode) that cuts SSRF/OOB callbacks and exfil.
- **Alerting** (`internal/alert`): text + JSONL sinks, fan-out.
- **CLI** (`cmd/heramba`): `serve`, `decoys` (list / canary), `config`
  (init / show), `doctor`, `version`.

### Status
Pre-release. Observe mode is safe to run in front of production first. See the
README for the honest shipped-vs-planned breakdown; richer decoy *content*,
AI-augmented classification, and managed cloud intelligence are the commercial
tier, not in this engine.
