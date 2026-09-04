#!/usr/bin/env bash
# Full verification for vsphere-inventory:
#   1. go vet ./...
#   2. go test -count=1 ./...   (must pass with zero failures and zero skips)
#   3. go build
#   4. start vcsim in the background, wait for readiness
#   5. run every subcommand (including --portgroup with a name discovered
#      from the vswitches output) against the simulator
#   6. check the no-URL error path
# Exits non-zero on the first failure and always tears vcsim down.
set -euo pipefail

cd "$(dirname "$0")/.."

BIN=${BIN:-bin/vsphere-inventory}
PORT=${VCSIM_PORT:-18989}
LOG=$(mktemp)
VCSIM_PID=""

cleanup() {
	if [ -n "$VCSIM_PID" ]; then
		kill "$VCSIM_PID" 2>/dev/null || true
		wait "$VCSIM_PID" 2>/dev/null || true
	fi
}
trap cleanup EXIT

fail() {
	echo "verify: FAIL: $*" >&2
	exit 1
}

echo "==> go vet ./..."
go vet ./...

echo "==> go test -count=1 ./..."
go test -count=1 ./...

echo "==> go build"
go build -o "$BIN" .

echo "==> starting vcsim on 127.0.0.1:$PORT"
# Build vcsim once and run the binary directly: `go run` spawns a child
# process that would survive a plain kill of the wrapper, breaking teardown.
VCSIM_BIN="$(mktemp -d)/vcsim"
go build -o "$VCSIM_BIN" github.com/vmware/govmomi/vcsim
"$VCSIM_BIN" -l "127.0.0.1:$PORT" -vm 8 -ds 3 -pg 3 >"$LOG" 2>&1 &
VCSIM_PID=$!

ready=""
i=0
while [ $i -lt 60 ]; do
	if (exec 3<>"/dev/tcp/127.0.0.1/$PORT") 2>/dev/null; then
		ready=1
		break
	fi
	if ! kill -0 "$VCSIM_PID" 2>/dev/null; then
		cat "$LOG" >&2
		fail "vcsim exited before becoming ready"
	fi
	i=$((i + 1))
	sleep 0.5
done
[ -n "$ready" ] || { cat "$LOG" >&2; fail "vcsim did not become ready"; }

export VSPHERE_URL="https://127.0.0.1:$PORT/sdk"
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

echo "==> vms"
"$BIN" vms || fail "vms"

echo "==> datastores"
"$BIN" datastores || fail "datastores"

echo "==> vswitches"
SWITCHES_OUT=$("$BIN" vswitches) || fail "vswitches"
echo "$SWITCHES_OUT"

PG=$(printf '%s\n' "$SWITCHES_OUT" | awk 'NR>1 && $1 != "SWITCH" { print $3; exit }')
[ -n "$PG" ] || fail "no port group discovered in vswitches output"
echo "==> vswitches --portgroup \"$PG\" (name discovered from output above)"
PG_OUT=$("$BIN" vswitches --portgroup "$PG") || fail "vswitches --portgroup \"$PG\""
echo "$PG_OUT"
printf '%s\n' "$PG_OUT" | grep -q '^NAME$' || fail "portgroup output missing NAME header"

echo "==> error path: unknown port group must exit non-zero"
if "$BIN" vswitches --portgroup "definitely-not-a-portgroup" >/dev/null 2>&1; then
	fail "unknown port group should fail"
fi

echo "==> error path: missing VSPHERE_URL must exit non-zero"
unset VSPHERE_URL
if "$BIN" vms >/dev/null 2>&1; then
	fail "vms should fail without a configured URL"
fi

echo "verify: OK"
