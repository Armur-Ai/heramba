# Missing-sensor degradation (H4)

Tests **H4**: *detection degrades gracefully, not catastrophically, when a sensor
is missing.* A "missing sensor" is modeled as a removed detection signal; the
sweep measures recall (held-out coordinated) and FPR (negative pool) with each
signal removed one at a time.

```bash
heramba research h4
heramba research h4 --artifacts docs/research/artifacts/h4
```

Code: [`internal/research/coord/sensors.go`](../../internal/research/coord/sensors.go).
Reference: [`artifacts/h4/`](artifacts/h4/).

## Reference sweep

Full-detector recall 1.00; request-only floor (the null) 0.00.

| missing sensor | recall | degradation |
|---|---|---|
| timing (synchrony) | 0.00 | **load-bearing** |
| target-overlap (shared) | 1.00 | graceful (redundant) |
| graph (division of labor) | 0.00 | **load-bearing** |
| deception (decoy contact) | 1.00 | graceful (redundant) |

## Verdict: PARTIAL (quantified)

Honest result rather than a forced pass. For a **labor-dividing** swarm the
detector needs **both** synchrony **and** the division-of-labor signal — removing
either collapses recall to the floor, so the H4 null *holds* for those two
sensors. The target-overlap and decoy sensors are **redundant** for this swarm
type: removing them leaves recall at 1.00, so the null is *rejected* for them.

In other words, the detector is **not degradable to a single missing sensor for a
given swarm type** — it requires synchrony plus the applicable co-targeting
signal. Full graceful degradation across every sensor would need either signal
redundancy from a mixed swarm population (some swarms share targets, some divide)
or a timing-independent coordination path (strong division-of-labor + enumeration
without synchrony). Both are tracked; neither is claimed today.

This is why H4 remains **PARTIAL** in the [capability matrix](capability-matrix.md):
the degradation is now *measured per sensor*, not asserted, and two of four
sensors are load-bearing.
