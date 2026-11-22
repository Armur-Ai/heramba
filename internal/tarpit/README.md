# internal/tarpit

Time-wasting slow responses and an endless fake-link maze for **confirmed-bad**
sources. Once a source is confirmed malicious (tripped a wire, hit a canary,
exhausted a decoy), make its own future requests to us crawl — burning an
automated attacker's wall-clock and budget.

This is defensive only: a tarpit **delays and limits the attacker's own request
against our endpoint** and serves bounded synthetic content. It never scans,
attacks, or sends traffic to the client. Every wait honors context cancellation
and no goroutine is spawned, so a disconnecting client leaks nothing.

## Dripper (`tarpit.go`)

```go
tp := tarpit.New(tarpit.Config{ChunkSize: 1, Interval: 200*time.Millisecond, Jitter: 50*time.Millisecond, MaxTotal: 30*time.Second})
n, err := tp.Drip(ctx, w, body)   // writes body slowly; err is ctx.Err() on cancel/budget
h := tp.ServeHandler("application/json", func(r *http.Request) []byte { return fakeBody(r) })
```

- `Drip` writes `body` one `ChunkSize` chunk at a time, waiting
  `Interval(+jitter)` before each chunk after the first, flushing `http.Flusher`
  writers. Runs in the caller's goroutine. Returns `context.Canceled` /
  `context.DeadlineExceeded` on cancel or `MaxTotal`.
- `NewSeeded(cfg, seed)` makes jitter reproducible for tests.

## Escalation (`escalate.go`)

```go
e := tarpit.NewEscalator(base, max, tarpit.Exponential) // or tarpit.Linear
delay := e.Offense(src)          // record an offense, get the new delay
cfg := e.ConfigFor(src, 1, jit)  // tarpit.Config with Interval = current delay
```

Delay grows with repeated offenses — `Linear`: `base*n`; `Exponential`:
`base*2^(n-1)` — capped at `max` (`0` = uncapped). `Delay`, `Count`, `Reset`
round it out. Safe for concurrent use.

## Labyrinth (`labyrinth.go`)

```go
lab := tarpit.NewLabyrinth(seed, "/maze", fanout, maxDepth)
h := lab.Handler(tp /*or nil*/, func(path string){ /* record a decoy hit */ })
html := lab.HTML(path)   // bounded page; Page(path) for the structured form
```

An endless tree of believable interlinked pages. Each page is a **pure function**
of its URL and the seed, so serving one is O(1) in time and memory (fanout is a
small constant) and no global state grows. Links descend until `maxDepth`, then
loop back to depth 1 — URLs stay bounded while the maze stays endless. A crawler
follows it forever; we never run out of RAM.

## Mounting at the edge

1. Keep one `Escalator` process-wide. When a source is confirmed bad (e.g. a
   `tripwire` or `decoy` hit), call `e.Offense(src)`.
2. For that source's subsequent requests, build `tp := tarpit.New(e.ConfigFor(src, 1, jitter))`
   and either `tp.Drip(r.Context(), w, body)` a canned fake, or route the source
   into `lab.Handler(tp, onVisit)` so it wanders the maze on a drip.
3. `onVisit` → emit `event.Deception(event.KindDecoyHit, ...)` per maze request.
