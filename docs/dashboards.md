# Dashboards: `heramba watch` and `heramba dashboard`

Heramba ships two live views over the same detection event stream:

- **`heramba watch`** — a terminal (TUI) attack dashboard.
- **`heramba dashboard`** — a dark enterprise web console in the browser.

Both attach to the **JSONL event stream** a running edge emits (its alert
sink), so neither couples to the engine. You can follow a live sink file, read a
pipe, or run a self-contained demo. The one that changes posture live is the web
dashboard mounted *by the edge itself* via `serve --dashboard-listen`, covered
at the end.

## Getting an event stream to watch

Both tools resolve their source the same way: a path argument, `-` for stdin, or
(when omitted) the `alert.file` from your `.heramba.yml`. The common patterns:

```bash
# follow the sink file configured in .heramba.yml
heramba watch

# follow a specific JSONL sink file
heramba watch events.jsonl

# live over a pipe straight from the edge
heramba serve --alert jsonl - | heramba watch -

# replay a recorded capture as a paced demo (no edge needed)
heramba watch --replay demo/attack.jsonl --speed 8
```

Everything below also works unchanged for `heramba dashboard` (swap the command
name), because they share the source-resolution and follow/replay plumbing.

> Put flags **before** the source argument (`heramba watch --mode protect
> events.jsonl`). Flags after a positional argument are not parsed.

## `heramba watch` — the TUI

```
$ heramba watch --help
usage: heramba watch [flags] [source]

source is a JSONL event file, '-' for stdin, or empty to use the
alert sink from config. Starts a live attack dashboard.

flags:
  -config string   config file (for the default source + mode) (default ".heramba.yml")
  -follow          tail the source for new events (tail -f) (default true)
  -max int         events kept in the live feed (default 200)
  -mode string     posture label in the header: observe|protect (default: from config)
  -plain           no TUI: print events as text (non-interactive / piping)
  -replay          replay the source as a recorded attack, paced by timestamp (for demos)
  -speed float     replay speed multiplier (use with --replay) (default 8)
```

### The views

A tab bar across the top switches between four bodies (press the number, or
`tab` / `shift+tab` to cycle):

1. **Overview** — the default split: a live event feed on the left, a
   source / campaign / top-decoy sidebar on the right.
2. **Sources** — a full-width table of attacking sources. Press `o` to cycle the
   sort: urgency (default), hits, decoys, recent.
3. **Campaigns** — coordinated multi-IP swarms grouped together.
4. **Timeline** — activity over time.

### The threat gauge

The header carries a single threat gauge that answers the only question an
operator has in the first second — *how bad is it right now?* It bands the live
score as `CALM → GUARDED → ELEVATED → HIGH → CRITICAL`, colored accordingly,
driven by severity, sustained volume, and whether a swarm is forming.

### The inspector

Move the feed cursor with `↑`/`↓` (or `j`/`k`) and press `enter` to open the
inspector on the selected event — the full labels, confidence, source, and
detail for one detection. `enter` again closes it.

### Filtering

Press `f` to cycle the feed's severity floor, so you can drop low-severity noise
and keep only medium-and-up (or high-and-up) during a busy incident. The filter
selects the *n most recent events that pass*, not merely the survivors of the
last n.

### Snapshot export

Press `s` to export a point-in-time Markdown snapshot — threat level, totals,
by-kind / by-severity breakdowns, and the top sources and decoys — written to a
timestamped file (`heramba-watch-YYYYMMDD-HHMMSS.md`) you can drop straight into
a ticket or a chat. A toast confirms the path.

### Keybindings

| Key | Action |
|---|---|
| `1` – `4` | jump to Overview / Sources / Campaigns / Timeline |
| `tab` / `shift+tab` | cycle views forward / back |
| `← →` / `h l` | cycle views |
| `space` / `p` | pause & resume the live stream |
| `↑ ↓` / `j k` | move the feed cursor |
| `enter` | inspect the selected event |
| `f` | cycle the feed severity filter |
| `o` | cycle the Sources sort order |
| `s` | export a Markdown snapshot |
| `c` | clear all accumulated state |
| `?` | toggle the help overlay |
| `q` / `ctrl+c` | quit |
| `esc` | close help, or quit |

### No TUI? Use `--plain`

For piping or a non-interactive terminal, `--plain` prints events as text
instead of drawing the TUI:

```
$ heramba watch --plain --follow=false demo/attack.jsonl
[10:00:00] low probe src=198.51.100.23 target="/robots.txt" conf=0.00 benign fetch
[10:00:01] medium agentic_traffic src=198.51.100.23 target="/api/v1/users" conf=0.71 classifier: tool-like timing, no JS
[10:00:01] high decoy_hit src=198.51.100.23 target="/.env" conf=1.00 touched decoy env file
[10:00:02] high decoy_hit src=198.51.100.23 target="/.git/config" conf=1.00 touched decoy VCS config
```

## `heramba dashboard` — the web console

