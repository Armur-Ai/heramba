# Decoys & canary tokens

Deception is Heramba's sharpest signal. A legitimate user never requests a decoy
path or uses a canary token, so **any hit is an attacker** — the detection
carries confidence `1.0`, and there is nothing to tune and no false-positive
rate to manage.

## Decoy endpoints

The edge serves a set of plausible lure paths that scanners and autonomous
agents routinely probe:

```bash
heramba decoys list
```

Defaults include `/.env`, `/.git/config`, `/.aws/credentials`, `/admin`,
`/api/internal`, `/actuator/env`, and more. A request to any of them emits a
`decoy_hit` event and returns a neutral `404` — indistinguishable from a path
that simply doesn't exist, so the attacker can't trivially tell they tripped a
sensor.

Add your own in `.heramba.yml`:

```yaml
decoys:
  enabled: true
  paths:
    - /internal/metrics-v2
    - /api/v0/debug
```

> The OSS engine **detects** decoy hits and serves a neutral response. Serving
> *believable fake content* to keep an attacker engaged (fake secrets, decoy
> records, stack-mimicking responses) is a commercial plugin — the engine stays
> honest about what it ships.

## Canary tokens

A canary is an unguessable token you plant where only an attacker would find it
— a fake credential in a config file, a decoy API key, a hidden form field.
Using it anywhere is a confirmed intrusion.

```bash
heramba decoys canary db-password
# canary token (label "db-password"):
#   hrmb_3a0ea0333739e3ff693757cd84aae412
```

Tokens are 128-bit cryptographically random and prefixed `hrmb_` so a used one
is unambiguously yours.

Tell the edge which tokens to watch for, and it emits a zero-FP
`canary_triggered` event when one appears in a request (and denies the request
in protect mode):

```yaml
decoys:
  canaries:
    - hrmb_3a0ea0333739e3ff693757cd84aae412
```

The edge scans the request **path, query, and header values** (so a fake API key
used in an `Authorization` header or a `?key=` param is caught) **and a
bounded prefix of the request body** (so a token planted in a POST/JSON field is
caught too), restoring the body so it still reaches your app.

## Why this beats signatures

A WAF matches known-bad payloads and drifts into false positives; deception
inverts the problem. You don't describe the attack — you plant ground that only
an attacker steps on. It's especially effective against **autonomous agents**,
which enumerate surface aggressively and walk straight into decoys.
