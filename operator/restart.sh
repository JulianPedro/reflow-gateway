#!/bin/bash
# Restart the operator: rebuild, kill old process, start new one

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
KUBECONFIG="${ROOT_DIR}/kubeconfig.yaml"

# Build
echo "Building operator..."
cd "$SCRIPT_DIR"
go build -o bin/manager cmd/main.go

# Kill old process
OLD_PID=$(pgrep -f "bin/manager.*metrics-bind-address" 2>/dev/null || true)
if [ -n "$OLD_PID" ]; then
  echo "Killing old operator (PID $OLD_PID)..."
  kill "$OLD_PID" 2>/dev/null
  sleep 1
fi

# Apply CRD
echo "Applying CRD..."
kubectl apply -f "$SCRIPT_DIR/config/crd/bases/mcp.reflow.io_mcpinstances.yaml"

# Start
echo "Starting operator..."
KUBECONFIG="$KUBECONFIG" "$SCRIPT_DIR/bin/manager" \
  -metrics-bind-address=:9090 \
  -health-probe-bind-address=:9091
