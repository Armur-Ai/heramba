# Capability matrix — original 12 + Phase 2

Status is grounded in committed, runnable evidence. A capability is **VERIFIED**
only when its tests pass in-tree; **PARTIAL** when the core works but a declared
piece is incomplete; **NOT STARTED** / **BLOCKED** otherwise. Nothing is called
"production-ready" without evidence. Code revision: `93557d5`.

## Original 12 capabilities (foundation, CP0–CP10)

| # | Capability | Status | Evidence |
|---|---|---|---|
| 01 | Reverse-proxy edge runtime (observe/protect) | VERIFIED | `internal/edge` (+R6 resilience suite) |
| 02 | Decoys + cryptographic canaries (zero-FP) | VERIFIED | `internal/decoy` |
| 03 | Deterministic agentic-traffic classifier | VERIFIED | `internal/classify` |
| 04 | Signal denial (header normalize, timing jitter) | VERIFIED | `internal/signal` |
| 05 | Egress firewall + rate limiting | VERIFIED | `internal/egress`, `internal/ratelimit` |
| 06 | Normalized evidence model | VERIFIED | `internal/evidence` |
| 07 | Temporal campaign reconstruction | VERIFIED | `internal/campaign` |
| 08 | Swarm detection + active learning | VERIFIED | `internal/campaign/{swarm,experiment}` |
| 09 | Multi-sensor correlation | VERIFIED | `internal/evidence/sensor` |
| 10 | Strategy + bounded containment | VERIFIED | `internal/strategy` |
| 11 | Deception app + asset/bizlogic graphs | VERIFIED (offline) | `internal/{deception,assetgraph,bizlogic}`; online policy-optimization still PARTIAL |
| 12 | Replay evaluation + labs | VERIFIED | `internal/replay`, `labs/` |

## Phase 2 (R0–R11)

| R | Capability | Status | Evidence |
|---|---|---|---|
| R0 | Safe handoff & gap analysis | VERIFIED | [PHASE2_EXECUTION_STATUS.md](PHASE2_EXECUTION_STATUS.md) |
| R1 | Pre-registered protocol & threat model | VERIFIED | `go test ./internal/research/...`; `heramba research manifest` |
| R2 | Fair coordination experiment (H1) | VERIFIED | `heramba research coord`; PR-AUC 0.935 |
| R3 | Deception objective experiment (H2) | VERIFIED | `heramba research objective`; completion → 0.00 |
| R4 | Adaptation race & co-evolution (H3) | VERIFIED | `heramba research coevolve` |
| R5 | Prompt-injection & data-plane safety (H5a) | VERIFIED | `go test ./internal/taint/...`; 3.7M-exec policy fuzz |
| R6 | Proxy correctness & resilience (H5b) | VERIFIED | `go test ./internal/edge/...`; p99 ~169µs |
| R7 | Supply-chain & release assurance | VERIFIED | `make fuzz`/`make sbom`; CI govulncheck |
| R8 | Scientific statistics & targets | VERIFIED | `heramba research stats` scorecard |
| R9 | Operations, explainability & control | VERIFIED | `go test ./internal/incident/...` |
| R10 | Advanced training package | VERIFIED | `go test ./labs/...` (8 labs, fresh dry run) |
| R11 | Final research & release review | VERIFIED | this report set |

## Hypotheses

| H | Claim | Verdict | Evidence |
|---|---|---|---|
| H1 | coordination distinguishable | **null rejected** | R2: full PR-AUC 0.935 > all baselines |
| H2 | adaptive deception reduces objectives | **null rejected** | R3: completion 1.00 → 0.00 |
| H3 | defender adapts fast enough for positive utility | **null rejected (detectable)** | R4: utility 0.93, response 4.0s; feint case lossy |
| H4 | graceful degradation under missing sensor | **PARTIAL** | R2 ablations degrade gracefully; full live-sensor sweep not run |
| H5 | safety holds, overhead bounded | **null rejected** | R5 fuzz + R6 sub-ms latency |

## Explicitly NOT production-ready / not claimed

- Coordination **FPR 9.4%** (target 1%), on underpowered synthetic benign.
- **H4** missing-sensor degradation: PARTIAL.
- Synthetic deception app / strategy engine **not auto-mounted into live `serve`**
  (opt-in).
- **No signed releases / SLSA provenance** yet; **SECURITY.md** has owner-draft
  fields.
- All experiment numbers are **synthetic-harness measurements**, not field
  performance; no certification claim.
