# Phase 2 threat model

The research threat model is encoded as `research.ActorModel` entries in
`research.DefaultProtocol()` so experiments reference it directly. Each actor
has explicit **capabilities**, a **request budget**, a **time budget**, an
**objective**, its **target visibility**, and what the **defender can see and
control**. Fixing budgets and visibility is what makes a *fair* comparison
possible (see [R2](protocol.md)).

## Actor families

| Actor | Kind | Shape | Objective | Adaptive |
|---|---|---|---|---|
| `scripted_scanner` | scripted | one source, sequential enumeration, fixed wordlist | find admin surface | no |
| `independent_scanners` | independent | N sources, parallel, **no shared memory** | find admin surface | no |
| `coordinated_swarm` | coordinated | N sources with **shared observations**, divided targeting, synchronized timing | find & exfiltrate | no |
| `legit_automation` | legit_automation | high volume but **narrow, known** endpoints | none (benign) | no |
| `human_tester` | human | low volume, exploratory, irregular timing | manual probe | no |
| `deception_aware_swarm` | deception_aware | coordinated **plus** consistency-checking and lure rejection | find & exfiltrate | **yes** |

The last family is the hard case and the held-out one: an adversary that checks
synthetic resources for inconsistencies and rejects obvious lures. It is a
**modeled heuristic**, not a live LLM — the model never claims otherwise.

## What the defender may see and do

Observables are limited to transport-level and behavioral signals: requests,
timing, decoy hits, and (for coordinated families) target overlap. Controls are
the bounded, typed actions from `internal/strategy`: rate-limit, decoy
activation, session restriction, escalation. No control is global and none is
free-form.

## Boundaries (non-negotiable)

- **Strictly local, isolated, authorized.** No live internet targets, no real
  exploitation, no worm-like propagation, no credential theft, no destructive
  behavior. All adversaries are synthetic traffic generators.
- **Synthetic secrets never work on real systems**; no real customer data is
  ever placed in a decoy. R3 ships a consistency validator and isolation tests
  that assert this.
- **Coordination ≠ attribution.** The defender may conclude "these sources look
  coordinated"; it may not conclude "this is an AI" from traffic alone.
- **No LLM on the enforcement path.** An LLM may *propose*; the typed policy
  decides from typed fields only. R5 hardens this with taint labels and an
  adversarial test battery.

## Attacker adaptation

The co-evolution lab (R4) alternates **bounded** attacker strategy shifts with
defender policy updates, using harmless surrogate actions only. It measures how
fast and how well the defender adapts — and where adaptation *hurts* (policy
oscillation, over-containment) — rather than assuming the defender wins.
