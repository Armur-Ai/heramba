# Lab 09 — Active defense, live

A **runnable, live** lab: it starts a real `heramba trap` endpoint, fires a
benign request that passes straight through, then follows a signed trip-URL and
shows the attacker getting tarpitted and fed watermarked poison.

Unlike labs 01–08 (which are Go tests that grade your understanding against the
engine packages), this one is a self-contained shell demo you watch happen. It
is the practical companion to [docs/active-defense.md](../../docs/active-defense.md).

## Run it

```bash
sh run.sh
# or choose the port:
PORT=9000 sh run.sh
```

The script needs only a Go toolchain and `curl`. It builds `bin/heramba`
itself, so there is nothing to install first. It is deterministic (`--seed 1`)
and cleans up the trap process on exit, so re-running it is the reset — there is
no state to clean up.

## What you'll see

1. **The bait.** `robots.txt` advertises `Disallow:` trip-URLs that no human
   would ever request. Each URL carries an HMAC-signed, self-describing token.
2. **A benign request passes through.** `GET /` returns an innocuous page
   instantly — it never touched a trip-URL, so the source is never confirmed.
3. **An attacker trips a wire.** Following one `Disallow:` line verifies the
   token cryptographically and confirms that source as hostile — a
   confidence-1.0 `decoy_hit`, zero false positives.
4. **The source is sunk.** From then on, a secret-looking path (`/.env`) drips
   out **watermarked, inert poison** and any other path (`/internal/reports`)
   serves the **endless labyrinth** of fake pages — both on an escalating tarpit
   drip that the demo cuts off after a few seconds.
5. **The record.** The `decoy_hit` and `counter_deployed` events the trap
   emitted are printed at the end.

## Why it's safe

Nothing is ever sent at the client. The tarpit only slows the attacker's **own**
request to the trap's own endpoint; the fake secrets are inert (never a real
vendor key format) and watermarked so a leaked value is always traceable back to
Heramba and provably fake. The boundaries are spelled out in
[docs/ACTIVE_DEFENSE_BOUNDARIES.md](../../docs/ACTIVE_DEFENSE_BOUNDARIES.md).

## Knobs

- `PORT=<n>` — which localhost port the trap binds (default `8099`).
- The script uses short tarpit intervals (`--delay 120ms --max-delay 400ms`) to
  keep the demo quick. Production defaults (`250ms`..`3s`) waste far more
  attacker wall-clock per connection.
- Add `--alert jsonl` to the `heramba trap` line in `run.sh` to emit
  machine-readable events you can pipe into `heramba watch -` or
  `heramba dashboard -` (see [docs/dashboards.md](../../docs/dashboards.md)).
