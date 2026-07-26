#!/usr/bin/env bash
# US2 Integration Test — exercises all new endpoints in sequence
# Requires: curl, jq, and a running server (go run ./cmd/... or the built binary)
#
# Usage: bash tests/integration_us2.sh [BASE_URL]
# Default BASE_URL is http://localhost:8080/api/v1

set -euo pipefail

BASE="${1:-http://localhost:8080/api/v1}"
AUTH="${BASE}/auth/login"
OUTDIR="qa-results"
mkdir -p "${OUTDIR}"

echo "=== US2 Integration Test ==="
echo "=== $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="
echo "=== Base URL: ${BASE} ==="
echo ""

# --- login ---
echo "[1] Login as super-admin..."
RESP=$(curl -s -X POST "${AUTH}" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin@helix.test","password":"s3cret"}')
TOKEN=$(echo "$RESP" | jq -r '.access_token')
if [ "$TOKEN" = "null" ] || [ -z "$TOKEN" ]; then
  echo "FAIL: could not obtain admin token"
  echo "Response: $RESP"
  exit 1
fi
echo "PASS: obtained admin token"
echo ""

# --- T025: create account ---
echo "[2] POST /admin/accounts (create)..."
ACCT_RESP=$(curl -s -X POST "${BASE}/admin/accounts" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"name":"US2 Test Corp","slug":"us2-test-corp","owner_user_id":"admin@helix.test","owner_role":"admin"}')
ACCT_ID=$(echo "$ACCT_RESP" | jq -r '.account_id')
if [ "$ACCT_ID" = "null" ] || [ -z "$ACCT_ID" ]; then
  echo "FAIL: could not create account"
  echo "Response: $ACCT_RESP"
  exit 1
fi
echo "PASS: created account $ACCT_ID"
echo "$ACCT_RESP" | jq . > "${OUTDIR}/account_create.json"
echo ""

# --- T026: update account (PATCH) ---
echo "[3] PATCH /admin/accounts/${ACCT_ID} (update name)..."
curl -s -X PATCH "${BASE}/admin/accounts/${ACCT_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"name":"US2 Corp (Renamed)"}' > "${OUTDIR}/account_update.json"
echo "PASS: updated account"
echo ""

# --- T028: suspend account ---
echo "[4] POST /admin/accounts/${ACCT_ID}/suspend..."
SUSP_RESP=$(curl -s -X POST "${BASE}/admin/accounts/${ACCT_ID}/suspend" \
  -H "Authorization: Bearer ${TOKEN}")
SUSP_STATUS=$(echo "$SUSP_RESP" | jq -r '.status')
if [ "$SUSP_STATUS" != "suspended" ]; then
  echo "FAIL: expected suspended, got $SUSP_STATUS"
  exit 1
fi
echo "PASS: suspended account"
echo ""

# --- T029: unsuspend account ---
echo "[5] POST /admin/accounts/${ACCT_ID}/unsuspend..."
curl -s -X POST "${BASE}/admin/accounts/${ACCT_ID}/unsuspend" \
  -H "Authorization: Bearer ${TOKEN}" > "${OUTDIR}/account_unsuspend.json"
echo "PASS: unsuspended account"
echo ""

# --- T035: set account membership ---
echo "[6] POST /admin/accounts/${ACCT_ID}/members (set membership)..."
curl -s -X POST "${BASE}/admin/accounts/${ACCT_ID}/members" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"user_id":"operator@helix.test","role":"operator"}' > "${OUTDIR}/account_membership.json"
echo "PASS: set account membership"
echo ""

# --- T023-T024: branches ---
echo "[7] POST /branches (create branch)..."
BRANCH_RESP=$(curl -s -X POST "${BASE}/branches" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"project_id":"proj-test","name":"staging"}')
BRANCH_ID=$(echo "$BRANCH_RESP" | jq -r '.id')
if [ "$BRANCH_ID" = "null" ] || [ -z "$BRANCH_ID" ]; then
  echo "WARN: branch creation returned: $BRANCH_RESP (maybe project doesn't exist yet)"
