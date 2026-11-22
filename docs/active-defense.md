# Active defense: the `heramba trap` endpoint

This is the guide to Heramba's active-defense plane — what it does, why it is
safe, and a hands-on walk-through of the `heramba trap` command with real,
copy-pasteable transcripts.

Everything here is **defensive**. Heramba only ever serves content from its own
endpoint and slows down an attacker's own requests *to that endpoint*. Nothing
is ever sent at a client, no third party is scanned, probed, or attacked, and
every fabricated secret is inert and watermarked. See
[ACTIVE_DEFENSE_BOUNDARIES.md](ACTIVE_DEFENSE_BOUNDARIES.md) for the product
policy that bounds all of this.

## The philosophy: deny ground truth, trap the autonomous attacker

An autonomous attacker — a crawler, a scanner, an LLM agent working a target —
depends on two things: **true signal** about what your app exposes, and **cheap,
fast requests** to enumerate it. Active defense attacks both:

1. **Deny ground truth.** Mix believable-but-false surface into what an attacker
   sees, so its map of your app is poisoned. It cannot tell a real endpoint from
   a planted one, or a real secret from an inert decoy.
2. **Trap the automation.** The instant something follows a link that *only* an
   automated crawler would ever request, that source is confirmed hostile — with
   no database and zero false positives — and every later request from it is
   sunk into a slow, endless, worthless maze.

A human browsing your site never sees any of this. A legitimate client never
requests a trip-URL, so it is never confirmed bad and never tarpitted.

## What `heramba trap` is

`heramba trap` is a standalone active-defense endpoint. It needs no upstream and
no config file — it is the three active-defense planes
([tripwire](../internal/tripwire/README.md),
[tarpit](../internal/tarpit/README.md),
[poison](../internal/poison/README.md)) wired together into one HTTP handler:

```
$ heramba trap --help
usage: heramba trap [flags]

A standalone active-defense endpoint: trip-wire bait in front, tarpit +
labyrinth + watermarked poison for anything that trips it.

flags:
  -alert string
    	alert format: text|jsonl (default "text")
  -base string
    	public base URL for bait links (default: http://<listen>)
  -delay duration
    	first-offense tarpit drip interval (default 250ms)
  -listen string
    	listen address (default "127.0.0.1:8080")
  -max-delay duration
    	cap on the tarpit drip interval (default 3s)
  -seed int
    	seed for deterministic bait tokens, labyrinth, and poison (default 1)
```

The signer key, the labyrinth, and the poison generator are all derived from
`--seed`, so a given seed produces a reproducible world across restarts.

## The bait it plants

Start one on a spare port:

```
$ heramba trap --listen 127.0.0.1:8099
heramba trap — active defense on http://127.0.0.1:8099 (base=http://127.0.0.1:8099)
  bait surfaces: /api/docs /assets/app.config.js /robots.txt /sitemap.xml
  trip one and every later request is tarpitted + poisoned. Ctrl-C to stop.
```

Four conventional paths serve bait directly, and two more kinds of bait ride
along on every ordinary response:

| Bait kind    | Where it appears                                        |
|--------------|---------------------------------------------------------|
| robots.txt   | `GET /robots.txt` — `Disallow:` lines pointing at traps |
| sitemap      | `GET /sitemap.xml` — a urlset of trip-URLs              |
| JS config    | `GET /assets/app.config.js` — a `window.__APP_CONFIG__` blob of fake "internal" endpoints |
| API docs     | `GET /api/docs` — a fake internal API-docs JSON         |
| HTML comment | dev-note `<!-- ... -->` comments injected into ordinary HTML pages before `</body>` |
| Link header  | a `Link:` response header (`rel="prefetch"` / `rel="preload"`) on ordinary pages |

Every planted URL carries an opaque token sealed with a keyed HMAC, so a hit is
fully attributable — which bait kind, which label, when planted — with **no
database**, and any forgery or tampering is detectable. The token is its own
clean path segment under a juicy-looking parent:

