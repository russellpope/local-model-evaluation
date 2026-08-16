#!/bin/sh
# verify.sh - build, test, and run the vsphere-inventory CLI against a local
# vcsim simulator, exercising every subcommand.
#
# This is the gate behind `make verify`. It fails (non-zero exit) if any step
# does not pass.
set -eu

# Run from the repository root (the parent of this script's directory).
cd "$(dirname "$0")/.."

BIN=bin
CLI=$BIN/vsphere-inventory
VCSIMSERVER=$BIN/vcsimserver
mkdir -p "$BIN"

echo "==> go vet ./..."
go vet ./...

echo "==> gofmt check"
if out=$(gofmt -l .); then
  if [ -n "$out" ]; then
    echo "ERROR: gofmt needed on:"
    echo "$out"
    exit 1
  fi
fi

echo "==> go test ./..."
go test ./...

echo "==> build CLI"
go build -o "$CLI" .

echo "==> build vcsimserver (local simulator)"
( cd tools/vcsimserver && go build -o "../../$VCSIMSERVER" . )

# --- Start the simulator -----------------------------------------------------
URL_FILE=$(mktemp)
ERR_FILE=$(mktemp)
SERVER_PID=""
cleanup() {
  [ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null || true
  rm -f "$URL_FILE" "$ERR_FILE"
}
trap cleanup EXIT INT TERM

# Port 0 lets the OS pick a free port; the server prints the chosen URL.
"$VCSIMSERVER" -l 127.0.0.1:0 -dc 1 -ds 3 -vm 2 -pg 3 \
  >"$URL_FILE" 2>"$ERR_FILE" &
SERVER_PID=$!

# Wait for the ready URL (first line of stdout).
VSPHERE_URL=""
i=0
while [ $i -lt 100 ]; do
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "ERROR: simulator exited early:"
    cat "$ERR_FILE"
    exit 1
  fi
  VSPHERE_URL=$(head -n 1 "$URL_FILE" 2>/dev/null || true)
  case "$VSPHERE_URL" in
    http://*|https://*) break ;;
  esac
  i=$((i + 1))
  sleep 0.1
done

if [ -z "$VSPHERE_URL" ]; then
  echo "ERROR: simulator did not report a URL in time:"
  cat "$ERR_FILE"
  exit 1
fi

echo "==> simulator ready at $VSPHERE_URL"
export VSPHERE_URL
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass

# --- Exercise every subcommand ----------------------------------------------
echo
echo "=== vms ==="
"$CLI" vms

echo
echo "=== datastores ==="
"$CLI" datastores

echo
echo "=== vswitches ==="
SWITCH_OUT=$("$CLI" vswitches)
printf '%s\n' "$SWITCH_OUT"

# Discover a real port group name from our own vswitches output (column 3,
# first data row) instead of hardcoding an inventory name.
PORTGROUP=$(printf '%s\n' "$SWITCH_OUT" | awk 'NR==2 && NF>=3 {print $3; exit}')
if [ -z "$PORTGROUP" ]; then
  echo "ERROR: could not discover a port group name from vswitches output"
  exit 1
fi

echo
echo "=== vswitches --portgroup $PORTGROUP ==="
"$CLI" vswitches --portgroup "$PORTGROUP"

echo
echo "==> verify: all subcommands ran successfully against vcsim"
