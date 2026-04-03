#!/usr/bin/env bash
# Health check script for SynapseOMS services.
# Usage: ./scripts/health-check.sh

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

check_service() {
  local name="$1"
  local url="$2"

  if curl -sf --max-time 5 "$url" > /dev/null 2>&1; then
    echo -e "  ${GREEN}[OK]${NC}  $name"
    return 0
  else
    echo -e "  ${RED}[FAIL]${NC} $name ($url)"
    return 1
  fi
}

echo "SynapseOMS Health Check"
echo "======================"
echo ""

failures=0

check_service "Gateway (REST)"     "http://localhost:8080/api/v1/health" || ((failures++))
check_service "Risk Engine (REST)" "http://localhost:8081/api/v1/health" || ((failures++))
check_service "Dashboard"          "http://localhost:3000"               || ((failures++))
check_service "ML Scorer"          "http://localhost:8090/health"        || ((failures++))

# Optional monitoring services
echo ""
echo "Monitoring (optional):"
check_service "Prometheus"  "http://localhost:9090/-/healthy" || true
check_service "Grafana"     "http://localhost:3001/api/health" || true

echo ""
if [ "$failures" -gt 0 ]; then
  echo -e "${RED}$failures core service(s) unhealthy.${NC}"
  exit 1
else
  echo -e "${GREEN}All core services healthy.${NC}"
fi
