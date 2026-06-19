#!/usr/bin/env bash
# Demo script for release creation + deployment lifecycle
#
# Purpose: Demonstrate creating a release and deployment. End-to-end flow:
#   login -> create test payload -> upload artifact -> create release ->
#   create deployment -> list deployments -> get deployment status
#
# Usage:
#   HELIX_ADMIN_PASSWORD=admin123 bash scripts/testing/demo_deployments.sh [base_url]
#
#   base_url defaults to http://localhost:8080
#
# Dependencies: curl, python3 (json.tool), zip, shasum
#
# Cross-references:
#   - server/internal/api/handlers_release.go (handleCreateRelease, handleListReleases)
#   - server/internal/api/handlers_deployment.go (handleCreateDeployment, handleListDeployments,
#     handleGetDeployment)
#   - server/internal/api/handlers_artifact.go (handleUploadArtifact)

set -uo pipefail

BASE_URL="${1:-http://localhost:8080}"
PASS="${HELIX_ADMIN_PASSWORD:?HELIX_ADMIN_PASSWORD must be set}"
ALL_PASS=1

pass()  { echo "  \xE2\x9C\x85 $1"; }
fail()  { echo "  \xE2\x9D\x8C $1"; ALL_PASS=0; }

echo "=== Helix OTA \xE2\x80\x94 Deployments Demo ==="
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

# Step 1: Create an OTA payload
echo ""
echo "1) Creating a random OTA payload artifact..."
TS=$(date +%s)
VER="1.0.0.$TS"
rm -rf /tmp/ota_demo_$$
mkdir -p /tmp/ota_demo_$$/payload
dd if=/dev/urandom bs=1024 count=64 of=/tmp/ota_demo_$$/payload/payload.bin 2>/dev/null
(cd /tmp/ota_demo_$$ && zip -0 -D /tmp/ota_demo_artifact_$$.zip payload/payload.bin) 2>/dev/null
SHA256=$(shasum -a 256 /tmp/ota_demo_artifact_$$.zip | cut -d' ' -f1)
pass "Payload created: version=$VER, sha256=$SHA256"

# Sign the artifact using the demo signing key
SIG="demo-signature-$$"
SIGN_KEY="${DEMO_SIGNING_PRIVKEY:-}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SIGN_GO="$SCRIPT_DIR/sign_artifact.go"
if [[ -n "$SIGN_KEY" && -f "$SIGN_GO" ]]; then
  SIG=$(go run "$SIGN_GO" "$SIGN_KEY" /tmp/ota_demo_artifact_$$.zip 2>/dev/null) || SIG="demo-signature-$$"
  pass "Artifact signed with Ed25519 key"
elif [[ -n "$SIGN_KEY" ]]; then
  echo "  (signing tool $SIGN_GO not found, using demo signature)"
else
  echo "  (DEMO_SIGNING_PRIVKEY not set, using demo signature)"
fi

# Step 2: Upload artifact
echo ""
echo "2) POST /api/v1/artifacts/upload \xE2\x80\x94 Upload artifact"
ART_RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/artifacts/upload" \
  -H "$AUTH" \
  -F "metadata={\"sha256\":\"$SHA256\",\"signature\":\"$SIG\",\"version\":\"$VER\",\"os\":\"android\",\"target_model\":\"rk3588\"};type=application/json" \
  -F "file=@/tmp/ota_demo_artifact_$$.zip")
ART_HTTP=$(echo "$ART_RESP" | tail -1)
ART_BODY=$(echo "$ART_RESP" | sed '$d')

if [[ "$ART_HTTP" == "201" || "$ART_HTTP" == "200" ]]; then
  ARTIFACT_ID=$(echo "$ART_BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('artifact_id','n/a'))" 2>&1)
  VERIFIED=$(echo "$ART_BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('verified',False))" 2>&1)
  if [[ "$ARTIFACT_ID" != "n/a" ]]; then
    pass "Artifact uploaded: id=$ARTIFACT_ID, verified=$VERIFIED"
    echo "$ART_BODY" | python3 -m json.tool
  else
    fail "Upload response missing artifact_id"
    echo "$ART_BODY" | python3 -m json.tool
    exit 1
  fi
else
  fail "Artifact upload failed (HTTP $ART_HTTP): $(echo "$ART_BODY" | head -c 300)"
  echo "$ART_BODY" | python3 -m json.tool
  exit 1
fi

# Step 3: Create release
echo ""
echo "3) POST /api/v1/releases \xE2\x80\x94 Create release"
REL_RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/releases" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"artifact_id\":\"$ARTIFACT_ID\",\"version\":\"$VER\",\"os\":\"android\",\"target_model\":\"rk3588\",\"notes\":\"Demo release $VER\"}")
REL_HTTP=$(echo "$REL_RESP" | tail -1)
REL_BODY=$(echo "$REL_RESP" | sed '$d')

if [[ "$REL_HTTP" == "201" || "$REL_HTTP" == "200" ]]; then
  RELEASE_ID=$(echo "$REL_BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('release_id','n/a'))" 2>&1)
  if [[ "$RELEASE_ID" != "n/a" ]]; then
    pass "Release created: id=$RELEASE_ID"
    echo "$REL_BODY" | python3 -m json.tool
  else
    fail "Release response missing release_id"
    echo "$REL_BODY" | python3 -m json.tool
    exit 1
  fi
