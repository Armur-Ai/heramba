# counter — active-defense extensions (`escalate.go`)

New file added alongside the existing `counter.go` (which is untouched). It
extends counter-agent deception with a larger lure pool and per-source
escalation, and emits `event.KindCounterDeployed`.

```go
lure := counter.GenerateAdvanced(-1)   // draws from base + extended lead pool

camp := counter.NewCampaign()
tier := camp.Escalate(src)              // none -> single -> multi -> flood
lures := counter.LuresFor(tier)         // 0/1/3/6 lures for the tier
body = counter.InjectHTMLAll(body, lures)

ev := counter.DeployEvent(src, tier, lures) // KindCounterDeployed, sev medium
keys := counter.Keys(lures)             // register as canaries
paths := counter.DecoyPaths(lures)      // register as decoys
```

- `Campaign` tracks per-source offense counts and escalates the counter-
  deception `Tier` the longer an agent keeps coming back (`Tier`, `Count`,
  `Reset`). Safe for concurrent use.
- `LuresFor(tier)` mints the matching number of lures; each still carries a
  unique tracking key (watch as a canary) and fabricated paths (register as
  decoys), exactly like `Generate`.
- `DeployEvent` records *our* injection action (confidence 0.9, medium
  severity), labeled with tier, lure count, keys and decoy paths.

Edge wiring: on each confirmed-agentic response to a source, call
`camp.Escalate(src)`, inject `LuresFor(tier)` with `InjectHTMLAll`, register the
keys/paths, and emit `DeployEvent`.
