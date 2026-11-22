# deception — active-defense extensions

New files added alongside the existing world/app/session (which are untouched).

## Breadcrumb trail (`breadcrumb.go`)

A coherent fake "trail" that leads an attacker from one decoy to the next.

```go
tr := deception.BuildTrail(seed, length)   // deterministic, length >= 2
entry := tr.Entry()                         // seed this from a tripwire/lure
for _, b := range tr.Steps() { ... }
body := tr.RenderHint(step)                 // onward link, or final secret blob
ev := tr.Event(step, src)                   // zero-FP decoy hit + trail labels
tr.Paths()    // register as decoys
tr.Secrets()  // register as canaries (the payoff)
```

Each step's `Next` equals the following step's `Path`, so following the
breadcrumbs walks decoy to decoy until a synthetic jackpot secret on the last
step. Same seed → identical trail (mirrors `GenerateWorld`).

## Tarpit-backed slow mode (`slowapp.go`)

Wrap the synthetic `App` so its responses drip out slowly, turning the honeypot
into a time sink for a confirmed-bad source without touching any handler.

```go
slow := deception.NewSlowApp(app, tarpit.Config{ChunkSize: 1, Interval: 200*time.Millisecond})
// or share a Tarpit (e.g. per-source escalation):
slow := deception.NewSlowAppTarpit(app, tp)
http.Handle("/synthetic/", slow)   // slow.ServeHTTP drips the App's full response
```

The App still computes its complete answer (world stays coherent, every
interaction recorded); `SlowApp` captures status/headers/body and re-emits them
on the tarpit schedule, honoring request-context cancellation. Uses
`internal/tarpit` (no new module dependency).
