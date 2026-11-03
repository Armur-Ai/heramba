# Heramba Phase 2 — Research report

**Status: research results on synthetic workloads.** Every number here is a
harness measurement produced by committed, runnable code — not a field-performance
claim. Targets are aspirational thresholds; a miss is reported as a miss.

- Code revision at writing: `93557d5` (Phase 2 baseline: `bc61202`).
- Reproduce everything from [reproducibility.md](reproducibility.md).

## Threat model

Six bounded actor families (scripted, independent, coordinated,
legit-automation, human, and a held-out **deception-aware** swarm), each with
explicit capabilities, request/time budgets, objective, and defender
observability — encoded in code at `research.DefaultProtocol()` and documented in
[threat-model.md](threat-model.md). Boundaries: strictly local, synthetic,
authorized; coordination detection ≠ LLM attribution; no LLM on the enforcement
path.

## Pre-registered protocol

Five hypotheses (H1–H5), each with an explicit null, primary endpoint, and
leakage-free train/tune/validation/test partitions, validated in code
(`Protocol.Validate` rejects duplicate IDs, non-falsifiable hypotheses, unknown
endpoints, and **train/test seed leakage**). See [protocol.md](protocol.md).

## Results & uncertainty

All intervals are 95% (Wilson for proportions, bootstrap for means/effects).

### H1 — coordination is distinguishable ([R2](coordination-experiment.md))

On held-out families with matched request budgets, the full detector achieves
**recall 1.00 [0.84, 1.00] at FPR 0.007 [0.002, 0.021]**, beating request-only,
static-correlation, and graph-correlation (each either misses labor-divided
swarms or floods on benign co-targeting). **H1 null rejected.** Ablations: removing cross-source correlation
collapses recall to 0.00; removing the synchrony gate inflates FPR to 1.00;
removing the decoy signal leaves PR-AUC unchanged (coordination is caught without
lures — the deception-aware swarm is 10/10).

### H2 — adaptive deception reduces objective completion ([R3](deception-experiment.md))

Objective completion across arms (500 trials): unprotected 1.00 → conventional
1.00 → static 0.88 → stateful 0.40 → **adaptive 0.00**. **H2 null rejected**
(>99% relative reduction). Zero benign disruption for every deception arm; only
the rate limiter disrupts benign traffic (5%). Against a **deception-aware**
adversary, static decoys are discovered 100% and never detected, while
consistent stateful/adaptive lures are never discovered and hold completion at
0.40 / 0.00.

### H3 — the defender adapts fast enough to keep positive utility ([R4](coevolution-lab.md))

Adaptation latency is bounded and decomposes into detection + response; the
**response component is a fixed 4.0s (inside the ≤5s target)**, detection grows
gracefully 5→12s as the adversary gets stealthier. Net utility is positive
(0.93) for detectable adversaries. **H3 null rejected** for them — but the
**feinting adversary exposes the limit**: 67% countered, 50% policy oscillation,
utility 0.45 (reported, not hidden).

### H4 — graceful degradation under a missing sensor

Now quantified by the [missing-sensor sweep](missing-sensor-h4.md) (`heramba
research h4`): removing the target-overlap or decoy sensor is **graceful** (recall
stays 1.00 — they are redundant), while removing synchrony or the
division-of-labor sensor **collapses** recall for labor-dividing swarms. The
detector needs synchrony plus the applicable co-targeting signal, so H4 remains
**PARTIAL** — measured per sensor, not asserted.

### H5 — safety holds, overhead bounded ([R5](data-plane-safety.md), [R6](../security/proxy-resilience.md))

Attacker-controlled input can never change a policy decision: proven structurally
(taint types) and by ~3.7M-exec fuzzing of the policy with zero invariant
violations, plus identical clean-vs-hostile decisions. Added proxy latency is
sub-millisecond (p50 ~47µs, p99 ~169µs). **H5 null rejected** on both the safety
and overhead sub-claims.

## Targets vs. observed ([R8](statistics.md))

| metric | target | observed | verdict |
|---|---|---|---|
| recall (held-out coordinated) | ≥ 0.90 | 1.00 | ✅ |
| false-positive rate | ≤ 0.01 | 0.007 [0.002, 0.021] (n=420) | ✅ (point; UB 0.021) |
| objective-completion reduction | ≥ 0.50 | 1.00 | ✅ |
| policy-safe response latency | ≤ 5s | 4.0s | ✅ |
| valid-artifact run rate | ≥ 0.95 | 1.00 | ✅ |

Precision at a realistic 1% prevalence is 0.586 — even a 0.7% FPR is punished by
rare-event prevalence, so the pooled synthetic precision overstates field
precision.

## Limitations & threats to validity

- **Synthetic traffic is more separable than production**; all numbers are
  harness measurements, not field claims.
- The **FPR misses the 1% target** and the benign sample is **underpowered**
  (n=32 ≪ ~381 needed); the coordinated branch can false-positive on benign
  clients that are tightly synchronized and share endpoints.
- The **deception-aware adversary and the arm/co-evolution models are
  heuristics**, not live LLM agents.
- H4 is **PARTIAL**; CPU/memory/recovery are not measured by the deterministic
  harnesses.

## Ethics & safety

Defensive research only. No live targets, no real exploitation, no propagation,
no credential theft, no destructive behavior; synthetic secrets use reserved
domains and cannot work on real systems (isolation tests). No deployable
offensive payloads are committed. No certification or "military-grade" claim is
made.
