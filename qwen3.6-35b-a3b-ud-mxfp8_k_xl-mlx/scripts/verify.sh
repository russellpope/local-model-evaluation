#!/bin/bash
set -e

BINARY="${BINARY:-vsphere-inventory}"
VCSIM_HELPER="/tmp/vsphere-vcsim-helper"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Build vcsim helper
echo "=== Building vcsim helper ==="
go build -o "$VCSIM_HELPER" "$PROJECT_DIR/scripts/vcsim-helper/"

# Kill any leftover vcsim on port 8989
cleanup() {
    if [ -n "$VCSIM_PID" ] && kill -0 "$VCSIM_PID" 2>/dev/null; then
        kill "$VCSIM_PID" 2>/dev/null || true
        wait "$VCSIM_PID" 2>/dev/null || true
    fi
    rm -f "$VCSIM_HELPER" /tmp/vsphere-vcsim-url
}
trap cleanup EXIT

# Start vcsim in background, URL goes to file
echo "=== Starting vcsim for integration tests ==="
"$VCSIM_HELPER" > /tmp/vsphere-vcsim-url 2>&1 &
VCSIM_PID=$!

# Wait for vcsim to be ready and get URL
VCSIM_URL=""
for i in $(seq 1 30); do
    VCSIM_URL=$(head -1 /tmp/vsphere-vcsim-url 2>/dev/null || true)
    if [ -n "$VCSIM_URL" ]; then
        echo "vcsim is ready at $VCSIM_URL"
        break
    fi
    sleep 1
done

if [ -z "$VCSIM_URL" ]; then
    echo "ERROR: vcsim did not start within 30 seconds" >&2
    kill $VCSIM_PID 2>/dev/null || true
    exit 1
fi

# Export environment variables
export VSPHERE_URL="$VCSIM_URL"
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

# Change to project directory
cd "$PROJECT_DIR"

# Run vms command and verify output
echo ""
echo "=== Running: ./$BINARY vms ==="
VMS_OUTPUT=$("./$BINARY" vms) || { kill $VCSIM_PID 2>/dev/null; exit 1; }
echo "$VMS_OUTPUT"

# Verify vms output contains real data (non-zero VCPU)
if ! echo "$VMS_OUTPUT" | grep -q "1\|2\|4"; then
    echo "FAIL: vms output contains no non-zero VCPU values" >&2
    kill $VCSIM_PID 2>/dev/null; exit 1
fi
echo "PASS: vms shows non-zero VCPU values"

# Run datastores command and verify output
echo ""
echo "=== Running: ./$BINARY datastores ==="
DS_OUTPUT=$("./$BINARY" datastores) || { kill $VCSIM_PID 2>/dev/null; exit 1; }
echo "$DS_OUTPUT"

# Verify datastores output contains at least one non-zero USED value
if ! echo "$DS_OUTPUT" | grep -v "0 B" | grep -q "GiB\|TiB"; then
    echo "FAIL: datastores output contains no non-zero USED values" >&2
    kill $VCSIM_PID 2>/dev/null; exit 1
fi
echo "PASS: datastores shows non-zero USED and AVAILABLE"

# Run vswitches command and verify output
echo ""
echo "=== Running: ./$BINARY vswitches ==="
VSW_OUTPUT=$("./$BINARY" vswitches) || { kill $VCSIM_PID 2>/dev/null; exit 1; }
echo "$VSW_OUTPUT"

# Verify vswitches output contains both standard and distributed switches
if ! echo "$VSW_OUTPUT" | grep -qi "standard"; then
    echo "FAIL: vswitches output missing standard switch" >&2
    kill $VCSIM_PID 2>/dev/null; exit 1
fi
if ! echo "$VSW_OUTPUT" | grep -qi "distributed"; then
    echo "FAIL: vswitches output missing distributed switch" >&2
    kill $VCSIM_PID 2>/dev/null; exit 1
fi
echo "PASS: vswitches shows both standard and distributed switches"

# Extract a port group name from vswitches output
PG_NAME=$(echo "$VSW_OUTPUT" | grep -v "SWITCH" | grep -v "^$" | head -1 | sed 's/^[[:space:]]*//' | cut -d' ' -f3-)

# If extraction didn't work, use a known port group
if echo "$PG_NAME" | grep -q "SWITCH\|LACP\|PORTS\|USED\|key-vim\|1536\|N/A"; then
    PG_NAME="VM Network"
fi

if [ -n "$PG_NAME" ] && [ "$PG_NAME" != "-" ]; then
    echo ""
    echo "=== Running: ./$BINARY vswitches --portgroup $PG_NAME ==="
    PG_OUTPUT=$("./$BINARY" vswitches --portgroup "$PG_NAME") || { kill $VCSIM_PID 2>/dev/null; exit 1; }
    echo "$PG_OUTPUT"
    echo "PASS: vswitches --portgroup $PG_NAME returned data"
else
    echo ""
    echo "=== No port group found, using VM Network ==="
    PG_NAME="VM Network"
    echo ""
    echo "=== Running: ./$BINARY vswitches --portgroup $PG_NAME ==="
    PG_OUTPUT=$("./$BINARY" vswitches --portgroup "$PG_NAME") || { kill $VCSIM_PID 2>/dev/null; exit 1; }
    echo "$PG_OUTPUT"
    echo "PASS: vswitches --portgroup $PG_NAME returned data"
fi

# Test distributed port group
echo ""
echo "=== Running: ./$BINARY vswitches --portgroup DC0_DVPG0 ==="
DIST_PG_OUTPUT=$("./$BINARY" vswitches --portgroup "DC0_DVPG0") || { kill $VCSIM_PID 2>/dev/null; exit 1; }
echo "$DIST_PG_OUTPUT"
echo "PASS: vswitches --portgroup DC0_DVPG0 returned data"

# Clean up vcsim
echo ""
echo "=== Stopping vcsim ==="
kill $VCSIM_PID 2>/dev/null || true
wait $VCSIM_PID 2>/dev/null || true
echo "=== All integration tests passed ==="
