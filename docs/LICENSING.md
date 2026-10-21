# Licensing

> **STATUS: DRAFT — proposed, NOT yet adopted.** Scaffolded 2026-10-21 per the
> Armur legal-setup brief. Not legal advice; pending counsel review + founder
> approval of the FSL two-year conversion and the source-available positioning.

## Proposed model

- The community engine is licensed under the **Functional Source License,
  Version 1.1, ALv2 Future License** (`SPDX: FSL-1.1-ALv2`). The canonical text
  is in `../LICENSE` (generated verbatim from the steward template with only the
  year and licensor-name fields filled; see `legal/proposed/` and the private
  release manifest for provenance hashes).
- **FSL is source-available, not OSI-approved open source.** Do not describe it
  as "open source" without qualification and do not display an OSI badge.
- **Future license:** each publicly released version additionally becomes
  available under **Apache-2.0 on the second anniversary** of that version's
  first public availability under FSL. The reference text is in
  `../LICENSES/Apache-2.0.txt`.
- **Important:** the presence of `LICENSES/Apache-2.0.txt` does **not**
  independently grant an immediate Apache license to the current engine. It is
  the *future* license that attaches per-version on its conversion date, tracked
  in `docs/legal/PUBLIC_RELEASES.json`.

## What this does and does not restrict

- Permitted: internal use, protecting your own public-facing application,
  forking, and (subject to the FSL text) qualifying professional services.
- Restricted: a **Competing Use** — making the software available to others in a
  commercial product/service that substitutes for the engine or for a product we
  offer using it. Read the `LICENSE` text; this summary is not the license.
- The restriction is **not permanent per version**: it lifts when that version
  converts to Apache-2.0.

## Founder decision required before publication

Adopt FSL (accept the 2-year conversion **and** the source-available label), OR,
if immediate OSI-approved open source matters more than restricting hosted
competitors, adopt **AGPL-3.0-only + separate commercial grants** instead. These
goals are not simultaneously satisfiable; do not claim both. See the private
`legal/research/LICENSE_DECISION.md`.

## Commercial / proprietary components
Proprietary extensions and hosted services are licensed separately (private
repo). FSL/Apache rights to this engine are not restricted by those terms.
