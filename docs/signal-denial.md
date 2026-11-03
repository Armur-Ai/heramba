# Signal denial

Attackers — and the oracles autonomous agents rely on — need **clean ground
truth**: a version banner to fingerprint, a timing difference to distinguish a
valid user from an invalid one, a distinctive error to confirm an injection.
Signal denial takes that away. Two mechanisms ship in the engine (protect mode).

## Response-header normalization

The edge strips headers that leak stack/framework/version and presents a generic
banner:

- removed: `Server`, `X-Powered-By`, `X-AspNet-Version`, `X-AspNetMvc-Version`,
  `X-Runtime`, `X-Version`, `X-Generator`, `X-Drupal-Cache`, `X-Backend-Server`
- `Server` is set to a neutral `heramba`

Fingerprinting tooling that keys on these now gets nothing useful.

## Timing jitter

Bounded random latency is added so timing-differential oracles lose their
signal. Blind SQLi and user-enumeration-by-response-time depend on a *stable*
timing delta; jitter in a configured `[min, max]` window drowns small deltas in
noise without materially harming UX. (The jitter amount is currently a built-in
protect-mode tarpit for flagged sources; exposing it as explicit config is on
the roadmap.)

## What this is and isn't

- It **is** a cheap, broad reduction of the attacker's confidence and the
  automated oracle's reliability.
- It **is not** a substitute for fixing the underlying bug. Normalizing a header
  doesn't patch the vulnerable endpoint; it denies the *reconnaissance* signal.
- Content-level response shaping (rewriting error bodies, synthesizing decoy
  responses, per-endpoint response-normalization recipes) is a commercial
  plugin — the engine ships the conservative, deterministic header/timing layer.

## Safety

All of this is **protect-mode only**. Observe mode never touches responses, so
you can run Heramba in front of production and measure before you enable any
inline behavior.
