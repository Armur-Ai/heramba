#!/bin/sh
# Heramba lab 09 — active defense, live.
#
# Self-contained demo of `heramba trap`: it builds the binary, starts a trap
# endpoint, fires a BENIGN request (which passes straight through), then follows
# a signed trip-URL (which confirms an attacker) and shows that source getting
# tarpitted and fed watermarked poison.
#
# Everything is defensive: the trap only serves its own content and slows an
# attacker's own requests to it. Nothing is ever sent at a client.
#
# Usage:  sh run.sh            # pick a port automatically-ish (default 8099)
#         PORT=9000 sh run.sh  # choose the port
#
# POSIX sh, no dependencies beyond a Go toolchain and curl.

set -eu

PORT="${PORT:-8099}"
BASE="http://127.0.0.1:${PORT}"

# Resolve the repo root (two levels up from this script) so the lab runs from
# anywhere and builds the binary itself.
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "${SCRIPT_DIR}/../.." && pwd)
BIN="${REPO_ROOT}/bin/heramba"
LOG=$(mktemp "${TMPDIR:-/tmp}/heramba-trap-lab.XXXXXX")

TRAP_PID=""
cleanup() {
	[ -n "${TRAP_PID}" ] && kill "${TRAP_PID}" 2>/dev/null || true
	rm -f "${LOG}"
}
trap cleanup EXIT INT TERM

say() { printf '\n\033[1;36m== %s\033[0m\n' "$1"; }

command -v curl >/dev/null 2>&1 || { echo "this lab needs curl" >&2; exit 1; }
command -v go   >/dev/null 2>&1 || { echo "this lab needs a Go toolchain" >&2; exit 1; }

say "building bin/heramba"
( cd "${REPO_ROOT}" && go build -o "${BIN}" ./cmd/heramba )
echo "built ${BIN}"

say "starting the trap on ${BASE}"
# Short drip intervals keep the demo quick; production defaults (250ms..3s)
# waste far more attacker time. --alert jsonl would emit machine-readable events.
"${BIN}" trap --listen "127.0.0.1:${PORT}" --seed 1 \
	--delay 120ms --max-delay 400ms >"${LOG}" 2>&1 &
TRAP_PID=$!

# Wait for the listener to come up.
i=0
while [ "${i}" -lt 50 ]; do
	if curl -s -o /dev/null "${BASE}/" 2>/dev/null; then break; fi
	i=$((i + 1))
	sleep 0.1
done
sed -n '1,3p' "${LOG}"

say "the bait: robots.txt advertises trip-URLs no human would ever request"
curl -s "${BASE}/robots.txt"

say "a BENIGN request passes straight through (fast, innocuous page)"
START=$(date +%s)
curl -s "${BASE}/" | sed -n '1,3p'
END=$(date +%s)
echo "benign request took $((END - START))s — not trapped."

say "an attacker follows a trip-URL (only a crawler/agent ever does)"
TRIP=$(curl -s "${BASE}/robots.txt" | awk '/Disallow: \/api\/v1\/admin/{print $2; exit}')
echo "following: ${TRIP}"
# Bound the request: once confirmed, this source is tarpitted, so the response
# drips out slowly and we cut it off to keep the demo moving.
curl -s -o /dev/null -w "  response: HTTP %{http_code}, %{size_download} bytes in %{time_total}s (tarpitted)\n" \
	--max-time 4 "${BASE}${TRIP}" || true

say "now every request from this source is sunk — a secret path gets POISON"
echo "(watermarked, inert fake secrets; cut off by the tarpit after a few seconds)"
curl -s --max-time 6 "${BASE}/.env" || true
echo

say "...and any other path gets the endless labyrinth of fake pages"
curl -s --max-time 6 "${BASE}/internal/reports" | sed -n '1,3p' || true

say "what the trap recorded"
grep -E 'decoy_hit|counter_deployed' "${LOG}" || echo "(no events captured — try a larger --max-time)"

say "done"
echo "A legitimate user never requested a trip-URL, so it was never confirmed"
echo "and never tarpitted. See docs/active-defense.md for the full write-up."
