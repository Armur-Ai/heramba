# Fair coordination-detection experiment (R2)

- code revision: `f2769d6`
- cohorts: 440 (20 coordinated / 420 not)
- families: [coord-test deception-aware-test indep-test legit-test benign-control]
- matched: 200 requests, 5m0s window per cohort (aggregate rate is not discriminative)

## Methods compared (operating threshold 0.50)

| method | recall [95% CI] | FPR [95% CI] | precision | PR-AUC |
|---|---|---|---|---|
| request_only | 0.000 [0.000, 0.161] | 0.290 [0.249, 0.336] | 0.000 [0.000, 0.031] | 0.023 |
| static_correlation | 0.000 [0.000, 0.161] | 1.000 [0.991, 1.000] | 0.000 [0.000, 0.009] | 0.023 |
| graph_correlation | 1.000 [0.839, 1.000] | 1.000 [0.991, 1.000] | 0.045 [0.030, 0.069] | 0.523 |
| full_heramba | 1.000 [0.839, 1.000] | 0.007 [0.002, 0.021] | 0.870 [0.679, 0.955] | 0.602 |

## Ablations (full detector, one signal removed)

| ablation | recall [95% CI] | FPR [95% CI] | PR-AUC |
|---|---|---|---|
| ablate_no_campaign_memory | 1.000 [0.839, 1.000] | 1.000 [0.991, 1.000] | 0.511 |
| ablate_no_cross_source | 0.000 [0.000, 0.161] | 1.000 [0.991, 1.000] | 0.023 |
| ablate_no_graph_motifs | 1.000 [0.839, 1.000] | 0.336 [0.292, 0.382] | 0.064 |
| ablate_no_active_probes | 1.000 [0.839, 1.000] | 0.336 [0.292, 0.382] | 0.562 |

## Per-family confusion (full_heramba, stratified)

| family | kind | coordinated | TP | FP | TN | FN |
|---|---|---|---|---|---|---|
| benign-control | benign | false | 0 | 3 | 397 | 0 |
| coord-test | coordinated | true | 10 | 0 | 0 | 0 |
| deception-aware-test | deception_aware | true | 10 | 0 | 0 | 0 |
| indep-test | independent | false | 0 | 0 | 10 | 0 |
| legit-test | legit_automation | false | 0 | 0 | 10 | 0 |

## Secondary endpoints

- detection latency (full_heramba, coordinated, session-granular): 14.4s [9.0, 20.1] over n=20
- throughput: 376246 classifications/sec (timing measurement, non-deterministic)

## Not measured

- cpu_time / memory_rss: not measured by this deterministic harness
- calibration reliability curve: scores are heuristic, not probabilistic; PR-AUC is reported instead
