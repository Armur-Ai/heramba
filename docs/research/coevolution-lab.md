# Adaptation race & co-evolution lab (R4)

Tests **H3**: *the defender can adapt to a bounded attacker strategy shift
quickly enough to retain positive utility* — and shows where adaptation **helps,
hurts, and fails**.

```bash
heramba research coevolve                          # markdown summary
heramba research coevolve --json
heramba research coevolve --artifacts docs/research/artifacts/coevolve
```

Code: [`internal/research/coevolve`](../../internal/research/coevolve).
Reference bundle: [`artifacts/coevolve/`](artifacts/coevolve/), including a
replayable event log for the feinting adversary.

## Strictly local, event-sourced, seeded

Everything is a harmless surrogate: logical ticks and labeled events, **no live
targets, no exploitation, no propagation, no credential theft, nothing
destructive**. Each round, an attacker shifts strategy at `t_change` and the
defender's pipeline advances through `t_observed → t_detected → t_decision →
t_applied → t_effective`; if the attacker reaches its objective first, that is
`t_objective`. The append-only event log replays byte-identically from a seed
(asserted in tests).

**Adaptation latency = `t_effective − t_change`**, decomposed into a **detection
component** (`t_detected − t_change`, which depends on how detectable the
adversary is) and a **response component** (`t_effective − t_detected`, the fixed
cost of deciding and rolling out a bounded policy change).

Defender **utility** per round = objective-prevention value − (false-positive +
compute) update costs − any feint-induced oscillation penalty.

## Reference results (revision `679597c`, 8 rounds/race, 100 seeds)

### Adaptive defender

| archetype | countered | adaptation latency (s) | detection | response | mean utility | oscillation |
|---|---|---|---|---|---|---|
| scripted | 1.00 | 9.0 | 5.0 | 4.0 | 0.93 | 0.00 |
| independent | 1.00 | 10.0 | 6.0 | 4.0 | 0.93 | 0.00 |
| coordinated | 1.00 | 11.0 | 7.0 | 4.0 | 0.93 | 0.00 |
| deception_aware | 1.00 | 14.0 | 10.0 | 4.0 | 0.93 | 0.00 |
| **adversarial_varied** | **0.67** | 16.0 | 12.0 | 4.0 | **0.45** | **0.50** |

**H3's null is rejected** for the detectable adversaries: adaptation latency is
**bounded** and net utility is clearly positive (0.93). The **response component
is a fixed 4.0s — inside the ≤5s policy-safe response target** (R8); the variable
cost is *detection*, which grows as the adversary gets stealthier (5s → 12s).

### Where adaptation helps, hurts, and fails

- **Helps** (scripted → deception-aware): the defender counters every shift; the
  only cost is detection latency, which degrades *gracefully*, not
  catastrophically, as detectability falls.
- **Hurts / fails** (adversarial_varied): a **feinting** adversary that shifts
  then reverts induces **policy oscillation 50%** of rounds and is countered only
  67% of the time, dragging utility down to 0.45. This is the honest limit —
  adaptation against a deliberately unstable adversary is lossy.

### Ablation (utility by defender mode)

| archetype | no_adaptation | adaptive | instant (upper bound) |
|---|---|---|---|
| scripted…deception_aware | 0.00 | 0.93 | 0.93 |
| adversarial_varied | 0.00 | 0.45 | 0.93 |

No-adaptation counters nothing (utility 0). Against stable adversaries the real
adaptive defender matches the physically-unattainable instant bound; against the
feinting adversary the gap (0.45 vs 0.93) is exactly the price of latency plus
oscillation.

## Honest caveats

- Ticks are **logical**; `TickSeconds` maps them to seconds for reporting only.
  This is a behavioral race model, not a wall-clock benchmark.
- Jitter is **seeded and bounded**; a live deployment adds observational
  nondeterminism not modeled here.
- "Detectability" is a per-archetype parameter, not a claim that a given real
  adversary has a given detectability.
