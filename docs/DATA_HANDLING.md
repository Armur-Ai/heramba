# Data Handling (community engine)

> **STATUS: DRAFT (2026-10-21).** Describes intended data behavior of the
> self-hostable engine. Avoids universal claims ("no data ever leaves the
> machine", "perfect attribution", "unbypassable safety"). Not legal advice.

## Community (self-hosted) operation

- Runs on **infrastructure the operator controls**; event data stays local to the
  operator's deployment by default.
- Sensitive fields (authorization/cookie headers, keys, raw credentials, sensitive
  query values) should be stripped/redacted at the edge before storage or before
  any optional model submission. **Request bodies are not collected by default.**
- Debug/body collection, if enabled, is explicitly bounded and access-controlled.

## If connected to the managed cloud (optional)

- Connecting to the commercial cloud is **opt-in** and introduces separate
  processing described in the commercial Privacy Policy / DPA (private repo).
- Cross-customer raw payload sharing is **off**; fleet intelligence is built only
  from minimized, reviewed indicators or demonstrably non-identifying aggregates.

## Not promised

- That hashing/HMAC/truncating IPs equals anonymization (identifiability depends
  on linkage).
- That any attacker was conclusively identified.
- That models are trained on your traffic — training is a **separate**,
  reviewed purpose and is **off by default**.

See the private `legal/DATA_MAP.md` and `legal/TELEMETRY_ASSESSMENT.md` for the
field-level analysis of the managed service.
