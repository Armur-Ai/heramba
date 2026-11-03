# Deception & attack-objective experiment (R3)

- code revision: `fe40522`
- attacker: budget 60, deception-aware=false (reject threshold 0.60)
- matched: depth 8, 500 trials per arm

| arm | objective completion [95% CI] | time-to-objective (s) | diverted | detection lat (s) | deception discovery | benign disruption |
|---|---|---|---|---|---|---|
| unprotected | 1.000 [0.992, 1.000] | 16.0 [16.0, 16.0] | 0.000 [0.000, 0.000] | n/a | 0.000 [0.000, 0.000] | 0.000 |
| conventional | 1.000 [0.992, 1.000] | 16.0 [16.0, 16.0] | 0.000 [0.000, 0.000] | n/a | 0.000 [0.000, 0.000] | 0.051 |
| static_decoys | 0.884 [0.853, 0.909] | 74.1 [72.0, 76.1] | 0.775 [0.767, 0.783] | 3.2 [3.1, 3.4] | 0.000 [0.000, 0.000] | 0.000 |
| stateful_deception | 0.400 [0.358, 0.444] | 93.8 [91.3, 96.2] | 0.863 [0.859, 0.867] | 2.7 [2.6, 2.8] | 0.000 [0.000, 0.000] | 0.000 |
| adaptive_deception | 0.000 [0.000, 0.008] | n/a | 0.949 [0.947, 0.951] | 2.7 [2.6, 2.8] | 0.000 [0.000, 0.000] | 0.000 |

## Not measured

- cpu_time / memory_rss: not measured; this is a deterministic behavioral model, not a load test
- this arm model is a transparent abstraction of the defenses, not the live engine (see R6 for proxy overhead)
