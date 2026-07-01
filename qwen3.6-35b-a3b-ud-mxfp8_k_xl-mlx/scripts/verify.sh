#!/bin/bash
set -e

BINARY="${BINARY:-vsphere-inventory}"
VCSIM_HELPER="/tmp/vsphere-vcsim"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Build vcsim helper
echo "=== Building vcsim helper ==="
go build -o "$VCSIM_HELPER" "$PROJECT_DIR/./cmd/vsphere-inventory/verify/"

# Start vcsim in background
echo "=== Starting vcsim for integration tests ==="
"$VCSIM_HELPER" &
VCSIM_PID=$!
echo "vcsim started with PID $VCSIM_PID"

# Wait for vcsim to be ready
for i in $(seq 1 30); do
    if curl -s http://localhost:8989/ 2>/dev/null | grep -q "OK"; then
        echo "vcsim is ready"
        break
    fi
    sleep 1
done

# Export environment variables
export VSPHERE_URL=https://127.0.0.1:8989/sdk
export VSPHERE_USERNAME=user
export VSPHERE_PASSWORD=pass
export VSPHERE_INSECURE=true

# Change to project directory
cd "$PROJECT_DIR"

# Run vms command
echo ""
echo "=== Running: ./$BINARY vms ==="
"./$BINARY" vms || { kill $VCSIM_PID 2>/dev/null; exit 1; }

# Run datastores command
echo ""
echo "=== Running: ./$BINARY datastores ==="
"./$BINARY" datastores || { kill $VCSIM_PID 2>/dev/null; exit 1; }

# Run vswitches command
echo ""
echo "=== Running: ./$BINARY vswitches ==="
VSWITCHES_OUTPUT=$("./$BINARY" vswitches) || { kill $VCSIM_PID 2>/dev/null; exit 1; }
echo "$VSWITCHES_OUTPUT"

# Extract a port group name from vswitches output
# The port group is the 3rd column, which may contain spaces
# Use a more robust extraction: get text between 2nd and 3rd column boundaries
PG_NAME=$(echo "$VSWITCHES_OUTPUT" | grep -v "SWITCH" | grep -v "^$" | head -1 | sed 's/^[[:space:]]*//' | cut -d' ' -f3-)

# If the above doesn't work well, try a simpler approach
if echo "$PG_NAME" | grep -q "SWITCH\|LACP\|PORTS\|USED\|key-vim\|1536\|N/A"; then
    PG_NAME="VM Network"
fi
if [ -n "$PG_NAME" ] && [ "$PG_NAME" != "-" ]; then
    echo ""
    echo "=== Running: ./$BINARY vswitches --portgroup $PG_NAME ==="
    "./$BINARY" vswitches --portgroup "$PG_NAME" || { kill $VCSIM_PID 2>/dev/null; exit 1; }
else
    echo ""
    echo "=== No port group found in vswitches output, skipping --portgroup test ==="
fi

# Clean up vcsim
echo ""
echo "=== Stopping vcsim ==="
kill $VCSIM_PID 2>/dev/null || true
wait $VCSIM_PID 2>/dev/null || true
echo "=== All integration tests passed ==="