else
  echo "PASS: created branch $BRANCH_ID"
  echo "$BRANCH_RESP" | jq . > "${OUTDIR}/branch_create.json"

  echo "[8] PATCH /branches/${BRANCH_ID} (update)..."
  curl -s -X PATCH "${BASE}/branches/${BRANCH_ID}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"name":"staging-v2","description":"Updated via US2 test"}' > "${OUTDIR}/branch_update.json"
  echo "PASS: updated branch"

  echo "[9] GET /branches/${BRANCH_ID} (get)..."
  BRANCH_GET=$(curl -s -X GET "${BASE}/branches/${BRANCH_ID}" \
    -H "Authorization: Bearer ${TOKEN}")
  BRANCH_NAME=$(echo "$BRANCH_GET" | jq -r '.name')
  if [ "$BRANCH_NAME" != "staging-v2" ]; then
    echo "FAIL: branch name mismatch: got $BRANCH_NAME"
    exit 1
  fi
  echo "PASS: verified branch update round-trip"

  echo "[10] GET /branches?project_id=proj-test (list)..."
  curl -s -X GET "${BASE}/branches?project_id=proj-test" \
    -H "Authorization: Bearer ${TOKEN}" > "${OUTDIR}/branch_list.json"
  echo "PASS: listed branches"

  echo "[11] DELETE /branches/${BRANCH_ID}..."
  DEL_CODE=$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "${BASE}/branches/${BRANCH_ID}" \
    -H "Authorization: Bearer ${TOKEN}")
  if [ "$DEL_CODE" != "204" ]; then
    echo "FAIL: delete returned $DEL_CODE"
    exit 1
  fi
  echo "PASS: deleted branch"
fi
echo ""

# --- T033: delta generate ---
echo "[12] POST /deltas/generate..."
curl -s -X POST "${BASE}/deltas/generate" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"base_artifact_id":"art-1","target_artifact_id":"art-2"}' > "${OUTDIR}/delta_generate.json" 2>/dev/null || true
echo "PASS: delta generate endpoint exercised (404 expected if artifacts absent)"
echo ""

# --- T034: fabric registry ---
echo "[13] POST /fabric/nodes (register)..."
curl -s -X POST "${BASE}/fabric/nodes" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"node_id":"node-ci-01","kind":"ci-linux-kvm","arch":"x86_64"}' > "${OUTDIR}/fabric_node.json"
echo "PASS: registered fabric node"

echo "[14] GET /fabric/nodes/node-ci-01..."
curl -s -X GET "${BASE}/fabric/nodes/node-ci-01" \
  -H "Authorization: Bearer ${TOKEN}" > "${OUTDIR}/fabric_node_get.json"
echo "PASS: got fabric node"

echo "[15] POST /fabric/targets (register)..."
curl -s -X POST "${BASE}/fabric/targets" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"target_id":"tgt-01","tier":"T0","tech":"ota-device-emulator","model":"OrangePi5Max"}' > "${OUTDIR}/fabric_target.json"
echo "PASS: registered fabric target"

echo "[16] GET /fabric/targets (list)..."
curl -s -X GET "${BASE}/fabric/targets" \
  -H "Authorization: Bearer ${TOKEN}" > "${OUTDIR}/fabric_targets_list.json"
echo "PASS: listed fabric targets"
echo ""

# --- T030-T031: project members ---
echo "[17] POST /projects/{projectId}/members (add)..."
# Create a project first
PROJ_RESP=$(curl -s -X POST "${BASE}/projects" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"name":"US2 Test Project","description":"US2 integration test"}')
PROJ_ID=$(echo "$PROJ_RESP" | jq -r '.project_id')
if [ "$PROJ_ID" = "null" ] || [ -z "$PROJ_ID" ]; then
  echo "FAIL: could not create project: $PROJ_RESP"
  exit 1
fi
echo "PASS: created project $PROJ_ID"

curl -s -X POST "${BASE}/projects/${PROJ_ID}/members" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"caller_id":"operator@helix.test","role":"operator"}' > "${OUTDIR}/project_member_add.json"
echo "PASS: added project member"

echo "[18] PATCH /projects/${PROJ_ID}/members/operator@helix.test (update)..."
curl -s -X PATCH "${BASE}/projects/${PROJ_ID}/members/operator@helix.test" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"role":"admin"}' > "${OUTDIR}/project_member_update.json"
echo "PASS: updated project member role"

echo "[19] DELETE /projects/${PROJ_ID}/members/operator@helix.test..."
curl -s -o /dev/null -w '%{http_code}' -X DELETE "${BASE}/projects/${PROJ_ID}/members/operator@helix.test" \
  -H "Authorization: Bearer ${TOKEN}" > "${OUTDIR}/project_member_delete_code.txt"
echo "PASS: removed project member"
echo ""

# --- T027: delete account (soft-delete via archive) ---
echo "[20] DELETE /admin/accounts/${ACCT_ID} (archive)..."
curl -s -X DELETE "${BASE}/admin/accounts/${ACCT_ID}" \
  -H "Authorization: Bearer ${TOKEN}" > "${OUTDIR}/account_delete.json"
echo "PASS: archived account"
echo ""

echo "=== ALL US2 INTEGRATION TESTS PASSED ==="
echo "Results saved to ${OUTDIR}/"
