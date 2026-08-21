#!/usr/bin/env bash
#
# Self-verification for vsphere-inventory against the vcsim simulator.
#
# It runs go vet and go test, builds the binary, then starts vcsim in the
# background and exercises every subcommand (including --portgroup) against it.
# The simulator is torn down when the script exits, regardless of the outcome.
#
# vcsim ships as a separate module (github.com/vmware/govmomi/vcsim) and is only
# needed for this smoke test, so it is fetched on demand and never committed to
# the application's go.mod.
set -o errexit
set -o nounset
set -o pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

VCSIM_PKG="github.com/vmware/govmomi/vcsim"
VCSIM_VERSION="v0.0.0-20260820135757-2d753e5caa0c"
ADDR="127.0.0.1:8989"
HOST="127.0.0.1"
PORT="8989"
ENVIRONMENT="https://${HOST}:${PORT}/sdk"
USERNAME="Administrator@vsphere-local"
PASSWORD="password"

VCSIM_PID=""
VCSIM_BIN=""

cleanup() {
  if [ -n "$VCSIM_PID" ]; then
    kill "$VCSIM_PID" >/dev/null 2>&1 || true
    wait "$VCSIM_PID" >/dev/null 2>&1 || true
  fi
  if [ -n "$VCSIM_BIN" ]; then
    rm -rf "$(dirname "$VCSIM_BIN")" 2>/dev/null || true
  fi
}
trap cleanup EXIT

fail() {
  echo "verify: FAILED: $*" >&2
  exit 1
}

echo "==> go vet ./..."
go vet ./...

echo "==> go test ./..."
go test ./...

echo "==> go build ./..."
go build -o bin/vsphere-inventory .

# vcsim is a verification-only tool. It ships as a separate module with replace
# directives, so the "@version" run override is rejected; instead add it to the
# build list (it is already in the module cache, so this works offline) and build
# it to a temporary binary so it can be killed cleanly on teardown.
echo "==> ensuring vcsim ($VCSIM_PKG@$VCSIM_VERSION) is available"
go get "${VCSIM_PKG}@${VCSIM_VERSION}" >/dev/null 2>&1 || \
  fail "could not fetch vcsim (run: go get ${VCSIM_PKG}@${VCSIM_VERSION})"

VCSIM_BIN="$(mktemp -d)/vcsim"
echo "==> building vcsim"
go build -o "$VCSIM_BIN" "${VCSIM_PKG}" >/dev/null 2>&1 || \
  fail "could not build vcsim"

echo "==> starting vcsim on ${ADDR}"
"$VCSIM_BIN" -vm 8 -ds 3 -pg 3 -cluster 1 -host 1 -l "$ADDR" >/dev/null 2>&1 &
VCSIM_PID=$!

# The SDK endpoint rejects GET (405), so readiness is a plain TCP probe.
echo "==> waiting for vcsim to accept connections on ${HOST}:${PORT}"
ready=0
for _ in $(seq 1 60); do
  if nc -z -w1 "$HOST" "$PORT"; then
    ready=1
    break
  fi
  if ! kill -0 "$VCSIM_PID" >/dev/null 2>&1; then
    fail "vcsim exited before becoming ready"
  fi
  sleep 0.5
done
[ "$ready" -eq 1 ] || fail "vcsim did not become ready in time"

BIN="./bin/vsphere-inventory"
BASE_URL_FLAGS=(--url "$ENVIRONMENT" --username "$USERNAME" --password "$PASSWORD" --insecure)

check_table() {
  # $1 = label, $2 = output; fail unless the output contains a table header.
  printf '%s\n' "$2" | grep -q "$1" || fail "$3 produced no table"
}

echo "==> vms"
out="$("$BIN" vms "${BASE_URL_FLAGS[@]}")" || fail "vms failed"
printf '%s\n' "$out"
check_table "NAME" "$out" "vms"

echo "==> datastores"
out="$("$BIN" datastores "${BASE_URL_FLAGS[@]}")" || fail "datastores failed"
printf '%s\n' "$out"
check_table "NAME" "$out" "datastores"

echo "==> vswitches"
out="$("$BIN" vswitches "${BASE_URL_FLAGS[@]}")" || fail "vswitches failed"
printf '%s\n' "$out"
check_table "SWITCH" "$out" "vswitches"

echo "==> vswitches --portgroup DC0_DVPG0"
out="$("$BIN" vswitches --portgroup DC0_DVPG0 "${BASE_URL_FLAGS[@]}")" || fail "portgroup lookup failed"
printf '%s\n' "$out"
check_table "NAME" "$out" "portgroup lookup"

echo "==> vswitches --portgroup \"VM Network\" (standard switch)"
out="$("$BIN" vswitches --portgroup "VM Network" "${BASE_URL_FLAGS[@]}")" || fail "standard portgroup lookup failed"
printf '%s\n' "$out"
check_table "NAME" "$out" "standard portgroup lookup"

# vcsim is verification-only; restore a tidy go.mod so it never leaks into the
# committed module graph.
echo "==> restoring tidy go.mod"
go mod tidy >/dev/null 2>&1

echo "==> verify: OK (vcsim torn down)"
