# Agentic-traffic classifier

Heramba scores how automated/agentic each request source looks. The classifier
is **deterministic and explainable** — it never calls an LLM. The detection path
must be fast, offline, reproducible, and immune to prompt-injection, so the core
is plain signal logic. (An AI-augmented classifier is a separate commercial
plugin that layers on top; it never becomes the critical path.)

## Signals

Each signal has a weight; a source's score is the sum of the signals that fired,
capped at 1.0. Every verdict lists exactly which signals fired and why.

| Signal | Weight | Fires when |
|---|---|---|
| `tool_user_agent` | 0.40 | UA is empty or a known non-browser client (curl, python-requests, go-http-client, nuclei, sqlmap, …) |
| `missing_browser_signals` | 0.25 | no `Sec-Fetch-*`, no `Accept-Language`, and UA isn't a browser |
| `non_browser_accept` | 0.15 | no `text/html` in Accept and no `Accept-Encoding` |
| `high_cadence` | 0.25 | sustained request rate above the human threshold (>5 rps) |
| `enumeration` | 0.20 | many distinct paths from one source in a short window |

## Labels

| Score | Label |
|---|---|
| ≥ 0.70 | `agentic-suspected` |
| ≥ 0.40 | `tool` |
| < 0.40 | `human` |

A browser with normal headers scores ~0 → `human`. A `curl` one-shot scores 0.4
→ `tool`. A `python-requests` client sweeping distinct paths at 10 rps fires all
four signals → `agentic-suspected`, which emits an `agentic_traffic` event (and,
in protect mode, tarpits the source).

## Configuring thresholds

All thresholds are configurable in `.heramba.yml` (zero values keep the
conservative defaults):

```yaml
classifier:
  cadence_rps: 5.0      # high_cadence fires above this req/s
  enum_distinct: 12     # enumeration fires at/above this many distinct paths
  tool_at: 0.4          # score >= this => "tool"
  agentic_at: 0.7       # score >= this => "agentic-suspected"
```

## Tuning & honesty

Thresholds are deliberately conservative defaults. This is a **behavioral
heuristic**, not identity: a flagged source is *suspected* automation, stated as
such. The value is that the signals are transparent — you can see precisely why
a source was flagged and decide what to do with it. Adding more signals (TLS/JA3 fingerprint, header-order
fingerprint, request-timing variance) is on the roadmap.
