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

## Reference results (revision `be6db33`, 52 cohorts: 20 coordinated / 32 not)

| method | recall [95% CI] | FPR [95% CI] | precision | PR-AUC |
|---|---|---|---|---|
| request_only | 0.00 [0.00, 0.16] | 0.03 [0.01, 0.16] | — | 0.192 |
| static_correlation | 0.00 [0.00, 0.16] | 1.00 [0.89, 1.00] | 0.00 | 0.192 |
| graph_correlation | 1.00 [0.84, 1.00] | 1.00 [0.89, 1.00] | 0.39 | 0.692 |
| **full_heramba** | **1.00 [0.84, 1.00]** | **0.09 [0.03, 0.24]** | **0.87** | **0.935** |

Intervals are Wilson 95%. **H1's null is rejected**: the full detector's PR-AUC
(0.935) strictly exceeds every weaker method, at recall 1.00 and FPR 0.09.

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

- **FPR is 9.4%, above the 1% aspirational target** (R8). The false positives are
  benign-control cohorts where a few legitimate clients happen to hit the same
  endpoints within a tight window — they look synchronized and shared. This is a
  real property of the shipped detector's coordinated branch (synchrony + shared
  targeting), reproduced faithfully here, not hidden. An enumeration-breadth gate
  on the coordinated branch is the natural mitigation (tracked, not yet applied,
  to avoid changing CP6 behavior mid-study).
- Synthetic traffic is more separable than production; these are harness
  measurements, not field claims. Detection latency is session-granular (time
  until enough of the cohort is active), not per-request.
- Calibration reliability curves are not reported — the method scores are
  heuristic, not probabilistic — so PR-AUC is the threshold-free comparison.
