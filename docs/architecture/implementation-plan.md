# Heramba — Implementation Plan (12 capabilities, 4 planes)

Derived from the master engineering prompt. The plan builds **inside the
existing codebase as modular packages**, not microservices. Every capability
lands as integrated code + tests before it is marked VERIFIED in
`../engineering/EXECUTION_STATUS.md`.

## Planes → packages

```
data plane (shipped)        internal/{edge,classify,decoy,signal,egress,ratelimit,counter,alert,event,config}
intelligence plane (new)    internal/evidence   — normalized events, Observation/Fact/Hypothesis, provenance, store
                            internal/campaign   — temporal graph, entity resolution, correlation, memory, investigation API
                            internal/bizlogic    — OpenAPI ingest, resource graph, workflow/authz invariants
adaptive-defense plane (new) internal/strategy  — typed bounded actions, policy-as-code, shadow/approval/expiry/rollback/audit
                            internal/deception  — stateful fake app: synthetic users/resources, session state, canaries
                            internal/assetgraph — separated real vs adversary-visible synthetic graph
research plane (new)        internal/replay     — seeded scenarios, deterministic replay, four baselines, stats, reports
                            labs/               — six progressive isolated labs
```

## Dependency-ordered checkpoints

| CP | Title | New packages | Gate (acceptance) |
|---|---|---|---|
| 0 | Baseline | docs | build/test green, state documented |
| 1 | Evidence infrastructure | `evidence` | versioned events roundtrip; invalid rejected; persists; proxy regression green |
| 2 | Campaign intelligence | `campaign` | multi-session reconstruction; false-merge correctable; benign controls; perf measured |
| 3 | Stateful deception | `deception` | coherent workflow; no prod data; valid IDs; legit traffic unaffected |
| 4 | Attack graph + business logic | `assetgraph`, `bizlogic` | real/synthetic isolation; synthetic authz violation detected; authorized allowed |
| 5 | Strategy + containment | `strategy` | unauthorized action rejected; shadow non-enforcing; scope/expiry respected; rollback; injection can't override policy |
| 6 | Swarm correlation + active learning | `campaign/swarm`, `campaign/experiment` | independent vs coordinated evaluated; FP measured; budgets enforced |
| 7 | Multi-sensor intelligence | `evidence/sensor` | cross-source timeline; dedup; missing-sensor safe; backpressure |
| 8 | Optimization + replay | `replay` | seeded reruns; four baselines comparable; real measurements; overhead reported |
| 9 | Conference labs | `labs/` | labs run/reset/grade locally; no external target; instructor materials excluded |
| 10 | Hardening + final validation | — | regression/security/perf/CI; README matches reality |

## Cross-cutting invariants (enforced every checkpoint)

1. Deterministic enforcement; LLM proposals never directly execute.
2. Observe/shadow by default; high-impact actions need approval + expiry + rollback.
3. Real assets/credentials strictly separated from synthetic deception.
4. All attacker-origin bytes are hostile input; no text overrides policy.
5. No fabricated tests/benchmarks; unavailable metrics marked explicitly.
6. Every capability: unit + integration (+ security/perf where relevant) tests,
   failure handling, documented config, reproducible demo.

## Commit discipline

INSPECT → IMPLEMENT → TEST → FIX → VERIFY → DOCUMENT → COMMIT. Push is gated:
commit/push only when `go test ./...` is green. Frequent small commits.
