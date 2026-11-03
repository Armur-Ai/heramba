# Proxy correctness & resilience (R6)

The edge is an inline reverse proxy in front of production traffic, so its first
duty is to **never silently corrupt an application**. This suite
([`internal/edge/resilience_test.go`](../../internal/edge/resilience_test.go))
exercises the real `edge.Engine` against live `httptest` backends.

```bash
go test ./internal/edge/...            # correctness + resilience + SLO
go test -race ./internal/edge/...      # race-clean
```

## Correctness (real backends, both modes)

| Property | Test |
|---|---|
| Keep-alive / connection reuse (25 reqs, one client) | `TestKeepAliveConnectionReuse` |
| Chunked / incrementally-flushed streaming | `TestChunkedStreamingResponse` |
| Server-Sent Events pass through untouched | `TestSSEPassthroughProtectMode` |
| 8 MiB body byte-for-byte (SHA-256), observe **and** protect | `TestLargeBodyIntegrity` |
| gzip response never rewritten / corrupted | `TestGzipResponseNotCorrupted` |
| Set-Cookie, Cache-Control, redirects (301 + Location) | `TestHeaderCookieRedirectPassthrough` |
| Request (POST) body forwarded intact | `TestRequestBodyForwarded` |
| Observe/protect **body parity** for benign traffic | `TestObserveProtectBodyParity` |

**No unsafe transforms by default.** Observe mode never alters a body or header
(body parity test). In protect mode the only body rewrite is counter-agent lure
injection, and it is **disabled on any `Content-Encoding`** (so compressed
responses are never touched) and fixes `Content-Length` to the new body length —
verified: a gzip response round-trips intact and keeps its `Content-Encoding`.

## Resilience & fault injection

| Fault | Documented safe behavior | Test |
|---|---|---|
| **Upstream outage** (backend dead) | edge returns **502**, no hang/panic | `TestUpstreamOutageReturns502` |
| **Telemetry/sensor outage** (alert sink errors) | traffic unaffected — decoy still 404s, real traffic flows; error ignored | `TestSinkFailureDoesNotCorruptTraffic` |
| **Client cancellation** | client sees the cancellation; edge does not wedge; next request works | `TestRequestCancellation` |
| **Graceful shutdown** | in-flight request drains to completion during `Shutdown` | `TestGracefulShutdownDrainsInflight` |

The sensor-outage case is the key fail-safe: the alert sink is the only
downstream dependency on the hot path, and a failing sink must not break
application traffic. (Note: the sink is called synchronously; a *slow* sink would
add latency, so production deployments should use a buffered/async sink — tracked
as a hardening note, not a silent corruption.)

## Added-latency SLO (measured, not synthetic zero-work)

Measured as the per-percentile difference between requests through the edge and
requests straight to a backend doing realistic small work (SHA-256 + 1 KiB body),
300 samples. Reference run on this machine:

| percentile | added latency |
|---|---|
| p50 | **~47 µs** |
| p95 | ~62 µs |
| p99 | ~169 µs |

The suite asserts a generous **p99 < 50 ms** ceiling to catch gross regressions
without flaking on CI timing; the real overhead is sub-millisecond. CPU/memory
under sustained load are **not** claimed here — this is an overhead measurement,
not a saturation load test; a full load profile needs a dedicated environment.

## Scope / honest gaps

- HTTP/2 and WebSocket upgrade are carried by Go's standard reverse proxy and
  transport; they are **not** separately asserted here (no TLS/h2 fixture yet) —
  tracked, not claimed.
- The latency numbers are single-machine and will vary by hardware; they are a
  regression guard and a rough overhead figure, not a published SLA.