```
$ heramba dashboard --help
usage: heramba dashboard [flags] [source]

source is a JSONL event file, '-' for stdin, or empty to use the
alert sink from config. Serves a dark enterprise web dashboard.

flags:
  -config string   config file (for the default source + mode) (default ".heramba.yml")
  -demo            serve a self-contained synthetic attack feed (no edge required)
  -follow          tail the source for new events (tail -f) (default true)
  -listen string   address to serve the dashboard on (default "127.0.0.1:7171")
  -max int         events retained for the feed and aggregates (default 500)
  -token string    bearer token required to use the control endpoints (empty = open; localhost only)
```

The entire frontend — HTML, CSS, JS, and an SVG charting micro-library — is
embedded in the binary. There is **no CDN and no build step**; it runs fully
offline.

### `--demo` vs. live-attach

**Demo** serves a self-contained, deterministic synthetic attack feed — nothing
else required:

```bash
heramba dashboard --demo --listen 127.0.0.1:7191
# open http://127.0.0.1:7191
```

In demo mode the control panel (mode / pause / clear / preset) is live *against
the demo feed only*.

**Live-attach** points the dashboard at a real edge's JSONL sink (file or pipe).
Attached this way it is a **read-only viewer**: the control panel is present but
every write is rejected with a plain reason — posture changes belong on the edge
(`heramba serve`), not in a detached viewer.

```bash
heramba dashboard events.jsonl                 # follow a live sink file
heramba serve --alert jsonl - | heramba dashboard -   # live over a pipe
```

### HTTP endpoints

Read side (GET):

| Path | Returns |
|---|---|
| `/` | the embedded SPA shell |
| `/api/events?n=` | recent events, newest first |
| `/api/metrics` | metrics snapshot (totals, by-kind, by-severity, score histogram) |
| `/api/timeline?window=` | bucketed counts over the trailing window |
| `/api/sources`, `/api/sources/{id}` | per-source aggregates and drill-down |
| `/api/breaches` | confirmed breaches (`canary_triggered`) |
| `/api/campaigns` | coordinated multi-IP swarms |
| `/api/decoys` | per-target decoy hit-map |
| `/api/stream` | **Server-Sent Events** live feed |
| `/api/export/events.csv`, `/api/export/events.json` | downloadable exports |
| `/healthz` | `ok` |

Every API response carries `Cache-Control: no-store`, `X-Content-Type-Options:
nosniff`, `X-Frame-Options: DENY`, a strict same-origin CSP, and
`Referrer-Policy: no-referrer`.

### SSE and exports in practice

```
$ curl -s --max-time 2 http://127.0.0.1:7191/api/stream
: connected

data: {"type":"metrics","metrics":{"mode":"protect","total":401,...}}
```

```
$ curl -s -D - -o /dev/null "http://127.0.0.1:7191/api/export/events.csv?n=3"
Content-Disposition: attachment; filename="heramba-events-20261122-161452.csv"
Content-Type: text/csv; charset=utf-8
X-Content-Type-Options: nosniff
```

## LIVE controls via `serve --dashboard-listen`

To change posture from the browser, mount the dashboard *on the edge itself*.
`heramba serve --dashboard-listen` wires the web console to the running engine,
so mode and decoy-preset changes **hot-swap the live detection surface** — the
same atomic reload a SIGHUP uses, with no dropped connections.

```bash
# in front of examples/sample-app, with a live dashboard on 7193
heramba serve \
  --config examples/.heramba.yml \
  --listen 127.0.0.1:8090 \
  --upstream http://127.0.0.1:3000 \
  --dashboard-listen 127.0.0.1:7193 \
  --dashboard-token s3cret
```

```
heramba edge — mode=observe listen=127.0.0.1:8090 (http) upstream=http://127.0.0.1:3000
  OBSERVE mode: detection only, real traffic passes through untouched.
  dashboard: web console on http://127.0.0.1:7193 — LIVE controls (bearer-token protected)
```

The relevant `serve` flags:

```
  -dashboard-listen string   web dashboard address, e.g. 127.0.0.1:7171 (empty=disabled)
  -dashboard-token string    bearer token required to use the dashboard's live controls
                             (empty=open; bind localhost)
```

### What the live controls can and cannot do

- `SetMode` — flip **observe ⇄ protect** live.
- `ApplyDecoyPreset` — swap the active framework decoy preset live.
- **Pause is deliberately unsupported** on a live edge: a production edge must
  not expose a "stop defending" button. Switch to observe mode to stop inline
  action.
- **Clear is unsupported**: session counters are the running audit tally and are
  not resettable from the UI.

### The bearer-token guard

`--dashboard-token` guards the `POST /api/control/*` write endpoints. With no
token the writes are open — intended only for a localhost bind. **When exposing
the dashboard beyond loopback, always set a token.** The read endpoints stay
open either way; only control is gated.

```
# without the token — rejected
$ curl -s -o /dev/null -w "%{http_code}\n" -X POST -d '{"mode":"protect"}' \
    http://127.0.0.1:7193/api/control/mode
401

# with the token — accepted, flips the live edge to protect
$ curl -s -X POST -H "Authorization: Bearer s3cret" -d '{"mode":"protect"}' \
    http://127.0.0.1:7193/api/control/mode
{"ok":true,"paused":false}
```

An annotated config for running a dashboard-backed edge is in
[`examples/dashboard`](../examples/dashboard).
