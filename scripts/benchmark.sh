#!/bin/bash
# Benchmark million-rps API. Usage: ./scripts/benchmark.sh [base_url] [duration] [concurrency]
# Examples:
#   ./scripts/benchmark.sh                          # http://localhost:8080, 30s, 200 conn
#   ./scripts/benchmark.sh http://localhost:8080 60 500
#   ./scripts/benchmark.sh http://lb:8080 30 200   # when using docker-compose scale

BASE_URL="${1:-http://localhost:8080}"
DURATION="${2:-30}"
CONCURRENCY="${3:-200}"
ENDPOINT="${BASE_URL}/todos?limit=100"

echo "=== million-rps benchmark ==="
echo "Target: $ENDPOINT"
echo "Duration: ${DURATION}s, Concurrency: $CONCURRENCY"
echo ""

echo "1. Warming cache..."
curl -sf "$ENDPOINT" > /dev/null && echo "   OK" || echo "   WARN: curl failed"
echo ""

echo "2. Running hey (timeout 120s to avoid timeouts under load)..."
if command -v hey &> /dev/null; then
  hey -z "${DURATION}s" -c "$CONCURRENCY" -t 120 -m GET "$ENDPOINT"
else
  echo "   hey not found. Install: go install github.com/rakyll/hey@latest"
  echo "   Or use autocannon: autocannon -c $CONCURRENCY -d $DURATION -t 120 \"$ENDPOINT\""
fi