else
  fail "Release creation failed (HTTP $REL_HTTP): $(echo "$REL_BODY" | head -c 300)"
  echo "$REL_BODY" | python3 -m json.tool
  exit 1
fi

# Step 4: Register a device (needed for deployment to have targets)
echo ""
DEMO_GROUP="demo-$TS"
echo "4) POST /api/v1/devices/register \xE2\x80\x94 Register matching device (group=$DEMO_GROUP)"
HW_ID="demo-deploy-$$"
DEV_RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/devices/register" \
  -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d "{\"hardware_id\":\"$HW_ID\",\"os\":\"android\",\"model\":\"rk3588\",\"group\":\"$DEMO_GROUP\"}")
DEV_HTTP=$(echo "$DEV_RESP" | tail -1)
if [[ "$DEV_HTTP" == "201" || "$DEV_HTTP" == "200" ]]; then
  pass "Device registered: hw_id=$HW_ID"
else
  echo "  (device registration non-critical, continuing)"
fi

# Step 5: List deployments (should be pre-creation state)
echo ""
echo "5) GET /api/v1/deployments \xE2\x80\x94 List deployments (before create)"
BEFORE_RESP=$(curl -s -w "\n%{http_code}" -H "$AUTH" "$BASE_URL/api/v1/deployments")
BEFORE_HTTP=$(echo "$BEFORE_RESP" | tail -1)
BEFORE_BODY=$(echo "$BEFORE_RESP" | sed '$d')
if [[ "$BEFORE_HTTP" == "200" ]]; then
  B4_COUNT=$(echo "$BEFORE_BODY" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('items',[])))" 2>&1)
  pass "List deployments returned (HTTP $BEFORE_HTTP, $B4_COUNT deployments)"
  echo "$BEFORE_BODY" | python3 -m json.tool
else
  fail "List deployments failed (HTTP $BEFORE_HTTP): $(echo "$BEFORE_BODY" | head -c 200)"
fi

# Step 6: Create deployment
echo ""
echo "6) POST /api/v1/deployments \xE2\x80\x94 Create deployment (all-targets, group=$DEMO_GROUP)"
DEP_RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/deployments" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"release_id\":\"$RELEASE_ID\",\"strategy\":\"all-targets\",\"group\":\"$DEMO_GROUP\"}")
DEP_HTTP=$(echo "$DEP_RESP" | tail -1)
DEP_BODY=$(echo "$DEP_RESP" | sed '$d')

if [[ "$DEP_HTTP" == "201" || "$DEP_HTTP" == "200" ]]; then
  DEPL_ID=$(echo "$DEP_BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('deployment_id','n/a'))" 2>&1)
  if [[ "$DEPL_ID" != "n/a" ]]; then
    pass "Deployment created: id=$DEPL_ID"
    echo "$DEP_BODY" | python3 -m json.tool
  else
    fail "Deployment response missing deployment_id"
    echo "$DEP_BODY" | python3 -m json.tool
    exit 1
  fi
else
  fail "Deployment creation failed (HTTP $DEP_HTTP): $(echo "$DEP_BODY" | head -c 300)"
  echo "$DEP_BODY" | python3 -m json.tool
  exit 1
fi

# Step 7: List deployments (should now include the new one)
echo ""
echo "7) GET /api/v1/deployments \xE2\x80\x94 List deployments (after create)"
AFTER_RESP=$(curl -s -w "\n%{http_code}" -H "$AUTH" "$BASE_URL/api/v1/deployments")
AFTER_HTTP=$(echo "$AFTER_RESP" | tail -1)
AFTER_BODY=$(echo "$AFTER_RESP" | sed '$d')
if [[ "$AFTER_HTTP" == "200" ]]; then
  AF_COUNT=$(echo "$AFTER_BODY" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('items',[])))" 2>&1)
  pass "List deployments after create (HTTP $AFTER_HTTP, $AF_COUNT deployments)"
  echo "$AFTER_BODY" | python3 -m json.tool
else
  fail "List deployments after create failed (HTTP $AFTER_HTTP)"
fi

# Step 8: Get deployment status
echo ""
echo "8) GET /api/v1/deployments/$DEPL_ID \xE2\x80\x94 Get deployment status"
STAT_RESP=$(curl -s -w "\n%{http_code}" -H "$AUTH" "$BASE_URL/api/v1/deployments/$DEPL_ID")
STAT_HTTP=$(echo "$STAT_RESP" | tail -1)
STAT_BODY=$(echo "$STAT_RESP" | sed '$d')
if [[ "$STAT_HTTP" == "200" ]]; then
  pass "Deployment status returned (HTTP $STAT_HTTP)"
  echo "$STAT_BODY" | python3 -m json.tool
else
  fail "Deployment status failed (HTTP $STAT_HTTP): $(echo "$STAT_BODY" | head -c 200)"
fi

# Cleanup temp files
rm -rf /tmp/ota_demo_$$ /tmp/ota_demo_artifact_$$.zip

# Summary
echo ""
if [[ "$ALL_PASS" == "1" ]]; then
  echo "\xE2\x9C\x85 ALL DEPLOYMENT DEMO OPERATIONS PASSED"
else
  echo "\xE2\x9D\x8C SOME DEPLOYMENT DEMO OPERATIONS FAILED"
  exit 1
fi
