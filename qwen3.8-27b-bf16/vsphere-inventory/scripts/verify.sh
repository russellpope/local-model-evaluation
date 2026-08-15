#!/usr/bin/env bash
# End-to-end check against the govmomi vCenter simulator (vcsim).
#
# Starts vcsim in the background, waits for readiness, builds the binary,
# runs every subcommand (including a --portgroup lookup whose value is
# discovered from the vswitches output), and tears the simulator down.
# Exits non-zero on any failure.
set -euo pipefail

cd "$(dirname "$0")/.."

BINARY=vsphere-inventory
PORT=8989
URL="https://127.0.0.1:${PORT}/sdk"

export VSPHERE_URL=$URL
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

VCSIM_PID=""
VCSIM_TMP=""
cleanup() {
  if [[ -n "$VCSIM_PID" ]]; then
    kill "$VCSIM_PID" 2>/dev/null || true
    wait "$VCSIM_PID" 2>/dev/null || true
  fi
  if [[ -n "$VCSIM_TMP" ]]; then
    rm -rf "$VCSIM_TMP"
  fi
}
trap cleanup EXIT

go build -o "$BINARY" .

# Build the vcsim binary (same govmomi version as the module) and run it
# directly, so its PID is the server process itself and teardown is clean.
VCSIM_TMP="$(mktemp -d)"
VCSIM_BIN="${VCSIM_TMP}/vcsim"
go build -o "$VCSIM_BIN" github.com/vmware/govmomi/vcsim
echo "==> starting vcsim (8 VMs, 3 datastores, 3 port groups)"
# -username/-password make the simulator enforce credentials, so auth
# failures would surface as errors instead of being silently accepted.
"$VCSIM_BIN" -vm 8 -ds 3 -pg 3 -l "127.0.0.1:${PORT}" -username user -password pass &
VCSIM_PID=$!

ready=""
for _ in $(seq 1 120); do
  if curl -sk "$URL" >/dev/null 2>&1; then
    ready=1
    break
  fi
  # Bail out early if the process died.
  kill -0 "$VCSIM_PID" 2>/dev/null || {
    echo "vcsim exited before becoming ready" >&2
    exit 1
  }
  sleep 0.5
done
if [[ -z "$ready" ]]; then
  echo "vcsim did not become ready in time" >&2
  exit 1
fi
echo "==> vcsim ready at $URL"

echo "==> vms"
./"$BINARY" vms

echo "==> datastores"
./"$BINARY" datastores

echo "==> vswitches"
./"$BINARY" vswitches

echo "==> vswitches --portgroup <discovered>"
# Discover a real port group name from our own vswitches output instead of
# hardcoding one: first data column after the header, skipping the "-" rows
# of switches that have no port groups.
PG="$(./"$BINARY" vswitches | awk 'NR > 1 && $3 != "-" { print $3; exit }')"
if [[ -z "$PG" ]]; then
  echo "no port group name found in vswitches output" >&2
  exit 1
fi
echo "    (using port group: $PG)"
./"$BINARY" vswitches --portgroup "$PG"

echo "==> verify: OK"
