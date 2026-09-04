#!/usr/bin/env bash
# Full self-check: vet + unit tests + a live end-to-end run against the
# govmomi vcsim simulator. Exits non-zero on any failure; tears the simulator
# down on exit.
set -euo pipefail

PORT="${VCSIM_PORT:-18989}"
BIN="${BIN:-./vsphere-inventory}"
WORK="$(mktemp -d)"
# vcsim ships as a pinned "tool" in go.mod (never imported by the binary).
VCSIM="${VCSIM:-go run github.com/vmware/govmomi/vcsim}"

fail() { echo "verify: FAIL: $*" >&2; exit 1; }

echo "==> go vet ./..."
go vet ./...

echo "==> go test ./..."
go test ./...

echo "==> go build"
go build -o "${BIN#./}" .

echo "==> starting vcsim on 127.0.0.1:${PORT}"
# shellcheck disable=SC2086
${VCSIM} -l "127.0.0.1:${PORT}" -vm 8 -ds 3 -pg 3 >"$WORK"/vcsim.log 2>&1 &
RUNNER_PID=$!
cleanup() {
	# vcsim prints "export ... GOVC_SIM_PID=<pid>" — kill the simulator
	# itself as well as the go-run wrapper.
	SIM_PID="$(sed -n 's/.*GOVC_SIM_PID=\([0-9]*\).*/\1/p' "$WORK"/vcsim.log 2>/dev/null | head -1)"
	if [ -n "${SIM_PID:-}" ]; then
		kill "${SIM_PID}" 2>/dev/null || true
	fi
	kill "${RUNNER_PID}" 2>/dev/null || true
	wait "${RUNNER_PID}" 2>/dev/null || true
	rm -rf "${WORK}"
}
trap cleanup EXIT

ready=0
for _ in $(seq 1 120); do
	# Any HTTP response (including 405 to GET) means the SOAP endpoint is up.
	if curl -sk -o /dev/null "https://127.0.0.1:${PORT}/sdk"; then
		ready=1
		break
	fi
	sleep 1
done
[ "${ready}" = 1 ] || { cat "$WORK"/vcsim.log >&2 || true; fail "vcsim did not come up on port ${PORT}"; }

export VSPHERE_URL="https://127.0.0.1:${PORT}/sdk"
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

echo "==> vms"
"${BIN}" vms >"$WORK"/vms.out || fail "vms"
grep -q '^NAME' "$WORK"/vms.out || fail "vms header"
[ "$(wc -l <"$WORK"/vms.out)" -gt 1 ] || fail "vms returned no rows"

echo "==> datastores"
"${BIN}" datastores >"$WORK"/ds.out || fail "datastores"
grep -q '^NAME' "$WORK"/ds.out || fail "datastores header"
[ "$(wc -l <"$WORK"/ds.out)" -gt 1 ] || fail "datastores returned no rows"

echo "==> vswitches"
"${BIN}" vswitches >"$WORK"/vsw.out || fail "vswitches"
grep -q '^SWITCH' "$WORK"/vsw.out || fail "vswitches header"
grep -q 'distributed' "$WORK"/vsw.out || fail "no distributed rows"
grep -q 'standard' "$WORK"/vsw.out || fail "no standard rows"

# Discover portgroup names from our own listing rather than hardcoding them:
# the name is everything between the SWITCH TYPE column and the five trailing
# fixed columns (names may contain spaces).
PG="$(awk 'NR>1 && $2=="distributed" {out=""; for (i=3; i<=NF-5; i++) out = out (i>3 ? " " : "") $i; print out; exit}' "$WORK"/vsw.out)"
[ -n "${PG}" ] || fail "could not discover a portgroup name from vswitches output"

echo "==> vswitches --portgroup ${PG}"
"${BIN}" vswitches --portgroup "${PG}" >"$WORK"/pg.out || fail "vswitches --portgroup"
grep -q '^NAME' "$WORK"/pg.out || fail "--portgroup header"

SPG="$(awk 'NR>1 && $2=="standard" {out=""; for (i=3; i<=NF-5; i++) out = out (i>3 ? " " : "") $i; print out; exit}' "$WORK"/vsw.out)"
if [ -n "${SPG}" ]; then
	echo "==> vswitches --portgroup '${SPG}' (standard)"
	"${BIN}" vswitches --portgroup "${SPG}" >/dev/null || fail "vswitches --portgroup standard"
fi

echo "verify: OK"
