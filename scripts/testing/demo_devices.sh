#!/usr/bin/env bash
# Demo script for device registration + status + list + by-hardware API
#
# Purpose: Demonstrate device registration, status check, list devices, and
# hardware ID reverse lookup endpoints. All operations verified with real
# captured HTTP responses.
#
# Usage:
#   HELIX_ADMIN_PASSWORD=admin123 bash scripts/testing/demo_devices.sh [base_url]
#
#   base_url defaults to http://localhost:8080
#
# Dependencies: curl, python3 (json.tool)
#
# Cross-references:
#   - server/internal/api/handlers_device.go (handleRegisterDevice, handleDeviceStatus,
#     handleListDevices, handleDeviceByHardware)
#   - tests/e2e/challenge_operational.sh

set -uo pipefail

BASE_URL="${1:-http://localhost:8080}"
PASS="${HELIX_ADMIN_PASSWORD:?HELIX_ADMIN_PASSWORD must be set}"
ALL_PASS=1

pass()  { echo "  \xE2\x9C\x85 $1"; }
fail()  { echo "  \xE2\x9D\x8C $1"; ALL_PASS=0; }

echo "=== Helix OTA \xE2\x80\x94 Devices Demo ==="
echo ""

# Step 0: Login
echo "0) POST /api/v1/auth/login \xE2\x80\x94 Authenticate"
RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"admin@helix.example\",\"password\":\"$PASS\"}")
HTTP_CODE=$(echo "$RESP" | tail -1)
BODY=$(echo "$RESP" | sed '$d')
if [[ "$HTTP_CODE" != "200" ]]; then
  fail "Login failed (HTTP $HTTP_CODE): $(echo "$BODY" | head -c 200)"
  exit 1
fi
TOKEN=$(echo "$BODY" | python3 -c "import sys,json; t=json.load(sys.stdin); print(t['access_token'])" 2>&1) || {
  fail "Login response has no access_token"; exit 1; }
pass "Login OK (token: ${TOKEN:0:20}...)"
AUTH="Authorization: Bearer $TOKEN"

# Step 1: Register device (unique hardware_id per run)
HARDWARE_ID="demo-$(date +%s)-$$"
echo ""
echo "1) POST /api/v1/devices/register \xE2\x80\x94 Register device (hw_id=$HARDWARE_ID)"
REG_RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/devices/register" \
  -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d "{\"hardware_id\":\"$HARDWARE_ID\",\"os\":\"android\",\"model\":\"rk3588\"}")
REG_HTTP=$(echo "$REG_RESP" | tail -1)
REG_BODY=$(echo "$REG_RESP" | sed '$d')

if [[ "$REG_HTTP" == "201" ]]; then
  DEVICE_ID=$(echo "$REG_BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('device_id','n/a'))" 2>&1)
  DEVICE_TOKEN=$(echo "$REG_BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('device_token','n/a'))" 2>&1)
  if [[ "$DEVICE_ID" != "n/a" && "$DEVICE_TOKEN" != "n/a" ]]; then
    pass "Device registered: id=$DEVICE_ID, token=${DEVICE_TOKEN:0:20}..."
    echo "$REG_BODY" | python3 -m json.tool
  else
    fail "Registration response missing device_id or device_token"
    echo "$REG_BODY" | python3 -m json.tool
    exit 1
  fi
elif [[ "$REG_HTTP" == "200" ]]; then
  DEVICE_ID=$(echo "$REG_BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('device_id','n/a'))" 2>&1)
  pass "Device already exists (idempotent): id=$DEVICE_ID"
else
  fail "Registration failed (HTTP $REG_HTTP): $(echo "$REG_BODY" | head -c 300)"
  exit 1
fi

# Step 2: Device status
echo ""
echo "2) GET /api/v1/devices/$DEVICE_ID/status \xE2\x80\x94 Device status"
STATUS_RESP=$(curl -s -w "\n%{http_code}" -H "$AUTH" "$BASE_URL/api/v1/devices/$DEVICE_ID/status")
STATUS_HTTP=$(echo "$STATUS_RESP" | tail -1)
STATUS_BODY=$(echo "$STATUS_RESP" | sed '$d')
if [[ "$STATUS_HTTP" == "200" ]]; then
  pass "Device status returned (HTTP $STATUS_HTTP)"
  echo "$STATUS_BODY" | python3 -m json.tool
else
  fail "Device status failed (HTTP $STATUS_HTTP): $(echo "$STATUS_BODY" | head -c 200)"
fi

# Step 3: List devices
echo ""
echo "3) GET /api/v1/devices \xE2\x80\x94 List devices"
LIST_RESP=$(curl -s -w "\n%{http_code}" -H "$AUTH" "$BASE_URL/api/v1/devices")
LIST_HTTP=$(echo "$LIST_RESP" | tail -1)
LIST_BODY=$(echo "$LIST_RESP" | sed '$d')
if [[ "$LIST_HTTP" == "200" ]]; then
  COUNT=$(echo "$LIST_BODY" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('items',[])))" 2>&1)
  pass "List devices returned (HTTP $LIST_HTTP, $COUNT devices)"
  echo "$LIST_BODY" | python3 -m json.tool | head -30
  echo "  ... (truncated)"
else
  fail "List devices failed (HTTP $LIST_HTTP): $(echo "$LIST_BODY" | head -c 200)"
fi

# Step 4: Hardware lookup
echo ""
echo "4) GET /api/v1/devices/by-hardware/$HARDWARE_ID \xE2\x80\x94 Hardware reverse lookup"
HW_RESP=$(curl -s -w "\n%{http_code}" -H "$AUTH" "$BASE_URL/api/v1/devices/by-hardware/$HARDWARE_ID")
HW_HTTP=$(echo "$HW_RESP" | tail -1)
HW_BODY=$(echo "$HW_RESP" | sed '$d')
if [[ "$HW_HTTP" == "200" ]]; then
  pass "Hardware lookup returned (HTTP $HW_HTTP)"
  echo "$HW_BODY" | python3 -m json.tool
else
  fail "Hardware lookup failed (HTTP $HW_HTTP): $(echo "$HW_BODY" | head -c 200)"
fi

# Summary
echo ""
if [[ "$ALL_PASS" == "1" ]]; then
  echo "\xE2\x9C\x85 ALL DEVICE DEMO OPERATIONS PASSED"
else
  echo "\xE2\x9D\x8C SOME DEVICE DEMO OPERATIONS FAILED"
  exit 1
fi
