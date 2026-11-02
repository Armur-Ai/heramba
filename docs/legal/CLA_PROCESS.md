# CLA Process (public-facing description)

> **STATUS: DRAFT — process NOT yet active.** Scaffolded 2026-10-21. Operational
> detail lives in the private `legal/CLA_OPERATIONS.md`. Not legal advice.

## Individual signing

- Proposed mechanism: the separately hosted **CLA Assistant service**
  (https://cla-assistant.io/), **subject to a sandbox evaluation** of the exact
  fields and privacy controls it captures/exports.
- **Do NOT** install `contributor-assistant/github-action` — that repository was
  **archived on 2026-03-23**. Hosted CLA Assistant is configured in the service,
  not by adding a `.github/cla.yml` file.

## What must be recorded for each signer

Exact accepted agreement text + SHA-256; agreement version; effective date;
authenticated identity; stable GitHub ID + current handle; assent timestamp;
named project (`heramba`); elected current PR; and, where relevant, private
legal name/contact (kept **out** of the public repo). A copy is delivered to the
signer.

## Corporate grants

Use a separate approved e-signature process; then record the corporate
authorization against the covered contributors and scope. A green individual
check is not sufficient for employer-owned work.

## Merge gating (prepare now, activate later)

- Require rights clearance for **all** material authors of a PR (including
  co-authors; account for squash merges and authorship changes).
- Narrow owner/bot exceptions require documented grounds — org membership alone
  is not ownership.
- Required CI checks before a protected-branch merge. **Activate enforcement only
  once the forms are approved and failure behavior is tested.**

## Test matrix (fixtures only — never create fake live assent)

Unsigned author · valid current signer · two-author PR · employer-owned work with
missing corporate grant · changed agreement version · historical-only signer ·
service unavailable.
