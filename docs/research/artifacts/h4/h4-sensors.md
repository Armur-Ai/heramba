# H4 — missing-sensor degradation sweep

- code revision: `322b409`
- full-detector recall: 1.000 [0.839, 1.000]
- request-only floor (null): 0.000 [0.000, 0.161]

| missing sensor | recall [95% CI] | FPR [95% CI] | degradation |
|---|---|---|---|
| timing (synchrony) | 0.000 [0.000, 0.161] | 0.000 [0.000, 0.009] | **load-bearing (recall collapses)** |
| target-overlap (shared) | 1.000 [0.839, 1.000] | 0.007 [0.002, 0.021] | graceful |
| graph (division of labor) | 0.000 [0.000, 0.161] | 0.000 [0.000, 0.009] | **load-bearing (recall collapses)** |
| deception (decoy contact) | 1.000 [0.839, 1.000] | 0.007 [0.002, 0.021] | graceful |

**Verdict:** PARTIAL: most sensors degrade gracefully, but at least one is load-bearing (removing it collapses recall) — H4 null holds for that sensor.

- a 'missing sensor' is modeled as a removed detection signal; recall/FPR are on the held-out set
- the floor is the request-only recall — the null hypothesis is that any removal drops to this floor
