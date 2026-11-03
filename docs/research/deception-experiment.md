# Deception & attack-objective experiment (R3)

Tests **H2**: *adaptive deception reduces attacker objective completion relative
to no defense, conventional rate limiting, and static decoys* — and, crucially,
holds up against a **deception-aware adversary** that checks for inconsistencies
and rejects obvious lures.

```bash
heramba research objective                       # naive adversary
heramba research objective --deception-aware     # adversary that rejects weak lures
heramba research objective --artifacts docs/research/artifacts/objective
```

Code: [`internal/research/objective`](../../internal/research/objective).
Reference bundle: [`artifacts/objective/`](artifacts/objective/).

## Two halves: real safety, modeled dynamics

**The safety claims are real, tested code** against the actual `deception.World`
and `assetgraph`:

- `ValidateWorld` checks the synthetic world for the five consistency dimensions
  a deception-aware adversary probes — identifiers (unique, referentially
  intact), permissions (coherent ownership/membership), workflows (resources
  reference real projects), temporal state (deterministic, non-zero), and the
  structural `Synthetic` tag. It is asserted to **pass on generated worlds** and
  to **catch injected inconsistencies** (dangling refs, a non-synthetic record,
  a real-looking email domain).
- **Real/synthetic isolation** is proven: every synthetic email uses an
  RFC-reserved domain (`.test`/`.example`/…), so a synthetic secret can never
  resolve to a real system; and the real asset graph **fails closed** on
  adversary export (`WriteAdversaryView` errors), while the synthetic graph
  exports fine.

**The five-arm comparison is a transparent, seeded model** of the defenses (not
the live proxy — proxy overhead is measured separately in R6). An attacker makes
progress toward an objective across a surface polluted with decoys; a believable
lure diverts (and, where the arm detects on decoy contact, contains and forces a
retry), unless the attacker is deception-aware and the lure is inconsistent, in
which case it recognizes and skips it. The model is labeled a model everywhere.

## Reference results (revision `fe40522`, 500 trials/arm, depth 8, budget 60)

### Naive adversary

| arm | objective completion | diverted | detection lat (s) | benign disruption |
|---|---|---|---|---|
| unprotected | 1.00 [0.99, 1.00] | 0.00 | n/a | 0.00 |
| conventional (rate limit) | 1.00 [0.99, 1.00] | 0.00 | n/a | **0.05** |
| static_decoys | 0.88 [0.85, 0.91] | 0.78 | 3.2 | 0.00 |
| stateful_deception | 0.40 [0.36, 0.44] | 0.86 | 2.7 | 0.00 |
| **adaptive_deception** | **0.00 [0.00, 0.01]** | 0.95 | 2.7 | 0.00 |

**H2's null is rejected**: adaptive deception drives objective completion to
~0 — a >99% relative reduction versus the predeclared baselines — while every
deception arm keeps **zero benign disruption** (benign clients never touch a
decoy). Only the blunt rate limiter disrupts legitimate traffic (5%).

### Deception-aware adversary (rejects lures below 0.60 consistency)

| arm | objective completion | diverted | detection lat (s) | deception discovery |
|---|---|---|---|---|
| static_decoys | 0.88 [0.85, 0.91] | 0.00 | **n/a (never detected)** | **1.00** |
| stateful_deception | 0.40 [0.36, 0.44] | 0.86 | 2.7 | 0.00 |
| adaptive_deception | 0.00 [0.00, 0.01] | 0.95 | 2.7 | 0.00 |

This is the substantive finding and **why the consistency validator matters**:
the low-consistency static decoys (0.30) are **discovered 100% of the time and
evaded with zero detections** — against a smart adversary they are worthless.
The consistent stateful (0.85) and adaptive (0.90) lures are **never discovered**
(0.00), so they keep diverting *and* detecting the deception-aware attacker.
Adaptive deception holds objective completion at ~0 against both adversaries.

## Honest limitations

- The five-arm comparison is a **behavioral model**, not the live engine; it
  abstracts the defenses to isolate the objective/diversion dynamics. The
  consistency validator and isolation proofs are the real, tested parts.
- CPU/memory are not measured here (this is not a load test); proxy overhead is
  R6's job.
- The deception-aware adversary is a threshold heuristic, not a live LLM agent.
