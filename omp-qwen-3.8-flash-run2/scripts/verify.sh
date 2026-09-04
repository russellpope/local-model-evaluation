#!/usr/bin/env bash
# verify.sh — full check: vet + unit tests + live run of every subcommand
# against the bundled vcsim simulator. Exits non-zero on any failure and
# always tears the simulator down.
set -euo pipefail

cd "$(dirname "$0")/.."

PORT="${VCSIM_PORT:-18989}"
BIN="bin/govc-inventory"
LOG="$(mktemp)"
VCSIM_PID=""
RUNPID=""

cleanup() {
  [ -n "$VCSIM_PID" ] && kill "$VCSIM_PID" 2>/dev/null || true
  [ -n "$RUNPID" ] && kill "$RUNPID" 2>/dev/null || true
  wait 2>/dev/null || true
  rm -f "$LOG"
}
trap cleanup EXIT

step() { printf '\n=== %s ===\n' "$*"; }

# run <label> <header-regex> <args...> — execute the binary, fail on non-zero
# exit or a missing header line. Output captured, never piped into grep -q
# (early exit would SIGPIPE the program).
run() {
  local label="$1" header="$2"; shift 2
  step "$label"
  local out rc=0
  out="$("$@" 2>&1)" || rc=$?
  if [ "$rc" -ne 0 ]; then echo "[$label] exit=$rc"; echo "$out"; exit 1; fi
  if ! printf '%s\n' "$out" | grep -Eq "$header"; then
    echo "[$label] output missing expected header $header:"; echo "$out"; exit 1
  fi
  LAST_OUT="$out"
  echo "-- $label OK"
}

step "go vet"
go vet ./...

step "go test"
go test ./...

step "build"
mkdir -p bin
go build -o "$BIN" .

step "start vcsim on 127.0.0.1:$PORT"
go run github.com/vmware/govmomi/vcsim -l "127.0.0.1:$PORT" -vm 8 -ds 3 -pg 3 >"$LOG" 2>&1 &
RUNPID=$!

# vcsim prints its readiness line (export GOVC_URL=... GOVC_SIM_PID=NNNN).
for _ in $(seq 1 60); do
  if grep -q 'GOVC_SIM_PID' "$LOG" 2>/dev/null; then break; fi
  if ! kill -0 "$RUNPID" 2>/dev/null; then
    echo "vcsim exited early:"; cat "$LOG"; exit 1
  fi
  sleep 0.5
done
if ! grep -q 'GOVC_SIM_PID' "$LOG"; then
  echo "vcsim not ready after 30s"; cat "$LOG"; exit 1
fi
VCSIM_PID="$(sed -n 's/.*GOVC_SIM_PID=\([0-9]*\).*/\1/p' "$LOG")"
echo "vcsim ready (pid $VCSIM_PID)"

export VSPHERE_URL="https://127.0.0.1:$PORT/sdk"
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

run "vms"        '^NAME'   "$BIN" vms
run "datastores" '^NAME'   "$BIN" datastores
run "vswitches"  '^SWITCH' "$BIN" vswitches

# Discover a real portgroup name from the vswitches output above.
PG="$(printf '%s\n' "$LAST_OUT" | awk '$2=="distributed" && $3 !~ /Uplinks/ {print $3; exit}')"
if [ -z "$PG" ]; then echo "could not discover a distributed portgroup"; exit 1; fi
step "vswitches --portgroup $PG"
OUT="$("$BIN" vswitches --portgroup "$PG")"
printf '%s\n' "$OUT" | head -4
ROWS=$(( $(printf '%s\n' "$OUT" | wc -l) - 1 ))
if [ "$ROWS" -lt 1 ]; then echo "portgroup $PG lists no VMs"; exit 1; fi
echo "-- portgroup OK ($ROWS VMs)"

step "unknown portgroup is a clean error"
set +e
ERR="$("$BIN" vswitches --portgroup no-such-pg 2>&1)" && rc=0 || rc=$?
set -e
[ "$rc" -ne 0 ] || { echo "want non-zero exit for unknown port group"; exit 1; }
case "$ERR" in *not\ found*) ;; *) echo "want not-found error, got: $ERR"; exit 1;; esac
echo "-- unknown portgroup OK"

step "config file + env + flag precedence smoke"
CFG="${TMPDIR:-/tmp}/govc-inventory-verify.$$.yaml"
{
  echo "url: https://127.0.0.1:$PORT/sdk"
  echo "username: user"
  echo "password: pass"
  echo "insecure: true"
  echo "timeout: 30s"
} > "$CFG"
"$BIN" vms --config "$CFG" >/dev/null            # file alone works
if VSPHERE_URL="https://127.0.0.1:1/sdk" "$BIN" vms --config "$CFG" >/dev/null 2>&1; then
  echo "expected env VSPHERE_URL to override the file"; rm -f "$CFG"; exit 1
fi
VSPHERE_URL="https://127.0.0.1:1/sdk" "$BIN" vms --config "$CFG" --url "https://127.0.0.1:$PORT/sdk" >/dev/null
rm -f "$CFG"
echo "-- config OK"

step "error handling: unreachable endpoint yields wrapped error"
set +e
ERR="$(VSPHERE_URL=https://127.0.0.1:1/sdk VSPHERE_USERNAME=x VSPHERE_PASSWORD=y VSPHERE_INSECURE=true "$BIN" vms 2>&1)" && rc=0 || rc=$?
set -e
[ "$rc" -ne 0 ] || { echo "want non-zero exit for unreachable endpoint"; exit 1; }
case "$ERR" in
  *connect\ to*) ;;
  *) echo "want wrapped connect error, got: $ERR"; exit 1 ;;
esac
echo "-- error handling OK"

printf '\nALL CHECKS PASSED\n'