```
$ curl -s http://127.0.0.1:8099/robots.txt
User-agent: *
Disallow: /backup/cm9ib3Rz.cm9ib3RzOiBleHBvc2VkIGRhdGFiYXNlIGJhY2t1cA.1795363909909634000.sAHnB_C9mG3zgURcF6mbHhjes3rgOFQuC5c0j_C-VZE/db-2024.sql
Disallow: /admin/export/cm9ib3Rz.cm9ib3RzOiBhZG1pbiB1c2VyIGV4cG9ydA.1795363909909634000.KOQCt_67vy9MWVsarcTaPW2QK3Knx7K_Krxbow3rwyU/users.csv
Disallow: /internal/debug/cm9ib3Rz.cm9ib3RzOiBpbnRlcm5hbCBkZWJ1ZyBjb25zb2xl.1795363909909635000.mMyhgpxA4yEJGMifObA1ejJ94bP0ub_4RaZlqt9u9SQ
Disallow: /.git/cm9ib3Rz.cm9ib3RzOiBleHBvc2VkIFZDUyBkaXJlY3Rvcnk.1795363909909636000.SscC2W7cSnJO5wYZJMQQlI_i_xFcio30BMt-y5a2mgI/config
Disallow: /api/v1/admin/cm9ib3Rz.cm9ib3RzOiBhZG1pbiBBUEkga2V5cw.1795363909909636000.YUazfMDsPiJ3H4_FIax2Ud8Lwx30048kC5fYG6zXTS4/keys
```

An ordinary page carries the comment and header bait too — note the `Link:`
header and the HTML comments pointing at more traps:

```
$ curl -s -i http://127.0.0.1:8099/
HTTP/1.1 200 OK
Content-Type: text/html; charset=utf-8
Link: <http://127.0.0.1:8099/backup/bGlua19oZWFkZXI.../snapshot.tar.gz>; rel="prefetch", <http://127.0.0.1:8099/.aws/bGlua19oZWFkZXI.../credentials>; rel="preload"
...

<!doctype html><html><head><title>Welcome</title></head><body>
<h1>It works</h1>
<p>Nothing to see here.</p>
<!-- TODO: remove before launch — staging backup at /backup/aHRtbF9jb21tZW50.../nightly.tar.gz -->
<!-- legacy admin tool (kill after migration): /admin/tools/aHRtbF9jb21tZW50.../db-console.php -->
<!-- internal API spec, do not link publicly: /api/internal/aHRtbF9jb21tZW50.../swagger.json -->
</body></html>
```

> The token strings embed a plant timestamp, so the exact value changes every
> run (even at the same seed). Don't hardcode one — fetch the surface and copy a
> live line, as the transcript below does.

## The signed trip-URL: a zero-false-positive confirmation

A legitimate user never requests any of these paths. A crawler or a snooping
agent does. When a request arrives, the matcher pulls any `.`-delimited token
out of the path or query and **verifies its HMAC signature** against the
process key. There is no path list to match against and nothing to get out of
sync; either the token is one Heramba signed, or it is not.

A verified hit confirms the source as hostile and emits a confidence-1.0
`decoy_hit` event. Follow a trip-URL from the robots.txt above:

```
# copy a real Disallow line from robots.txt, then request it:
$ TRIP=$(curl -s http://127.0.0.1:8099/robots.txt | awk '/Disallow: \/api\/v1\/admin/{print $2}')
$ curl -s -o /dev/null "http://127.0.0.1:8099$TRIP"
```

The trap's alert log records the confirmation (text sink shown; `--alert jsonl`
for machine output):

```
[16:13:01] high decoy_hit src=127.0.0.1 target="/api/v1/admin/cm9ib3Rz.../keys" conf=1.00 \
    GET /api/v1/admin/cm9ib3Rz.../keys — tripwire (robots)
```

The same event as JSONL — note the `bait_kind`, `bait_label`, `token_where`,
and `planted_at` labels, all recovered from the token alone:

```json
{"time":"2026-11-22T16:13:01.750762Z","kind":"decoy_hit","severity":3,
 "severity_label":"high","source":"127.0.0.1",
 "target":"/api/v1/admin/cm9ib3Rz.../keys","confidence":1,
 "detail":"GET /api/v1/admin/cm9ib3Rz.../keys — tripwire (robots)",
 "labels":{"bait_kind":"robots","bait_label":"robots: admin API keys",
           "planted_at":"2026-11-22T16:13:00Z","token_where":"path","tripwire":"true"}}
```

Confidence is 1.0 because the confirmation is cryptographic, not heuristic.
There is no threshold to tune and no benign traffic that can trip it.

## What happens next: tarpit, labyrinth, and watermarked poison

