# Research protocol

Heramba's Phase 2 claims are evaluated against a **pre-registered protocol** that
is code, not prose: [`internal/research`](../../internal/research). The manifest
is the single source of truth — regenerate it any time with:

```bash
heramba research manifest            # validated JSON to stdout
heramba research manifest --out docs/research/protocol.manifest.json
```

`research.Protocol.Validate()` rejects a protocol that is internally
inconsistent, and the default protocol is asserted valid in CI
(`go test ./internal/research/...`). This means the experiments cannot run
against a protocol with duplicate hypotheses, a non-falsifiable hypothesis, an
endpoint nothing reports, or **train/test seed leakage**.

## Pre-registered hypotheses

Every hypothesis carries an explicit **null** (so it is falsifiable), a
prediction, and the primary endpoint that decides it. Full statements live in
`research.DefaultProtocol()`.

| ID | Claim (abbreviated) | Null | Primary endpoint |
|---|---|---|---|
| **H1** | Coordination is distinguishable from independent scanning / legit automation without IP or UA | coordination features add nothing over a rate-only heuristic | recall (@ matched FPR) |
| **H2** | Adaptive deception reduces attacker objective completion vs. weaker arms | no reduction vs. predeclared baseline | objective-completion rate |
| **H3** | The defender adapts to a bounded strategy shift fast enough to keep positive utility | adaptation latency unbounded or utility non-positive | adaptation latency |
| **H4** | Detection degrades gracefully with a missing sensor | any missing sensor collapses recall to the rate-only baseline | recall |
| **H5** | Safety holds, overhead bounded: attacker text never changes policy | attacker input can alter a decision, or overhead exceeds target | benign disruption |

## Endpoints

Primary (decision-driving): recall, false-positive rate, objective-completion
rate, adaptation latency, benign disruption. Secondary (reported, not
decision-driving): precision, PR-AUC, detection latency, time-to-objective,
diversion fraction, defender utility, throughput, deception-discovery rate.

## Data partitions and leakage prevention

Four partitions, strictly separated: **train → tune → validation → test**. The
held-out **test** families use a disjoint seed range (9000+) and include a
`deception_aware_swarm` family that is **never tuned on**.
`Protocol.Validate()` fails if any test seed also appears in a tuning family.

Additional leakage rules (enforced by construction and documented in the
manifest's `leakage_prevention`):

- no feature derived from IP octets, user-agent strings, or simulator-specific
  constants;
- the scoring labels (`IsAttacker`/`IsGoal`) are never visible to any detector;
- fixed seeds and sample sizes chosen before running — no peek-then-extend.

## Statistical methods

Wilson score intervals for proportions, nonparametric bootstrap for mean metrics
and their differences, per-scenario confusion matrices with stratified
reporting, and effect sizes alongside every point estimate. See
[statistics](statistics.md) (R8).

## Coordination vs. attribution

Heramba detects **suspicious coordination** from behavioral features. It does
**not** attribute traffic to "an LLM" or "an AI." The protocol treats those as
different questions: coordination is measurable here; LLM attribution is not
claimed from traffic alone.

## Threats to validity (declared up front)

- Synthetic traffic is more separable than production; reported numbers are
  harness measurements, **not** field performance claims.
- The deception-aware adversary is a modeled heuristic, not a live LLM agent.
- The deterministic harness cannot honestly measure CPU/memory/recovery; those
  are marked *not measured* rather than invented.

See the [threat model](threat-model.md) for the actor families and budgets.
