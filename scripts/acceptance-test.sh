#!/usr/bin/env bash
# acceptance-test.sh — Phase 1 end-to-end acceptance test using curl.
#
# Usage:
#   ./scripts/acceptance-test.sh [GATEWAY_URL]
#
# Default GATEWAY_URL: http://localhost:8080

set -euo pipefail

GATEWAY_URL="${1:-${GATEWAY_URL:-http://localhost:8080}}"
GATEWAY_URL="${GATEWAY_URL%/}"

PASS=0
FAIL=0

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BOLD='\033[1m'
NC='\033[0m' # No Color

pass() {
  PASS=$((PASS + 1))
  echo -e "  ${GREEN}PASS${NC} $1"
}

fail() {
  FAIL=$((FAIL + 1))
  echo -e "  ${RED}FAIL${NC} $1"
}

echo -e "${BOLD}=== Phase 1 Acceptance Test ===${NC}"
echo -e "Gateway: ${GATEWAY_URL}"
echo ""

# -------------------------------------------------------
# Step 1: GET /api/v1/instruments — verify 6 instruments
# -------------------------------------------------------
echo -e "${BOLD}Step 1: GET /api/v1/instruments${NC}"
INSTRUMENTS=$(curl -sf "${GATEWAY_URL}/api/v1/instruments" 2>/dev/null) || {
  fail "could not reach gateway at ${GATEWAY_URL}/api/v1/instruments"
  echo -e "\n${RED}Gateway not reachable. Is the Docker Compose stack running?${NC}"
  exit 1
}

COUNT=$(echo "${INSTRUMENTS}" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "${INSTRUMENTS}" | jq 'length' 2>/dev/null || echo "?")
if [ "${COUNT}" = "6" ]; then
  pass "6 instruments found"
else
  fail "expected 6 instruments, got ${COUNT}"
fi

# -------------------------------------------------------
# Step 2: Health check
# -------------------------------------------------------
echo -e "${BOLD}Step 2: Health check${NC}"
HEALTH=$(curl -sf "${GATEWAY_URL}/api/v1/health" 2>/dev/null) || HEALTH=""
if echo "${HEALTH}" | grep -q '"ok"'; then
  pass "health endpoint returns ok"
else
  fail "health endpoint did not return ok"
fi

# -------------------------------------------------------
# Step 3: POST /api/v1/orders — market buy 10 AAPL
# -------------------------------------------------------
echo -e "${BOLD}Step 3: POST /api/v1/orders — market buy 10 AAPL${NC}"
ORDER_RESP=$(curl -sf -w "\n%{http_code}" -X POST \
  -H "Content-Type: application/json" \
  -d '{"instrument_id":"AAPL","side":"buy","type":"market","quantity":"10","price":"0"}' \
  "${GATEWAY_URL}/api/v1/orders" 2>/dev/null) || ORDER_RESP=""

HTTP_CODE=$(echo "${ORDER_RESP}" | tail -1)
ORDER_BODY=$(echo "${ORDER_RESP}" | sed '$d')

if [ "${HTTP_CODE}" = "201" ]; then
  pass "201 Created"
else
  fail "expected 201, got ${HTTP_CODE}"
fi

# Extract order ID and status
ORDER_ID=$(echo "${ORDER_BODY}" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || echo "${ORDER_BODY}" | jq -r '.id' 2>/dev/null || echo "")
ORDER_STATUS=$(echo "${ORDER_BODY}" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || echo "${ORDER_BODY}" | jq -r '.status' 2>/dev/null || echo "")

if [ "${ORDER_STATUS}" = "new" ]; then
  pass "initial status is 'new'"
else
  fail "expected status 'new', got '${ORDER_STATUS}'"
fi

if [ -z "${ORDER_ID}" ]; then
  fail "order ID is empty — cannot continue"
  echo -e "\n${RED}Cannot proceed without order ID.${NC}"
  exit 1
fi
echo -e "  Order ID: ${ORDER_ID}"

# -------------------------------------------------------
# Step 4: Wait for order to reach 'filled' status
# -------------------------------------------------------
echo -e "${BOLD}Step 4: Wait for order to reach 'filled' (polling, max 10s)${NC}"
FILLED=false
for i in $(seq 1 20); do
  sleep 0.5
  POLL=$(curl -sf "${GATEWAY_URL}/api/v1/orders/${ORDER_ID}" 2>/dev/null) || continue
  STATUS=$(echo "${POLL}" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || echo "${POLL}" | jq -r '.status' 2>/dev/null || echo "")
  echo -e "    poll ${i}: status=${STATUS}"
  if [ "${STATUS}" = "filled" ]; then
    FILLED=true
    break
  fi
