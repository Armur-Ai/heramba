# internal/poison

Intelligence poisoning for **confirmed-bad** sources. Feed a confirmed attacker
believable-but-false data so whatever they exfiltrate from our honeypot is
worthless and self-incriminating. Every poisoned value carries an embedded,
keyed watermark: if it ever reappears — a later request, a paste site, another
victim's logs — we know it came from us and that it is fake.

Pure defense: this only produces fake data served **from** our honeypot. It
attacks no one. Generated secrets are **inert** — never a real vendor credential
format that could be mistaken for a live key — and always watermarked.

## Watermark (`poison.go`)

```go
m, _ := poison.NewMarker(key []byte)   // or poison.NewRandomMarker()
w := m.Watermark("tok_abc")            // "tok_abc.hrmbwm1<authenticated tag>"
ok := m.IsOurs(w)                      // true: recognizably ours, tamper-evident
orig, ok := m.Strip(w)                 // recover the original value
```

A fixed human-recognizable sentinel (`hrmbwm1`) makes a value obvious in logs;
an HMAC-SHA256 tag makes it unforgeable and tamper-evident (altering the body
invalidates it). `Watermark` is idempotent. `ErrNoKey` on an empty key.

## Generator (`generate.go`)

```go
g := poison.NewGenerator(seed, m)
keys := g.APIKeys(8)        // []Secret{Name, Kind, Value, Synthetic}
users := g.Users(8)         // []User{ID, Username, Email, PasswordHash, APIKey, Synthetic}
sql := g.UsersSQLDump(20)   // fake INSERT dump (classic "exfil" artifact)
env := g.ConfigEnv()        // fake .env blob of watermarked secrets
vals := g.AllValues(5, 5)   // sorted watermarked values — register as canaries
```

Output is **deterministic** from `seed` (same seed → byte-identical) and each
dataset uses an independent sub-stream, so generating users never perturbs the
key stream. Safety rails baked in:

- Secret values use a generic `tok_`/`pw_` form — never `AKIA`, `sk_live_`,
  `ghp_`, `AIza`, bcrypt `$2a$`, etc.
- Password hashes use an inert `$fake$` scheme, never a real hash format.
- Emails use the RFC 2606 reserved `.test` TLD; records are tagged `Synthetic`.
- Every secret value is watermarked (`IsOurs` is true for all of them).

## Mounting at the edge

1. Build one process-wide `Marker` from a configured secret and a `Generator`.
2. For a confirmed-bad source, serve `g.UsersSQLDump(n)` / `g.ConfigEnv()` /
   `g.APIKeys(n)` in place of (or injected into) real-looking responses — pair
   with `internal/tarpit` to drip them out slowly.
3. Register `g.AllValues(...)` as canaries (`internal/decoy`) so if a poisoned
   value ever comes back to us, it fires a `KindCanaryTriggered` event — and
   `m.IsOurs(v)` confirms provenance anywhere the value turns up.
