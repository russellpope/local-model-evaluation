#!/usr/bin/env bash
# Full automated check: vet, unit/integration tests, then every subcommand
# against a live vcsim simulator. Exits non-zero on the first failure and
# always tears the simulator down.
set -euo pipefail

cd "$(dirname "$0")/.."
BIN="bin/vsphere-inventory"
VCSIM_PORT="${VCSIM_PORT:-8989}"
VCSIM_HOST="127.0.0.1"
VCSIM_PID=""
LOG="$(mktemp)"

fail() {
  echo "verify: FAILED: $*" >&2
  exit 1
}

cleanup() {
  if [[ -n "$VCSIM_PID" ]] && kill -0 "$VCSIM_PID" 2>/dev/null; then
    echo "==> stopping vcsim (pid $VCSIM_PID)"
    # `go run` exec's a compiled child that holds the port; kill children
    # first, then the parent, so nothing is orphaned.
    pkill -P "$VCSIM_PID" 2>/dev/null || true
    kill "$VCSIM_PID" 2>/dev/null || true
    wait "$VCSIM_PID" 2>/dev/null || true
  fi
  # Reap any survivor still holding the listen port.
  if command -v lsof >/dev/null 2>&1; then
    stale="$(lsof -nP -iTCP:"$VCSIM_PORT" -sTCP:LISTEN -t 2>/dev/null || true)"
    [[ -z "$stale" ]] || kill $stale 2>/dev/null || true
  fi
  rm -f "$LOG"
}
trap cleanup EXIT INT TERM

echo "==> gofmt"
unformatted="$(gofmt -l cmd internal main.go || true)"
[[ -z "$unformatted" ]] || fail "gofmt needed: $unformatted"

echo "==> go vet ./..."
go vet ./... || fail "go vet"

echo "==> go test ./..."
go test ./... || fail "go test"

echo "==> build $BIN"
mkdir -p bin
go build -o "$BIN" . || fail "go build"


echo "==> start vcsim on ${VCSIM_HOST}:${VCSIM_PORT} (8 VMs, 3 datastores, 3 DVPGs)"
if command -v nc >/dev/null 2>&1 && nc -z "$VCSIM_HOST" "$VCSIM_PORT"; then
  fail "port ${VCSIM_PORT} already in use; stop the listener or set VCSIM_PORT"
fi
# vcsim ships as a nested govmomi module that cannot be fetched with
# `go run pkg@version` (replace-directive restriction); tools/vcsim vendors
# its verbatim driver against the pinned govmomi version.
go -C tools/vcsim run . -l "${VCSIM_HOST}:${VCSIM_PORT}" -vm 8 -ds 3 -pg 3 >"$LOG" 2>&1 &
VCSIM_PID=$!

for i in $(seq 1 60); do
  if grep -q "GOVC_URL=" "$LOG" 2>/dev/null; then break; fi
  kill -0 "$VCSIM_PID" 2>/dev/null || { cat "$LOG" >&2; fail "vcsim exited early"; }
  sleep 0.5
done
grep -q "GOVC_URL=" "$LOG" || fail "vcsim did not become ready"
echo "==> vcsim ready"

export VSPHERE_URL="https://${VCSIM_HOST}:${VCSIM_PORT}/sdk"
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

run() { # run <label> <args...> — assert exit 0 and header on first line
  local label="$1"; shift
  echo "==> ${BIN##*/} $* [$label]"
  local out
  out="$("$BIN" "$@" 2>&1)" || { printf '%s\n' "$out" >&2; fail "$label exit non-zero"; }
  printf '%s\n' "$out"
  [[ -n "$out" ]] || fail "$label produced no output"
}

run "vms" vms
run "datastores" datastores
VSOUT="$("$BIN" vswitches)"
printf '%s\n' "$VSOUT"
grep -q "standard" <<<"$VSOUT" || fail "vswitches: no standard switch rows"
grep -q "distributed" <<<"$VSOUT" || fail "vswitches: no distributed rows"

# Discover a real distributed port group name from our own listing; extract
# the PORTGROUP cell by header column offsets (VLAN may contain spaces).
HDR="$(printf '%s\n' "$VSOUT" | head -1)"
ROW="$(printf '%s\n' "$VSOUT" | grep " distributed " | head -1)"
[[ -n "$ROW" ]] || fail "no distributed row to derive a port group from"
S=$(( $(awk -v h="$HDR" 'BEGIN{print index(h,"PORTGROUP")}') - 1 ))
E=$(( $(awk -v h="$HDR" 'BEGIN{print index(h,"VLAN")}') - 1 ))
PG="$(printf '%s\n' "$ROW" | cut -c$((S+1))-$((E)) | sed 's/[[:space:]]*$//')"
[[ -n "$PG" ]] || fail "could not parse a port group name from the listing"
echo "==> discovered port group: '$PG'"

OUT="$("$BIN" vswitches --portgroup "$PG")"
printf '%s\n' "$OUT"
grep -q "^NAME" <<<"$OUT" || fail "--portgroup output missing VM header"
ROWS=$(printf '%s\n' "$OUT" | tail -n +2 | grep -c . || true)
[[ "$ROWS" -ge 1 ]] || fail "--portgroup '$PG' returned no VMs but the simulator attaches VMs to DVPG0"

echo "==> verify: ALL CHECKS PASSED"
