# Operations, explainability & human control (R9)

The R9 gate is operator-centric: *can an operator explain every consequential
action, undo it, and audit its evidence?* Heramba answers yes through an
investigation timeline that links the whole chain and an exportable incident
report.

Code: [`internal/incident`](../../internal/incident). Alert integration:
[`internal/alert/webhook.go`](../../internal/alert/webhook.go).

## The investigation timeline

`incident.Builder` composes the existing typed records — `evidence.SecurityEvent`
→ `evidence.Fact` → `evidence.Hypothesis` → `strategy.ActionExecution` — into one
chronological chain of six stages:

```
observation → fact → hypothesis → proposed_action → approved_action → outcome
```

Every entry carries what an operator needs to judge it:

- **observed vs inferred** — observations and facts are `observed`; hypotheses
  and actions are `inferred`/decisions, marked as such;
- **confidence and uncertainty** on each hypothesis, plus the **count of
  contradicting evidence** (what argues *against* the inference);
- **source health** on observations (sensor liveness);
- **causal notes** and standing **caveats** ("correlation is not causation"; "a
  multi-source actor link is an identity inference to corroborate before
  acting").

`Incident.Explain(actionID)` returns the causal chain for a single action — the
action's entries plus the evidence/hypothesis entries it derived from — so an
operator can trace any action back to its ground truth.

## Human control, proven by the gates

Every containment is **scoped, time-limited, auditable, and reversible** — and
the incident report checks it:

- `AllConsequentialReversible()` — every *enforced* action must carry a rollback
  path (a rollback token, or already expired/rolled-back). Tested: an enforced
  action without a rollback token fails the gate.
- `AllConsequentialHaveEvidence()` — every state-changing action must cite
  evidence. Tested: an action with no evidence fails the gate.
- **Blast radius** is estimated per action: a bounded scope (exactly one
  source/session/decoy — nothing global) and whether the impact can touch
  legitimate users.
- The underlying engine already provides **shadow mode (default)**, **approval
  gates** for medium+ impact, **expiry/auto-rollback**, and an **emergency
  disable** kill-switch ([strategy](../strategy.md)); the timeline surfaces the
  approval and outcome stages for each action.

## Retention & export

- `RetentionPolicy{MaxAge}` with `Prune(now)` drops entries past the window; the
  policy is recorded in the report. Retention windows are a deployment choice.
- `Incident.WriteMarkdown` / `WriteJSON` export a complete operator report:
  the timeline, per-action blast radius and reversibility, the human-control
  gates, retention, and caveats.
- Campaign **merge/split correction** already exists at the campaign layer
  ([campaign intelligence](campaign.md)); the timeline consumes the corrected
  hypotheses.

## Alert integration (functional and tested — not fabricated)

Per the directive, integrations are added only when they actually work. Heramba
ships text and JSONL sinks, and now a **webhook sink** (`alert.NewWebhook`) that
POSTs each event as JSON above a severity floor. It is verified against a live
`httptest` server (delivery, severity filtering, and error-on-non-2xx) — not a
stub. No Slack/SIEM connector is claimed; those would each need the same
functional test before being added.
