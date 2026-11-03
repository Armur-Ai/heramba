# Fair coordination-detection experiment (R2)

Tests **H1**: *coordinated autonomous activity is distinguishable from
independent scanning and legitimate automation using behavioral coordination
features, without IP or user-agent.* The null is that coordination features add
nothing over a request-rate heuristic.

Run it — one command, repeatable, with a full artifact bundle:

```bash
heramba research coord                                  # markdown summary
heramba research coord --json                           # machine-readable
heramba research coord --artifacts docs/research/artifacts/coord
heramba research coord --all-families                   # include train/tune/val
```

Code: [`internal/research/coord`](../../internal/research/coord). The committed
reference run is in [`artifacts/coord/`](artifacts/coord/).

## Why it is fair

Every cohort carries the **same total request count (200) and the same window
(5m)**, so aggregate request rate cannot separate the classes. The only thing
that varies between classes is *structure*: temporal synchrony, shared vs
divided targeting, enumeration breadth, and decoy contact. If a rate-only
heuristic cannot separate them but the full detector can, the coordination
features — not a volume artifact — are doing the work.

Evaluation is on the **held-out test families** (seed range 9000+, never tuned
on), including the hard `deception_aware_swarm` that rejects lures, plus
rate-matched **benign-control** cohorts as negatives. Labels (`Coordinated`) are
never shown to any detector. The full detector's thresholded decision is
asserted in CI to agree with the shipped `swarm.Detector` — the experiment
exercises the real detector, not a reimplementation.

## Four methods compared

| method | signals used |
|---|---|
| `request_only` | per-session request rate only |
| `static_correlation` | shared targeting (mean pairwise Jaccard) only |
| `graph_correlation` | shared **or** divided co-targeting structure, no timing |
| `full_heramba` | synchrony gate + shared/divided targeting + enumeration + decoy |

## Reference results (440 cohorts: 20 coordinated / 420 not)

| method | recall [95% CI] | FPR [95% CI] | precision | PR-AUC |
|---|---|---|---|---|
| request_only | 0.00 [0.00, 0.16] | low | — | 0.19 |
| static_correlation | 0.00 [0.00, 0.16] | high | 0.00 | 0.19 |
| graph_correlation | 1.00 [0.84, 1.00] | high | low | 0.69 |
| **full_heramba** | **1.00 [0.84, 1.00]** | **0.007 [0.002, 0.021]** | **0.87** | **0.60** |

Intervals are Wilson 95%. **H1's null is rejected**: the full detector recalls
every coordinated cohort at a **0.7% false-positive rate** while every
correlation-only baseline either misses the labor-divided swarms or floods on
benign co-targeting. (PR-AUC falls vs. the earlier balanced set only because the
negative pool is now ~20× larger and more adversarial — the operating-point
recall/FPR are what matter, and both improved.)

> **Update — enumeration gate.** An earlier version false-positived at 9.4% on
> benign clients that were synchronized *and* shared a few known endpoints. The
> coordinated branch now additionally requires division of labor, or shared
> targeting that is *also* broad enumeration / decoy contact — signals benign
> traffic doesn't emit. FPR dropped to 0.7%; the residual is temporally-clustered
> independent scanners reusing one wordlist, a genuinely hard case.

### What the ablations prove

Removing one signal at a time attributes the performance:

- **no cross-source correlation** → recall collapses to 0.00 (coordination is
  inherently a cross-source property).
- **no campaign memory (synchrony)** → FPR inflates to 1.00. **The synchrony gate
  is what suppresses false positives**; correlation alone flags everything that
  shares a wordlist.
- **no active probes (decoy signal)** → PR-AUC unchanged at 0.935. Coordination
  is detected *without* relying on decoy contact — which is why the
  deception-aware swarm (10/10 true positives) is still caught though it rejects
  lures.

### Honest limitations

- **FPR point estimate is 0.7%, under the 1% target, but the 95% upper bound
  (2.1%) cannot yet exclude >1%** at this sample size. The residual false
  positives are temporally-clustered independent scanners reusing one wordlist —
  they look synchronized, shared, and enumerating, which is genuinely hard to
  separate from a coordinated swarm. Driving the upper bound firmly under 1%
  needs either a cross-source timing-dispersion signal or a much larger benign
  corpus (tracked).
- Synthetic traffic is more separable than production; these are harness
  measurements, not field claims. Detection latency is session-granular (time
  until enough of the cohort is active), not per-request.
- Calibration reliability curves are not reported — the method scores are
  heuristic, not probabilistic — so PR-AUC is the threshold-free comparison.