done

if [ "${FILLED}" = "true" ]; then
  pass "order reached 'filled' status"
else
  fail "order did not reach 'filled' within 10s"
fi

# -------------------------------------------------------
# Step 5: Verify filled order details
# -------------------------------------------------------
echo -e "${BOLD}Step 5: GET /api/v1/orders/${ORDER_ID} — verify fill details${NC}"
FINAL_ORDER=$(curl -sf "${GATEWAY_URL}/api/v1/orders/${ORDER_ID}" 2>/dev/null) || FINAL_ORDER=""

FILLED_QTY=$(echo "${FINAL_ORDER}" | python3 -c "import sys,json; print(json.load(sys.stdin)['filled_quantity'])" 2>/dev/null || echo "${FINAL_ORDER}" | jq -r '.filled_quantity' 2>/dev/null || echo "")
FILLS_COUNT=$(echo "${FINAL_ORDER}" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('fills',[])))" 2>/dev/null || echo "${FINAL_ORDER}" | jq '.fills | length' 2>/dev/null || echo "0")

if [ "${FILLED_QTY}" = "10" ]; then
  pass "filled_quantity = 10"
else
  fail "expected filled_quantity '10', got '${FILLED_QTY}'"
fi

if [ "${FILLS_COUNT}" -gt 0 ] 2>/dev/null; then
  pass "fills array has ${FILLS_COUNT} fill(s)"
else
  fail "fills array is empty"
fi

# -------------------------------------------------------
# Step 6: GET /api/v1/positions/AAPL — quantity >= 10
# -------------------------------------------------------
echo -e "${BOLD}Step 6: GET /api/v1/positions/AAPL — verify position${NC}"
POS_RESP=$(curl -sf "${GATEWAY_URL}/api/v1/positions/AAPL" 2>/dev/null) || POS_RESP=""

POS_QTY=$(echo "${POS_RESP}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['quantity'] if isinstance(d,dict) else d[0]['quantity'])" 2>/dev/null || echo "${POS_RESP}" | jq -r 'if type=="array" then .[0].quantity else .quantity end' 2>/dev/null || echo "")

if [ -n "${POS_QTY}" ]; then
  # Check quantity is at least 10 (could be higher if tests ran before)
  QTY_INT=$(echo "${POS_QTY}" | python3 -c "import sys; print(int(float(sys.stdin.read())))" 2>/dev/null || echo "0")
  if [ "${QTY_INT}" -ge 10 ] 2>/dev/null; then
    pass "AAPL position quantity = ${POS_QTY} (>= 10)"
  else
    fail "expected position quantity >= 10, got '${POS_QTY}'"
  fi
else
  fail "could not read AAPL position"
fi

# -------------------------------------------------------
# Step 7: WebSocket check (optional — requires wscat)
# -------------------------------------------------------
echo -e "${BOLD}Step 7: WebSocket check (optional)${NC}"
if command -v wscat &>/dev/null; then
  WS_URL=$(echo "${GATEWAY_URL}" | sed 's|^http|ws|')/ws/orders
  echo -e "  Connecting to ${WS_URL} for 3 seconds..."
  WS_OUT=$(timeout 3 wscat -c "${WS_URL}" 2>/dev/null || true)
  if [ -n "${WS_OUT}" ]; then
    pass "received WebSocket message(s)"
  else
    echo -e "  ${YELLOW}SKIP${NC} no messages received (may need an active order)"
  fi
else
  echo -e "  ${YELLOW}SKIP${NC} wscat not installed (npm install -g wscat)"
fi

# -------------------------------------------------------
# Summary
# -------------------------------------------------------
echo ""
echo -e "${BOLD}=== Results ===${NC}"
echo -e "  ${GREEN}Passed: ${PASS}${NC}"
echo -e "  ${RED}Failed: ${FAIL}${NC}"
echo ""

if [ "${FAIL}" -gt 0 ]; then
  echo -e "${RED}ACCEPTANCE TEST FAILED${NC}"
  exit 1
else
  echo -e "${GREEN}ACCEPTANCE TEST PASSED${NC}"
  exit 0
fi
