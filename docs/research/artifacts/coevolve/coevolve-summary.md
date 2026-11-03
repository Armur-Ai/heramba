# Adaptation race & co-evolution lab (R4)

- code revision: `679597c`
- 8 rounds/race, 100 seeds; strictly local, seeded, event-sourced

## Adaptive defender (adaptation latency = detection + response)

| archetype | countered | adaptation latency (s) | detection | response | mean utility | util σ | oscillation |
|---|---|---|---|---|---|---|---|
| scripted | 1.000 [0.995, 1.000] | 9.0 [9.0, 9.1] | 5.0 [5.0, 5.1] | 4.0 [4.0, 4.0] | 0.930 [0.930, 0.930] | 0.000 | 0.000 [0.000, 0.005] |
| independent | 1.000 [0.995, 1.000] | 10.0 [10.0, 10.1] | 6.0 [6.0, 6.1] | 4.0 [4.0, 4.0] | 0.930 [0.930, 0.930] | 0.000 | 0.000 [0.000, 0.005] |
| coordinated | 1.000 [0.995, 1.000] | 11.0 [11.0, 11.1] | 7.0 [7.0, 7.1] | 4.0 [4.0, 4.0] | 0.930 [0.930, 0.930] | 0.000 | 0.000 [0.000, 0.005] |
| deception_aware | 1.000 [0.995, 1.000] | 14.0 [14.0, 14.1] | 10.0 [10.0, 10.1] | 4.0 [4.0, 4.0] | 0.930 [0.930, 0.930] | 0.000 | 0.000 [0.000, 0.005] |
| adversarial_varied | 0.669 [0.635, 0.700] | 16.0 [16.0, 16.1] | 12.0 [12.0, 12.1] | 4.0 [4.0, 4.0] | 0.448 [0.414, 0.483] | 0.496 | 0.501 [0.467, 0.536] |

## Ablation: utility by defender mode (mean [95% CI])

| archetype | no_adaptation | adaptive | instant |
|---|---|---|---|
| scripted | 0.000 [0.000, 0.000] | 0.930 [0.930, 0.930] | 0.930 [0.930, 0.930] |
| independent | 0.000 [0.000, 0.000] | 0.930 [0.930, 0.930] | 0.930 [0.930, 0.930] |
| coordinated | 0.000 [0.000, 0.000] | 0.930 [0.930, 0.930] | 0.930 [0.930, 0.930] |
| deception_aware | 0.000 [0.000, 0.000] | 0.930 [0.930, 0.930] | 0.930 [0.930, 0.930] |
| adversarial_varied | 0.000 [0.000, 0.000] | 0.448 [0.414, 0.483] | 0.930 [0.930, 0.930] |

## Not measured

- wall-clock CPU: ticks are logical; TickSeconds maps them for reporting only
- a non-seeded deployment adds observational nondeterminism; here jitter is seeded and bounded
