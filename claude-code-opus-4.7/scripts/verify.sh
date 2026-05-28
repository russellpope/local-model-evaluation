#!/usr/bin/env bash
#
# Self-check: build the CLI, start the bundled vcsim simulator, exercise every
# subcommand (vms, datastores, vswitches, and vswitches --portgroup) against it,
# then tear the simulator down. Exits non-zero on any failure.
#
# This is portable across make versions (macOS ships GNU make 3.81, which does
# not support .ONESHELL), so the lifecycle/trap logic lives here in one shell.
set -euo pipefail

cd "$(dirname "$0")/.."

BINARY="./vsphere-inventory"
VCSIM_BIN="./bin/vcsim"
VCSIM_LOG="./.vcsim-verify.log"
VCSIM_ADDR="127.0.0.1:8989"
VCSIM_URL="https://${VCSIM_ADDR}/sdk"

echo "==> building CLI and simulator launcher"
go build -o "$BINARY" .
mkdir -p bin
go build -o "$VCSIM_BIN" ./tools/vcsim

echo "==> starting vcsim on ${VCSIM_ADDR}"
"$VCSIM_BIN" -vm 8 -ds 3 -pg 3 -l "$VCSIM_ADDR" >"$VCSIM_LOG" 2>&1 &
VCSIM_PID=$!
cleanup() { kill "$VCSIM_PID" >/dev/null 2>&1 || true; rm -f "$VCSIM_LOG"; }
trap cleanup EXIT

echo "    waiting for simulator to accept connections..."
for _ in $(seq 1 100); do
  if grep -q "listening at" "$VCSIM_LOG" 2>/dev/null; then break; fi
  if ! kill -0 "$VCSIM_PID" >/dev/null 2>&1; then
    echo "vcsim failed to start:"
    cat "$VCSIM_LOG"
    exit 1
  fi
  sleep 0.1
done

export VSPHERE_URL="$VCSIM_URL" VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true

echo "==> vms"
"$BINARY" vms
echo "==> datastores"
"$BINARY" datastores
echo "==> vswitches"
"$BINARY" vswitches

# Discover a port-group name from the vswitches output rather than hardcoding;
# columns are separated by two or more spaces, so names with single spaces
# (e.g. "VM Network") survive the split.
PG="$("$BINARY" vswitches | awk -F'  +' 'NR>1 && $3 != "-" {print $3; exit}')"
if [ -z "$PG" ]; then
  echo "no port group discovered from vswitches output"
  exit 1
fi
echo "==> vswitches --portgroup \"$PG\""
"$BINARY" vswitches --portgroup "$PG"

echo
echo "==> SUCCESS: all subcommands ran against vcsim with exit 0"
