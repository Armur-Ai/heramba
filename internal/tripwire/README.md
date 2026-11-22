# internal/tripwire

Signed trip-URL tokens and bait surfaces. A tripwire is a planted link or path
that **only** an automated crawler or a snooping attacker would ever request — a
legitimate user never sees it. Every planted URL carries an opaque, self-
describing token sealed with a keyed HMAC, so a hit is fully attributable (which
bait, when planted) with **no database**, and forgery/tampering is detectable.

Everything here is pure defense: surfaces are served **from** our honeypot and a
tripwire only *tells us* that bait was touched. Nothing contacts the client.

## Token scheme (`token.go`)

```go
s, _ := tripwire.NewSigner(key []byte)   // or tripwire.NewRandomSigner()
tok := s.Mint(tripwire.KindRobots, "/backup/")   // opaque, URL-safe string
claims, err := s.Verify(tok)                      // authenticates signature
claims, err := s.VerifyFresh(tok, 24*time.Hour)   // + expiry window
```

- `Signer` holds only the secret key; it keeps no per-token state and is safe for
  concurrent use. Keep the key stable across restarts so tokens stay verifiable.
- `Claims{Kind, Label, Planted}` — what the token sealed. `Verify` checks the
  HMAC only; `VerifyFresh(tok, maxAge)` also rejects tokens older than `maxAge`
  (`maxAge <= 0` = no expiry). Errors: `ErrMalformed`, `ErrBadSignature`,
  `ErrExpired`, `ErrNoKey`.
- Tokens are `base64url(kind).base64url(label).plantedUnixNano.base64url(mac)`
  — exactly three `.` separators and no `/`, so they drop into a path segment or
  query value untouched.

Bait `Kind`s: `KindRobots`, `KindSitemap`, `KindHTMLComment`, `KindLinkHeader`,
`KindJSConfig`, `KindAPIDoc`, `KindManual`.

## Bait renderers (`bait.go`)

```go
p := tripwire.NewPlanter(s, "https://shop.example.com") // base may be ""
b := p.RobotsTxt()   // Bait{Body, ContentType, Paths}
```

`Planter` mints a fresh token per planted URL and embeds it as its own clean path
segment under a juicy-looking parent (`/backup/<token>/db-2024.sql`). Renderers,
each returning a `Bait{Body, ContentType, Paths}`:

| method | surface | serve as |
|---|---|---|
| `RobotsTxt()`    | robots.txt with `Disallow:` trip paths | `text/plain` |
| `SitemapXML()`   | sitemaps.org urlset of trip URLs       | `application/xml` |
| `HTMLComments()` | dev-note HTML comments (inject pre-`</body>`) | inline |
| `LinkHeader()`   | `Link:` header value (prefetch/preload)| header value |
| `JSConfig()`     | `window.__APP_CONFIG__` internalEndpoints blob | `application/javascript` |
| `APIDocs()`      | fake internal API-docs JSON            | `application/json` |

`AllPaths()` returns the sorted union of every planted path (handy to also seed a
`decoy`/canary registry).

## Matcher (`matcher.go`)

```go
m := tripwire.NewMatcher(s)                    // or .WithMaxAge(24*time.Hour)
hit, ok := m.MatchRequest(r)                    // Hit{Claims, Token, Where}
ev, ok := m.InspectRequest(r, clientIP)         // event.Deception(KindDecoyHit)
```

The matcher verifies tokens cryptographically from any path segment or query
value (cheap 3-dot pre-filter before the HMAC), so it needs no list of planted
paths. `Inspect`/`InspectRequest` emit a confidence-1.0 `event.KindDecoyHit`
labeled `tripwire`, `bait_kind`, `bait_label`, `token_where`, `planted_at`.

## Mounting at the edge

1. Build one process-wide `Signer` from a configured secret (or `NewRandomSigner`
   for ephemeral bait), a `Planter`, and a `Matcher`.
2. Serve the bait: route `GET /robots.txt`, `/sitemap.xml`, a JS-config path and
   a fake API-docs path to the matching `Planter` renderer; inject
   `HTMLComments().Body` before `</body>` and add `LinkHeader().Body` as a
   `Link:` header on ordinary responses.
3. On every inbound request call `m.InspectRequest(r, clientIP)`; on `ok`, emit
   the event and escalate the source (e.g. hand it to `internal/tarpit`).
