# swarm-defence  ·  community engine (codename)

> **Working codename only — `swarm-defence`.** The public product name is pending
> a trademark-cleared naming review (an earlier codename, "Mirage", is already
> used by other security projects and is not being adopted). Do not treat any
> name here as final.
>
> **STATUS: pre-release. Licensing and contribution process are DRAFT and NOT yet
> active** — see `docs/LICENSING.md` and `CONTRIBUTING.md`. Nothing here is legal
> advice.

Active-deception & counter-agent defense for the app/API layer — the community
engine. Defends the layer autonomous attack swarms target (app/API/business
logic, pre-auth) by **denying the attacker clean ground-truth signal** and
trapping it in deception, rather than matching payloads (WAF) or watching hosts
(EDR/XDR).

## What this engine does (shipped vs. planned)

> Distinguish clearly as the code lands. At pre-release, treat all of the below
> as **planned** unless a feature's own docs say "shipped".

- Edge runtime (reverse-proxy / sidecar) with **observe mode** (default) and
  inline **protect mode**.
- Decoy/honeypot runtime with zero-false-positive alerting (decoy *content* is a
  separate proprietary module).
- Signal-denial primitives: response normalization, bounded latency, egress
  control hooks (specific recipes are a separate proprietary module).
- Base behavioral classifier (heuristics).
- Plugin interfaces so premium/proprietary modules can extend it.
- Local storage, local dashboard, webhook alerting.

## Licensing (PROPOSED — activate only after approval)

> The community engine is proposed to be **source-available under FSL-1.1-ALv2**.
> Each version would also become available under **Apache License 2.0 on the
> second anniversary** of its first availability under FSL. Armur Technologies
> Inc. offers separate proprietary extensions and hosted services. Contributors
> retain copyright and, through the applicable signed CLA, authorize Armur to use
> and license their contributions in community and proprietary products.

This is **source-available, not OSI-approved open source** — do not display an
OSI badge. See `docs/LICENSING.md`, `docs/LICENSING_FAQ.md`, `LICENSE`, and the
future-license reference `LICENSES/Apache-2.0.txt`.

## Links
`LICENSE` · `docs/LICENSING.md` · `CONTRIBUTING.md` · `SECURITY.md` ·
`docs/ACTIVE_DEFENSE_BOUNDARIES.md` · `docs/DATA_HANDLING.md` · `TRADEMARKS.md`

## Relationship to the commercial product
A separate private repository holds proprietary modules + cloud control plane
(fleet intelligence, managed hosting, dashboards). This engine is a genuinely
useful standalone product; the commercial side wins on network-effect
intelligence and no-ops, not by crippling this engine.
