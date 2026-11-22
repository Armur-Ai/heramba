# internal/dashboard

A self-contained, embeddable **dark enterprise web dashboard** for Heramba's
active-defense / deception edge. Decoupled from the rest of the system: it
consumes detection events and a metrics snapshot through function fields
(mirroring `internal/admin`) and drives write controls through a `Controller`
the caller supplies. The entire frontend — HTML, CSS, JS, and a hand-rolled
MIT-licensed SVG charting micro-library — is embedded via `go:embed`. **No CDN,
no build step, no new module dependencies** (stdlib only). Runs fully offline.

## Public API

```go
// Build a handler from functional options (all optional).
h := dashboard.Handler(
    dashboard.WithEvents(src.Recent),     // func(n int) []event.Event, newest first
    dashboard.WithMetrics(src.Metrics),   // func() dashboard.Metrics
    dashboard.WithController(ctl),         // dashboard.Controller (write side)
    dashboard.Authorize("bearer-token"),   // optional: guards POST /api/control/*
)
http.ListenAndServe("127.0.0.1:7171", h)  // bind localhost by default

// Or run standalone against a synthetic live attack stream:
h := dashboard.DemoHandler()               // seeds a backlog + live ticker
```

### Options

| Option | Purpose |
| --- | --- |
| `WithEvents(EventsFunc)` | `func(n int) []event.Event` — recent events, newest first (same shape as `admin.EventsFunc`). |
| `WithMetrics(MetricsFunc)` | `func() Metrics` — metrics snapshot. |
| `WithController(Controller)` | Write side for the control panel. Defaults to a safe in-memory `MemoryController`. |
| `Authorize(token string)` | Require `Authorization: Bearer <token>` on all `POST /api/control/*`. Empty token (default) leaves writes open — intended for localhost binds. |
| `WithBroker(*Broker)` | Pub/sub broker the live stream (`GET /api/stream`) fans out. Supply one that your event source also publishes to for instant UI updates; otherwise a private broker is created so the endpoint still works (heartbeats only). |

### Controller

```go
type Controller interface {
    SetMode(mode string) error          // "observe" | "protect"
    Clear() error                        // reset counters
    SetPaused(paused bool) error         // pause/resume pipeline
    ApplyDecoyPreset(preset string) error
}
```

`MemoryController` (via `NewMemoryController()`) is the safe default and records
state without touching a real edge.

### Types

- `Metrics{ Mode, Total, ByKind, BySeverity, Sources, Uptime, Score }` — mirrors
  `admin.Metrics` (local copy to stay decoupled).
- `ScoreHistogram{ Bounds, CumCounts, Sum, Count }` — cumulative agentic-score
  histogram; `CumCounts` has `len(Bounds)+1` entries (trailing `+Inf`).

### Demo source

- `NewDemoFeed()` / `NewDemoFeedAt(seed int64, now time.Time)` (deterministic).
- Methods: `Recent(n) []event.Event` (an `EventsFunc`), `Metrics() Metrics` (a
  `MetricsFunc`), `Seed(n)` to backfill, `Tick(dt)` to append a live batch.
- `DemoHandler(opts...)` wires a seeded feed + background ticker.

## HTTP endpoints

Read (GET):

| Path | Returns |
| --- | --- |
| `/` | Embedded SPA shell (HTML). |
| `/app.css`, `/app.js`, `/vendor/charts.js` | Embedded static assets. |
| `/api/events?n=` | `[]event.Event` (encodes `[]`, never `null`; `n` capped). |
| `/api/metrics` | Metrics snapshot (`uptime_seconds`, `by_kind`, `by_severity`, `score`). |
| `/api/timeline?window=` | Bucketed counts over the trailing window (60 buckets). |
| `/api/sources` | Per-source aggregates, worst first. |
| `/api/sources/{id}` | Per-source drill: that source's events (newest first, capped) + rolled-up kinds/targets/severity/first-last. `?source=` also accepted. |
| `/api/breaches` | Confirmed breaches — `canary_triggered` events (count, distinct sources, per-target, recent). |
| `/api/campaigns` | Coordinated multi-IP swarms (>=2 sources; `coordinated` at >=3). |
| `/api/decoys` | Per-target decoy hit-map rows (also drives the Top Targets panel). |
| `/api/stream` | **Server-Sent Events** live feed: `data: {json}\n\n` frames (`{"type":"event",…}` / `{"type":"metrics",…}`) plus heartbeat comments. Primed with a metrics frame; subscription torn down on disconnect. |
| `/api/export/events.csv` | Recent events as a downloadable CSV attachment (`?n=` honoured/capped). |
| `/api/export/events.json` | Recent events as a downloadable, pretty-printed JSON attachment. |
| `/healthz` | `ok`. |

All API responses carry `Cache-Control: no-store`; every response carries
`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, a strict same-origin
CSP and `Referrer-Policy: no-referrer`. Registered paths return `405` for the
wrong method.

### Live stream broker

`GET /api/stream` is fed by a `Broker` — a dependency-free channel fan-out:

```go
b := dashboard.NewBroker()
h := dashboard.Handler(dashboard.WithEvents(src.Recent), dashboard.WithBroker(b))

// publish from your event source (non-blocking; drops frames for a slow client):
b.Publish(dashboard.EventFrame(ev))               // a detection event
b.Publish(dashboard.StreamFrame{Type: "metrics"}) // a metrics refresh
b.Close()                                          // on shutdown
```

`Publish` fans out inline on the caller's goroutine (the broker owns none and
cannot leak them); `Subscribers()` reports the current connection count. The
demo feed publishes every generated event, so `DemoHandler` exercises the full
SSE path.

Write (POST, guarded by `Authorize` when set — 401 otherwise):

| Path | Body |
| --- | --- |
| `/api/control/mode` | `{"mode":"observe"\|"protect"}` |
| `/api/control/clear` | _(none)_ |
| `/api/control/pause` | `{"paused":true\|false}` |
| `/api/control/decoy-preset` | `{"preset":"<name>"}` |

## Security

The dashboard exposes write controls and should **bind localhost by default**.
When exposing it beyond loopback, always set a bearer token via `Authorize` so
`/api/control/*` rejects unauthenticated writes.

## Tests

`go test ./internal/dashboard/...` covers every endpoint (status + content-type),
`[]` encoding on empty events, JSON shapes, timeline bucketing math, the
per-source drill and confirmed-breaches aggregates, CSV/JSON export headers and
bodies, the SSE broker (fan-out, idempotent unsubscribe, drop-on-slow, close)
and the `/api/stream` endpoint over `httptest` (content type, priming frame,
event delivery, subscriber teardown on cancel), the hardening guarantees
(security headers, `no-store`, `405`, `n` caps), control-auth (reject/accept),
the demo feed, the offline-asset guarantees (no external script origins), and an
end-to-end pass over a demo-backed handler.
