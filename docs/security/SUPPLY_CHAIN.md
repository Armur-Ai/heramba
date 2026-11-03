# Supply-chain & release assurance (R7)

What is actually enforced in this repo, and the honest limits. No certification
or "military-grade" claim is made — only what CI runs and what the artifacts
prove.

## CI checks (`.github/workflows/ci.yml`)

Every push/PR runs three jobs:

1. **build-test** — `gofmt` check, `go vet`, `go build`, and `go test -race` on
   Go 1.25.x and 1.27.x.
2. **fuzz-smoke** — `make fuzz FUZZTIME=20s` runs every fuzz target (below) for a
   short time to catch panics and policy-invariant violations per PR.
3. **supply-chain** — `govulncheck ./...` (dependency vulnerability scan), a
   `go mod tidy` cleanliness gate (`git diff --exit-code go.mod go.sum`), an SBOM
   freshness check (regenerate and diff), and SBOM artifact upload.

## Fuzzing

`make fuzz` (and CI) fuzz the four untrusted-input surfaces the directive calls
out; each target asserts "never panics" plus a domain invariant:

| Surface | Target | Invariant |
|---|---|---|
| Parser (untrusted JSON) | `taint.FuzzDecodeObservation` | accepted input never trusts the free text / source key |
| Log sanitizer | `taint.FuzzSanitizeForLog` | output never contains CR/LF/ANSI/control runes |
| Decoy routing | `decoy.FuzzDecoyRouting` | arbitrary method/path never panics |
| Event ingestion | `evidence.FuzzStoreLoad` | corrupt/hostile JSONL validates-before-accept, soft-skips, never panics |
| Policy evaluation | `strategy.FuzzPolicyEvaluate` | unbounded scope / disallowed type is **never** allowed; allowed state-changing actions have bounded duration |

Reference local run: millions of executions per target, **zero crashes**
(`FuzzPolicyEvaluate` ~3.7M execs — the injection-proof policy invariants held
under all generated inputs).

## Dependencies

- **Minimal and pinned.** The module has exactly one runtime dependency,
  `gopkg.in/yaml.v3 v3.0.1`, pinned in `go.mod`/`go.sum`. The full dependency set
  is in the committed [SBOM](../../sbom.cdx.json) (CycloneDX 1.5).
- **Update policy.** Dependency bumps go through a PR; CI's `govulncheck` and
  `go mod tidy` gates must pass; the SBOM is regenerated (`make sbom`) in the same
  PR so the committed SBOM never drifts.

## SBOM & reproducible builds

- `make sbom` runs a self-contained generator (`tools/sbom`) over
  `go list -m -json all` and emits a **deterministic** CycloneDX SBOM (components
  sorted, no wall-clock timestamp unless `SOURCE_DATE_EPOCH` is set). Verify by
  regenerating and diffing — CI does exactly this.
- **Reproducible build guidance.** Build with a pinned Go toolchain and
  `-trimpath`, inject version metadata via `-ldflags` (see `Makefile`), and set
  `SOURCE_DATE_EPOCH` for any timestamped artifact. `CGO_ENABLED=0` yields a
  static binary. Signed release artifacts / provenance attestation are a
  release-time step (e.g. cosign + SLSA) and are **not yet wired** — tracked, not
  claimed.

## Secrets, access, retention, data minimization, safe defaults

- **Secrets handling.** No secrets in the repo; the user's email and any
  credentials are never logged or transmitted. Synthetic "secrets" in the
  deception world are structurally tagged and use reserved domains, so they can
  never be mistaken for or work as real secrets (R3 isolation tests).
- **Access control / least privilege.** Defensive actions are a closed, typed set
  behind a fail-closed policy and tool allowlist (R5); an LLM origin grants no
  execution authority.
- **Audit retention.** Every action lifecycle transition is audited
  (`strategy.Engine.Audit`); the evidence store is append-only JSONL with
  provenance. Retention windows are a deployment policy, documented per
  deployment.
- **Data minimization.** The classifier and campaign layers key on transport
  signals and behavior, not identities; an IP is a transport key, never an
  identity.
- **Safe defaults.** Observe mode (detection only) is the default; every inline
  effect is opt-in; the strategy engine defaults to shadow mode.

## Vulnerability disclosure

See [SECURITY.md](../../SECURITY.md) for the reporting channel, supported
versions, and coordinated-disclosure request. (That file has owner-provided
fields still marked DRAFT; publication requires the owner to fill them.)

## Explicitly NOT claimed

- No certification, accreditation, or "military-grade" validation.
- Signed releases / SLSA provenance not yet wired (release-time work).
- `govulncheck` covers known advisories for the Go toolchain and dependencies;
  it is not a guarantee of absence of vulnerabilities.
