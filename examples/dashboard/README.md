# Example: Heramba with live dashboards

This example shows how to put a live view in front of a running Heramba edge —
both the `watch` TUI and the `heramba dashboard` web console. The full guide is
[docs/dashboards.md](../../docs/dashboards.md).

The dashboards are enabled on the command line, not in the config. The config
([`.heramba.yml`](.heramba.yml)) only decides where events are written (`alert.file`)
and whether the admin listener is up (`admin_listen`).

## Terminal TUI over the sink file

```bash
# start the sample upstream and the edge (writes heramba-events.jsonl)
go run ./examples/sample-app &
heramba serve --config examples/dashboard/.heramba.yml

# in another terminal: tail the live event stream as a TUI
heramba watch heramba-events.jsonl
```

## Web console, read-only, over the same sink

```bash
# flags come before the source argument
heramba dashboard --listen 127.0.0.1:7171 heramba-events.jsonl
# open http://127.0.0.1:7171  (control panel is read-only when attached this way)
```

Or with no edge at all, a self-contained demo feed:

```bash
heramba dashboard --demo
```

## Web console with LIVE controls (mounted by the edge)

To change posture from the browser (observe ⇄ protect, decoy preset), mount the
dashboard on the edge itself. Always set a bearer token when it is reachable
beyond localhost — it guards the `POST /api/control/*` writes:

```bash
heramba serve \
  --config examples/dashboard/.heramba.yml \
  --dashboard-listen 127.0.0.1:7171 \
  --dashboard-token "$(head -c 24 /dev/urandom | base64)"
```

The startup banner reports which it is:

```
dashboard: web console on http://127.0.0.1:7171 — LIVE controls (bearer-token protected)
```

## Try it quickly

```bash
# a decoy probe fires a zero-false-positive detection that shows up live
curl -s -o /dev/null http://127.0.0.1:8080/.env
```
