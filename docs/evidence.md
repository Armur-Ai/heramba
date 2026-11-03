# Evidence model

The evidence model (`internal/evidence`) is the foundation of Heramba's
intelligence plane. It exists to keep one distinction absolutely clear: **what
Heramba observed** versus **what Heramba inferred**. Confusing the two is how
defensive tools end up blocking real users on a guess.

## The vocabulary

| Type | Meaning | Confidence |
|---|---|---|
| `SecurityEvent` | A normalized, versioned atomic observation (a decoy hit, an agentic score crossing threshold, a blocked callback), with provenance and a tentative attack `Stage`. | carries the detection's own confidence (1.0 for deception) |
| `Fact` | A validated restatement of one or more observations with **no inference** ("source X requested decoy /.env"). Requires ≥1 derivation. | 1.0 by construction |
| `Hypothesis` | An **inferred** claim ("sessions A and B are the same actor") carrying *both* supporting and contradicting evidence, a confidence, an uncertainty, a review deadline, and a status. | < 1.0; never certainty from inference |
| `Relationship` | A typed, directed edge between records. `IsInference()` separates probabilistic links (`same_actor`) from observational ones (`same_source`). | per-edge |

A `Hypothesis` can gain confidence (`Support`), lose it (`Contradict`), **decay**
over time toward a floor, and **expire** past its review deadline — so stale
inference loses force instead of lingering as fact.

## Provenance is mandatory

Every stored record names the **sensor** that produced it and an opaque
**raw reference** so a human can audit back to the ground truth. A record
without provenance fails validation and is never stored.

## The store

`evidence.Store` is concurrency-safe and validates **before** it accepts — a
malformed or hostile record never enters (or persists). With `OpenStore` it
appends to a JSONL log using a tagged envelope, round-trips on reload, and
**soft-skips** records of an unknown schema version rather than crashing. Query
by transport source key (`EventsBySource`) or time window (`EventsInWindow`).

## Why a source IP is not an identity

`SecurityEvent.SourceKey` is a *transport* key. Grouping events by it into a
`Session` is an observation. Linking sessions across time or across keys into an
`Actor` is a `Hypothesis` with its own evidence and uncertainty — see
[campaign intelligence](campaign.md). This separation is enforced structurally,
not by convention.
