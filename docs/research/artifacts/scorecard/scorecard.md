# Targets vs. observed scorecard (R8)

- code revision: `bfc8f36`
- targets are **aspirational evaluation thresholds, not claims**

| metric | target | observed [95% CI] | met? | context |
|---|---|---|---|---|
| recall (held-out coordinated) | >= 0.90 | 1.000 [0.839, 1.000] (n=20) | ✅ met | full_heramba on held-out test families |
| false-positive rate | <= 0.01 | 0.094 [0.032, 0.242] (n=32) | ❌ MISS | benign controls + negative families; UNDERPOWERED for a 1% target (n=32 < ~381 needed for a ±1% interval) |
| objective-completion reduction (adaptive vs unprotected) | >= 0.50 | 1.000 [1.000, 1.000] (n=500) | ✅ met | unprotected 1.000 -> adaptive 0.000 |
| policy-safe response latency (response component, worst archetype) | <= 5s | 4.000 [4.000, 4.000] (n=800) | ✅ met | decision->applied->effective; detection latency reported separately |
| valid-artifact run rate | >= 0.95 | 1.000 [1.000, 1.000] (n=3) | ✅ met | coord + objective + coevolve each emitted a complete report |

## Effect sizes (matched comparisons, bootstrap 95% CI)

- **objective-completion: static_decoys - adaptive_deception**: 0.884 [0.856, 0.912] (n=1000) — positive = adaptive prevents more objectives (matched seeds)

## Notes

- precision at realistic 1% prevalence (recall 1.000, FPR 0.094): 0.097 — pooled synthetic precision overstates field precision
- targets are aspirational evaluation thresholds, not claims; a MISS is reported as-is
- synthetic traffic is more separable than production; observed numbers are harness measurements
