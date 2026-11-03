# Threat model

## What Heramba defends

The **application / API layer that autonomous attack swarms target**: pre-auth
and authenticated business-logic surface reached over HTTP. Heramba is built for
the era of agentic offense — tools that enumerate surface, chain weaknesses, and
exfiltrate without a human in the loop.

## Who it's for

Teams running web apps/APIs who want an inline sensor and deception layer that
(a) catches automated enumeration with zero false positives, and (b) denies
attackers the clean signal their tooling depends on — without the false-positive
tax of signature WAFs.

## Attacker model

- **Automated scanners and agentic attackers** probing paths, enumerating
  objects, fingerprinting the stack, and relying on timing/error oracles.
- Reaching the app over the network (Heramba is network-layer; it does not
  inspect host processes — that's EDR's job).

## What Heramba does / does not claim

**Does (shipped):**
- Detect decoy/canary interaction with confidence 1.0 (zero FP).
- Score and flag agentic traffic from deterministic behavioral signals.
- Deny fingerprinting signal (header normalization) and blunt timing oracles
  (jitter) in protect mode.
- Cut out-of-policy outbound callbacks (egress firewall) in protect mode.

**Does not claim:**
- It is **not** a WAF and does not promise to block exploits by payload
  matching. It changes the economics (deception + signal denial), not a
  signature database.
- It does **not** patch vulnerabilities. Denying recon signal is not a fix.
- The classifier is a **heuristic**: a flagged source is *suspected* automation,
  not a proven identity. It can be evaded; it raises cost, it isn't a guarantee.
- It is **not** EDR/XDR/SIEM. It watches app-layer traffic, not hosts.

## Safe-by-default posture

- **Observe is the default** and never alters or blocks legitimate traffic.
- **Protect is explicit opt-in** and its inline actions (normalization, tarpit,
  egress deny) are bounded and documented.
- No LLM on the detection path — nothing an attacker's input can prompt-inject
  into the decision.

## Limitations & honest status

This is **pre-release**. Canary request-matching in the edge, configurable
classifier thresholds, richer signals (JA3/header-order), and content-level
deception are roadmap items, not shipped. Treat each feature as present only
where its own docs/tests say so. Report issues via `SECURITY.md`.
