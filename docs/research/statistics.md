# Scientific statistics & targets (R8)

The directive's performance numbers are **aspirational evaluation thresholds**,
never claims. R8 makes the gap between *target* and *observed* explicit and
reproducible.

```bash
heramba research stats                               # target-vs-observed scorecard
heramba research stats --json
heramba research stats --artifacts docs/research/artifacts/scorecard
```

Code: [`internal/research/stats`](../../internal/research/stats) (the math) and
[`internal/research/scorecard`](../../internal/research/scorecard) (the
reconciliation, which reruns the three experiments and scores them). The
scorecard reproduces from the same seeds the experiments use, so re-running
regenerates the same table (bootstrap/throughput aside).

## Statistical methods

- **Wilson score intervals** for every proportion (recall, FPR, completion) —
  correct at the low base rates and small samples security evaluation lives at
  (tested on 0/n, n/n, and mid cases).
- **Nonparametric bootstrap** (percentile 95%) for mean metrics and for **effect
  sizes** (matched differences between arms).
- **PR-AUC** as the threshold-free comparison; **per-scenario confusion
  matrices** reported stratified, never only pooled.
- **Precision at realistic prevalence** — pooled precision on a balanced
  synthetic set overstates field precision, so the scorecard recomputes it at a
  1% attack prevalence.
- **Sample-size adequacy** — `RequiredNForProportionCI` reports how many benign
  sessions a 1% FPR target needs for a ±1% interval, and the scorecard flags the
  FPR row as **underpowered** when the evaluated n falls short.

## Reference scorecard (revision `bfc8f36`)

| metric | target | observed [95% CI] | met? |
|---|---|---|---|
| recall (held-out coordinated) | ≥ 0.90 | 1.000 [0.839, 1.000] (n=20) | ✅ |
| false-positive rate | ≤ 0.01 | 0.094 [0.032, 0.242] (n=32) | ❌ **MISS** — and **underpowered** (n=32 ≪ ~381) |
| objective-completion reduction (adaptive vs unprotected) | ≥ 0.50 | 1.000 (n=500) | ✅ |
| policy-safe response latency (response component) | ≤ 5s | 4.0s (n=800) | ✅ |
| valid-artifact run rate | ≥ 0.95 | 1.000 (n=3) | ✅ |

**Effect size** (matched seeds, bootstrap 95%): adaptive deception prevents
**0.884 [0.856, 0.912]** more objective completions than static decoys.

**Precision at realistic 1% prevalence**: with recall 1.00 and FPR 0.094,
precision is **0.097** — a direct, honest illustration that a 9.4% FPR is far
too high for rare-event detection, and exactly why the FPR row is a MISS.

## Honesty rules enforced

- **A MISS is reported as a MISS.** The FPR target is not met; the scorecard says
  so and quantifies why (the interval, the prevalence consequence, the
  underpowered n). The data is never altered to hit a target.
- **No missing observation is substituted with zero**; absent data widens the
  interval instead.
- **No aggregate success without stratification** — per-family confusion matrices
  accompany the pooled numbers (R2).
- **Observed ≠ target** everywhere — the scorecard's whole purpose.

## Immutable raw data & reproduction

The reference artifacts under [`artifacts/`](artifacts/) (coord, objective,
coevolve, scorecard) are the committed raw outputs. Re-running the commands above
regenerates them; CI’s determinism tests prove the metric fields are
byte-identical across runs. The analysis is the code — there is no hand-edited
spreadsheet between raw measurements and the reported tables.
