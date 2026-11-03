# Metrics & alerting

Heramba's detections flow through an **alert sink**. The engine ships
deterministic local sinks; richer destinations (Slack/PagerDuty/SIEM) are
commercial plugins that attach via the same `Sink` interface.

## Alert output

Configure in `.heramba.yml`:

```yaml
alert:
  format: jsonl        # text | jsonl
  file: /var/log/heramba.jsonl   # empty = stdout
```

- **text** — a compact human line per detection.
- **jsonl** — one JSON object per line, for ingestion (each is an `event.Event`:
  time, kind, severity, source, target, confidence, labels).

Event kinds: `decoy_hit`, `canary_triggered`, `agentic_traffic`,
`egress_blocked`, `rate_limited`, `probe`. Deception kinds (`decoy_hit`,
`canary_triggered`) always carry `confidence: 1.0` — zero false positives by
construction.

## Session detection summary (shipped)

`heramba serve` wraps the configured sink in a counting sink and prints a
per-kind summary on shutdown:

```
heramba: 3 detection(s) this session:
  decoy_hit          2
  canary_triggered   1
```

A session with no detections says so explicitly (an empty result is stated, not
implied).

## Scope & honesty

- Counts are **in-process, per-session** — they reset on restart. Durable,
  cross-instance aggregation and dashboards are out of scope for the OSS engine
  (that's the managed/cloud tier).
- Observe mode emits the same detections as protect mode; only the inline
  *actions* differ. So you can measure what protect *would* do before enabling
  it.