Once a source is confirmed, **every** later request from it (keyed by client IP)
is sunk. The per-source delay escalates exponentially with each offense —
starting at `--delay` (default 250ms) and capped at `--max-delay` (default 3s) —
and the response body is dripped out one chunk at a time on that schedule. Each
drip honors the request context and spawns no goroutine, so a disconnecting
attacker leaks nothing, and any single response is hard-capped at 30 seconds.

**Secret-looking paths get watermarked poison.** A path that looks like it holds
credentials (`.env`, `secret`, `credential`, `config`, `.git`, `backup`,
`dump`, `.aws`, `id_rsa`) is answered with a fabricated, inert `.env` dripped
slowly. Here is a full capture (sped up for the transcript; it hits the 30s cap
in practice):

```
$ curl -s "http://127.0.0.1:8099/.env"
# application config (synthetic)
APP_ENV=production
LOG_LEVEL=info
DB_HOST=db.internal.example.test
DB_USER=app_ro
DB_PASSWORD=pw_ede5b13592dfefcc4031bc5e.hrmbwm1p2d3f67g4ceuk
PAYMENTS_SERVICE_TOKEN=tok_d09c1af47ef58589fa2c9390a2d69680e077bf88.hrmbwm1cde57vqoprd2g
INTERNAL_API_KEY=tok_2aa5dfe3b9b5d519a488a531fd4cd9c9836c549b.hrmbwm1gxnsgizgw2owq
ANALYTICS_WRITE_KEY=tok_e02bdc1aa949d7822e2abf...
```

Everything in it is safe by construction:

- Secret values use a generic `tok_`/`pw_` shape — never a real vendor format
  (`AKIA`, `sk_live_`, `ghp_`, `AIza`, …) that could be mistaken for a live key.
- Every value carries the `hrmbwm1` watermark sentinel plus an HMAC tag. If a
  poisoned value ever reappears — in a later request, a paste site, another
  victim's logs — `IsOurs()` proves it came from us and that it is fake.
- Hostnames use the reserved `.test` TLD; the whole blob is synthetic.

This poisons the attacker's loot: whatever it exfiltrates is worthless and
self-incriminating. The trap records the act as a `counter_deployed` event:

```json
{"kind":"counter_deployed","severity":2,"severity_label":"medium","source":"127.0.0.1",
 "target":"/.env","confidence":0,"detail":"served watermarked poison config to confirmed-bad source",
 "labels":{"plane":"poison"}}
```

**Everything else gets the labyrinth.** Any non-secret path from a confirmed
source is answered from an endless tree of believable, interlinked fake pages:

```
$ curl -s "http://127.0.0.1:8099/internal/reports"
<!doctype html><html><head><title>Archive 83f8</title></head><body>
<h1>Archive 83f8</h1>
<p>Internal document index. Select a ...
```

Each page is a pure function of its URL and the seed, so serving one is O(1) in
time and memory and no global state grows. Links descend to a bounded depth,
then loop back — URLs stay bounded while the maze stays endless. A crawler
follows it forever; Heramba never runs out of RAM.

## Why this is safe

| Concern | How the trap is bounded |
|---|---|
| False positives | A hit is a verified HMAC signature on bait only an automated client would request. Confidence is 1.0 by construction. |
| Attacking back | Nothing is ever sent to the client. The tarpit only slows the attacker's *own* request to *our* endpoint. |
| Real credentials leaking | Generated secrets are inert, never a real vendor format, and always watermarked. |
| Resource exhaustion (ours) | Labyrinth pages are O(1); drips spawn no goroutines, honor cancellation, and cap each response at 30s. |
| Impersonation | All content is served from the operator's own authorized endpoint; no third party is involved. |

## Flags worth knowing

- `--seed N` — makes bait tokens, the labyrinth, and the poison reproducible.
  Keep it stable across restarts so tokens minted before a restart still verify.
- `--base URL` — the public base URL embedded in bait links. Set this to the
  externally reachable URL when the trap sits behind a proxy or a different host.
- `--delay` / `--max-delay` — the first-offense drip interval and its escalation
  cap. Lower them to make a demo quick; the defaults waste the most attacker
  time per connection.
- `--alert text|jsonl` — human log vs. machine-readable events on stdout. Pipe
  the JSONL form straight into `heramba watch -` or `heramba dashboard -` (see
  [dashboards.md](dashboards.md)).

## Try it yourself

The runnable lab at [`labs/09-active-defense-trap`](../labs/09-active-defense-trap)
builds the binary, starts a trap, fires a benign request, trips a wire, and
shows the attacker getting tarpitted and poisoned — end to end, in one script.
