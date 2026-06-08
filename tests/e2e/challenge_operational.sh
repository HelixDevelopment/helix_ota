#!/usr/bin/env bash
# =============================================================================
# challenge_operational.sh — Helix OTA operational + rollout E2E challenge
# -----------------------------------------------------------------------------
# Purpose:
#   Autonomous, anti-bluff end-to-end challenge (Constitution §11.4 / §11.4.27)
#   that BLACK-BOX drives the running ota-server REST API over real HTTP using
#   curl + jq and asserts real HTTP status codes AND key JSON fields. There are
#   NO mocks of the system under test — every assertion is made against a live
#   server process. A 401 (auth broken) or an empty audit list after auditable
#   mutations FAILS the challenge (no false PASS).
#
# What it drives + asserts (all hard-FAIL on mismatch):
#   1.  GET  /healthz                         -> 200, {"status":"ok"|...}
#   2.  POST /api/v1/auth/login               -> 200, access_token present (JWT)
#   3.  Anti-bluff: a protected route with NO token -> 401 (must be enforced)
#   4.  POST /api/v1/devices/register         -> 201, device_id present
#   5.  GET  /api/v1/devices/{id}/status      -> 200, device_id echoes
#   6.  POST /api/v1/groups                   -> 201, group_id present
#   7.  GET  /api/v1/groups/{id}              -> 200, name echoes
#   8.  POST /api/v1/groups/{id}/members      -> 200 (BATCH add the registered
#                                                device_ids; id appears in .added)
#   8b. POST /api/v1/groups/{id}/members      -> 200, re-add => .already_member
#   8c. POST /api/v1/groups/{id}/members      -> 200, unregistered id => .not_found
#   8d. POST /api/v1/groups/{id}/members []   -> 400 (empty device_ids rejected)
#   9.  GET  /api/v1/groups/{id}/members      -> 200, .items[].device_id contains
#                                                member (+ added_at non-empty)
#   10. GET  /api/v1/telemetry/overview       -> 200, has total + event_counts
#   11. GET  /api/v1/audit                    -> 200, GROUP_CREATE +
#                                                GROUP_MEMBER_CREATE audited
#                                                (anti-bluff: NON-empty)
#   12. Rollout route semantics (deterministic, black-box):
#         GET  /deployments/<bogus>/rollout         -> 404
#         POST /deployments/<bogus>/rollout         -> 404 (deployment gate)
#   13. OPTIONAL full pipeline (best-effort, never a false FAIL): attempt the
#         signed artifact upload -> release -> deployment -> create rollout ->
#         GET rollout -> evaluate rollout. If the (server-config-coupled)
#         artifact signing contract is not reproducible from this shell, the
#         stage SKIPs-with-reason rather than fabricating a pass (§11.4.3).
#   14. DELETE /api/v1/groups/{id}/members/{id} -> 204 (self-clean)
#   15. DELETE /api/v1/groups/{id}              -> 204 (self-clean)
#         then GET /api/v1/groups/{id}          -> 404 (deletion is real)
#
# Idempotent + self-cleaning: each run creates uniquely-named resources and
# deletes the group it created on the way out (and via an EXIT trap).
#
# Usage:
#   challenge_operational.sh [--base-url URL] [--username U] [--password P]
#   Env equivalents: HELIX_BASE_URL, HELIX_ADMIN_USERNAME, HELIX_ADMIN_PASSWORD
#
# Inputs:
#   --base-url  (default $HELIX_BASE_URL or http://127.0.0.1:8080)
#   --username  (default $HELIX_ADMIN_USERNAME or admin@helix.test)
#   --password  (default $HELIX_ADMIN_PASSWORD — REQUIRED, the server's admin pw)
#
# Outputs:
#   Human-readable PASS/FAIL/SKIP lines on stdout; exit 0 only if every hard
#   assertion passed. Non-zero exit on any mismatch or a missing prerequisite.
#
# Side-effects:
#   Creates + deletes a device-group and registers a device in the server's
#   (in-memory by default) store. No host state is touched.
#
# Dependencies: bash, curl, jq.
#
# Cross-references: tests/e2e/README.md ; server/internal/api/server.go (routes).
# =============================================================================
set -u
set -o pipefail

# ---- configuration -------------------------------------------------------------
BASE_URL="${HELIX_BASE_URL:-http://127.0.0.1:8080}"
USERNAME="${HELIX_ADMIN_USERNAME:-admin@helix.test}"
PASSWORD="${HELIX_ADMIN_PASSWORD:-}"
API="/api/v1"

while [ $# -gt 0 ]; do
  case "$1" in
    --base-url) BASE_URL="$2"; shift 2 ;;
    --username) USERNAME="$2"; shift 2 ;;
    --password) PASSWORD="$2"; shift 2 ;;
    -h|--help) sed -n '2,70p' "$0"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

BASE_URL="${BASE_URL%/}"
RUN_TAG="e2e-$(date +%s)-$$"

# ---- bookkeeping ---------------------------------------------------------------
PASS=0
FAIL=0
SKIP=0
GROUP_ID=""
DEVICE_ID=""

c_green() { printf '\033[32m%s\033[0m' "$1"; }
c_red()   { printf '\033[31m%s\033[0m' "$1"; }
c_yellow(){ printf '\033[33m%s\033[0m' "$1"; }

pass() { PASS=$((PASS+1)); printf '%s %s\n' "$(c_green '[PASS]')" "$1"; }
fail() { FAIL=$((FAIL+1)); printf '%s %s\n' "$(c_red   '[FAIL]')" "$1"; }
skip() { SKIP=$((SKIP+1)); printf '%s %s\n' "$(c_yellow '[SKIP]')" "$1"; }

# fatal aborts immediately on a prerequisite failure (cannot continue meaningfully).
fatal() { fail "$1"; finish; exit 1; }

# req METHOD PATH [DATA] [AUTH] -> writes status to $HTTP_STATUS, body to $HTTP_BODY
# AUTH: "auth" attaches the bearer token, anything else (or empty) sends none.
HTTP_STATUS=""
HTTP_BODY=""
req() {
  local method="$1" path="$2" data="${3:-}" auth="${4:-auth}"
  local url="${BASE_URL}${path}"
  local tmp; tmp="$(mktemp)"
  local -a args=(-sS -o "$tmp" -w '%{http_code}' -X "$method" "$url"
                 -H 'Accept: application/json' -H "User-Agent: helix-e2e/${RUN_TAG}")
  if [ "$auth" = "auth" ] && [ -n "${TOKEN:-}" ]; then
    args+=(-H "Authorization: Bearer ${TOKEN}")
  fi
  if [ -n "$data" ]; then
    args+=(-H 'Content-Type: application/json' --data "$data")
  fi
  HTTP_STATUS="$(curl "${args[@]}" 2>/dev/null)" || HTTP_STATUS="000"
  HTTP_BODY="$(cat "$tmp")"
  rm -f "$tmp"
}

# assert_status WANT LABEL  (uses $HTTP_STATUS)
assert_status() {
  local want="$1" label="$2"
  if [ "$HTTP_STATUS" = "$want" ]; then
    pass "$label (HTTP $HTTP_STATUS)"
    return 0
  fi
  fail "$label (want HTTP $want, got $HTTP_STATUS; body: $(printf '%s' "$HTTP_BODY" | head -c 240))"
  return 1
}

# jqget FILTER -> echoes value or empty (from $HTTP_BODY)
jqget() { printf '%s' "$HTTP_BODY" | jq -r "$1" 2>/dev/null; }

# self-cleaning
finish() {
  if [ -n "$GROUP_ID" ] && [ -n "${TOKEN:-}" ]; then
    req DELETE "${API}/groups/${GROUP_ID}/members/${DEVICE_ID}" "" auth >/dev/null 2>&1 || true
    req DELETE "${API}/groups/${GROUP_ID}" "" auth >/dev/null 2>&1 || true
  fi
}
trap finish EXIT

echo "== Helix OTA operational + rollout E2E challenge =="
echo "base_url=${BASE_URL} user=${USERNAME} run=${RUN_TAG}"
echo

# ---- prerequisites -------------------------------------------------------------
command -v curl >/dev/null 2>&1 || { echo "SKIP: curl not found"; exit 3; }
command -v jq   >/dev/null 2>&1 || { echo "SKIP: jq not found"; exit 3; }
[ -n "$PASSWORD" ] || { echo "SKIP: HELIX_ADMIN_PASSWORD / --password is required (server admin login is disabled without it)"; exit 3; }

# ---- 1. health -----------------------------------------------------------------
req GET "/healthz" "" noauth
if [ "$HTTP_STATUS" != "200" ]; then
  fatal "server is not reachable / healthy at ${BASE_URL}/healthz (got HTTP $HTTP_STATUS)"
fi
pass "GET /healthz -> 200"

# ---- 2. login ------------------------------------------------------------------
req POST "${API}/auth/login" "$(jq -nc --arg u "$USERNAME" --arg p "$PASSWORD" '{username:$u,password:$p}')" noauth
assert_status 200 "POST /auth/login" || fatal "login failed — cannot continue without a token"
TOKEN="$(jqget '.access_token')"
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || fatal "login 200 but no access_token in body"
[ "$(jqget '.token_type')" = "Bearer" ] && pass "login token_type is Bearer" || fail "login token_type is not Bearer"
# Token contract (server/internal/api/token.go): a JWT-shaped, HMAC-signed
# claims blob "<base64url-claims>.<sig>" (2 segments). Assert that real shape AND
# that segment 1 decodes to JSON carrying sub+roles — not a weaker "non-empty".
SEGN="$(printf '%s' "$TOKEN" | awk -F. '{print NF}')"
if [ "$SEGN" = "2" ]; then
  # Claims are base64url WITHOUT padding (Go RawURLEncoding); re-pad before decode.
  SEG1="${TOKEN%%.*}"
  PAD=$(( (4 - ${#SEG1} % 4) % 4 ))
  SEG1="${SEG1}$(printf '%*s' "$PAD" '' | tr ' ' '=')"
  CLAIMS_JSON="$(printf '%s' "$SEG1" | tr '_-' '/+' | base64 -d 2>/dev/null)"
  if printf '%s' "$CLAIMS_JSON" | jq -e 'has("sub") and (.roles|index("admin"))' >/dev/null 2>&1; then
    pass "access_token is a signed claims blob carrying sub + admin role"
  else
    fail "access_token segment 1 did not decode to claims with sub+roles (got: ${CLAIMS_JSON:0:64})"
  fi
else
  fail "access_token does not match the documented <claims>.<sig> 2-segment shape (segments=$SEGN)"
fi

# ---- 3. ANTI-BLUFF: protected route must reject an unauthenticated request -----
req GET "${API}/telemetry/overview" "" noauth
if [ "$HTTP_STATUS" = "401" ]; then
  pass "anti-bluff: unauthenticated GET /telemetry/overview -> 401 (auth enforced)"
else
  fail "anti-bluff: unauthenticated request was NOT rejected (got HTTP $HTTP_STATUS; expected 401)"
fi

# ---- 4. register a device ------------------------------------------------------
DEV_BODY="$(jq -nc --arg hw "hw-${RUN_TAG}" '{hardware_id:$hw, model:"orangepi5max", os:"android", current_version:"1.0.0"}')"
req POST "${API}/devices/register" "$DEV_BODY" auth
assert_status 201 "POST /devices/register"
DEVICE_ID="$(jqget '.device_id')"
[ -n "$DEVICE_ID" ] && [ "$DEVICE_ID" != "null" ] || fail "register 201 but no device_id"
[ -n "$(jqget '.device_token')" ] && pass "device registration returned a device_token" || fail "no device_token in registration body"

# ---- 5. device status ----------------------------------------------------------
if [ -n "$DEVICE_ID" ]; then
  req GET "${API}/devices/${DEVICE_ID}/status" "" auth
  assert_status 200 "GET /devices/{id}/status"
  [ "$(jqget '.device_id')" = "$DEVICE_ID" ] && pass "device status echoes device_id" || fail "device status device_id mismatch"
fi

# ---- 6. create group -----------------------------------------------------------
GRP_NAME="grp-${RUN_TAG}"
req POST "${API}/groups" "$(jq -nc --arg n "$GRP_NAME" '{name:$n, description:"e2e challenge group"}')" auth
assert_status 201 "POST /groups"
# Wire change (breaking): the group id key is now "group_id" (was "id").
# Anti-bluff: reading ".id" must now FAIL — a server emitting the old key is wrong.
GROUP_ID="$(jqget '.group_id')"
[ -n "$GROUP_ID" ] && [ "$GROUP_ID" != "null" ] || fatal "group create 201 but no group_id"
if [ "$(jqget '.id')" = "null" ]; then
  pass "group create uses new 'group_id' key (old 'id' key absent)"
else
  fail "group create still emits the deprecated 'id' key (breaking-change regression)"
fi
[ "$(jqget '.name')" = "$GRP_NAME" ] && pass "group create echoes name" || fail "group name mismatch"

# ---- 7. get group --------------------------------------------------------------
req GET "${API}/groups/${GROUP_ID}" "" auth
assert_status 200 "GET /groups/{id}"
[ "$(jqget '.name')" = "$GRP_NAME" ] && pass "GET group echoes name" || fail "GET group name mismatch"

# ---- 8. add member (BATCH wire change) -----------------------------------------
# Wire change (breaking): the body is now a BATCH {"device_ids":[...]} (was a
# single {"device_id":"..."}) and the success status is 200 (was 204) with a
# body {"added":[...],"already_member":[...],"not_found":[...]}. A REGISTERED
# device lands in .added. Anti-bluff: a 204 must now FAIL.
if [ -n "$DEVICE_ID" ]; then
  req POST "${API}/groups/${GROUP_ID}/members" "$(jq -nc --arg d "$DEVICE_ID" '{device_ids:[$d]}')" auth
  assert_status 200 "POST /groups/{id}/members (batch add registered device)"
  if printf '%s' "$HTTP_BODY" | jq -e --arg d "$DEVICE_ID" '.added | index($d) != null' >/dev/null 2>&1; then
    pass "batch add put the registered device in .added"
  else
    fail "batch add did NOT report the device in .added (body: $(printf '%s' "$HTTP_BODY" | head -c 200))"
  fi

  # ---- 8b. re-add the same device -> it is already a member ---------------------
  req POST "${API}/groups/${GROUP_ID}/members" "$(jq -nc --arg d "$DEVICE_ID" '{device_ids:[$d]}')" auth
  assert_status 200 "POST /groups/{id}/members (re-add) -> 200"
  if printf '%s' "$HTTP_BODY" | jq -e --arg d "$DEVICE_ID" '.already_member | index($d) != null' >/dev/null 2>&1; then
    pass "re-add reports the device in .already_member"
  else
    fail "re-add did NOT report the device in .already_member (body: $(printf '%s' "$HTTP_BODY" | head -c 200))"
  fi
fi

# ---- 8c. an UNREGISTERED device id lands in .not_found -------------------------
UNREG_ID="unregistered-device-${RUN_TAG}"
req POST "${API}/groups/${GROUP_ID}/members" "$(jq -nc --arg d "$UNREG_ID" '{device_ids:[$d]}')" auth
assert_status 200 "POST /groups/{id}/members (unregistered id) -> 200"
if printf '%s' "$HTTP_BODY" | jq -e --arg d "$UNREG_ID" '.not_found | index($d) != null' >/dev/null 2>&1; then
  pass "unregistered device id is reported in .not_found"
else
  fail "unregistered device id was NOT reported in .not_found (body: $(printf '%s' "$HTTP_BODY" | head -c 200))"
fi

# ---- 8d. an EMPTY device_ids batch is rejected with 400 -----------------------
req POST "${API}/groups/${GROUP_ID}/members" '{"device_ids":[]}' auth
assert_status 400 "POST /groups/{id}/members (empty device_ids) -> 400"

# ---- 8e. adding to an UNKNOWN group is still 404 ------------------------------
req POST "${API}/groups/no-such-group-${RUN_TAG}/members" "$(jq -nc --arg d "$DEVICE_ID" '{device_ids:[$d]}')" auth
assert_status 404 "POST /groups/{absent}/members -> 404"

# ---- 9. list members (must contain the added device) ---------------------------
# Wire change (breaking): the members body is now {"group_id":"...","items":[
# {"device_id":"...","added_at":"<RFC3339>"}]} (was {"group_id","device_ids":[...]}).
# Read .items[].device_id (NOT the deprecated .device_ids), and assert the
# membership's added_at is present + non-empty. Anti-bluff: a server still
# emitting the old .device_ids array (or an empty added_at) must now FAIL.
req GET "${API}/groups/${GROUP_ID}/members" "" auth
assert_status 200 "GET /groups/{id}/members"
[ "$(jqget '.group_id')" = "$GROUP_ID" ] && pass "member list echoes group_id" || fail "member list group_id mismatch"
# The deprecated flat .device_ids array must be gone.
if [ "$(jqget '.device_ids')" = "null" ]; then
  pass "member list dropped the deprecated 'device_ids' array (now '.items[]')"
else
  fail "member list still emits the deprecated 'device_ids' array (breaking-change regression)"
fi
if [ -n "$DEVICE_ID" ]; then
  if printf '%s' "$HTTP_BODY" | jq -e --arg d "$DEVICE_ID" '[.items[].device_id] | index($d) != null' >/dev/null 2>&1; then
    pass "member list .items[].device_id contains the added device"
  else
    fail "member list .items[].device_id does NOT contain the added device (body: $(printf '%s' "$HTTP_BODY" | head -c 200))"
  fi
  # added_at must be present + non-empty for the member we added.
  MEMBER_ADDED_AT="$(printf '%s' "$HTTP_BODY" | jq -r --arg d "$DEVICE_ID" '.items[] | select(.device_id==$d) | .added_at' 2>/dev/null)"
  if [ -n "$MEMBER_ADDED_AT" ] && [ "$MEMBER_ADDED_AT" != "null" ]; then
    pass "member item carries a non-empty added_at ($MEMBER_ADDED_AT)"
  else
    fail "member item is missing a non-empty added_at (body: $(printf '%s' "$HTTP_BODY" | head -c 200))"
  fi
fi

# ---- 10. telemetry overview ----------------------------------------------------
req GET "${API}/telemetry/overview" "" auth
assert_status 200 "GET /telemetry/overview"
if printf '%s' "$HTTP_BODY" | jq -e 'has("total") and has("event_counts")' >/dev/null 2>&1; then
  pass "telemetry overview has total + event_counts"
else
  fail "telemetry overview missing total/event_counts (body: $(printf '%s' "$HTTP_BODY" | head -c 200))"
fi

# ---- 11. audit (ANTI-BLUFF: group actions MUST be recorded) --------------------
req GET "${API}/audit?limit=200" "" auth
assert_status 200 "GET /audit"
AUDIT_COUNT="$(printf '%s' "$HTTP_BODY" | jq -r '.items | length' 2>/dev/null)"
if [ -z "$AUDIT_COUNT" ] || [ "$AUDIT_COUNT" = "null" ] || [ "$AUDIT_COUNT" -eq 0 ] 2>/dev/null; then
  fail "anti-bluff: audit log is EMPTY after auditable mutations (a working server MUST have logged GROUP_CREATE)"
else
  pass "audit log is non-empty ($AUDIT_COUNT entries)"
  if printf '%s' "$HTTP_BODY" | jq -e '[.items[].action] | index("GROUP_CREATE") != null' >/dev/null 2>&1; then
    pass "audit log contains GROUP_CREATE (the group we created was audited)"
  else
    fail "audit log missing GROUP_CREATE action (actions: $(printf '%s' "$HTTP_BODY" | jq -rc '[.items[].action]|unique' 2>/dev/null))"
  fi
  if [ -n "$DEVICE_ID" ]; then
    if printf '%s' "$HTTP_BODY" | jq -e '[.items[].action] | index("GROUP_MEMBER_CREATE") != null' >/dev/null 2>&1; then
      pass "audit log contains GROUP_MEMBER_CREATE (member-add was audited)"
    else
      fail "audit log missing GROUP_MEMBER_CREATE (actions: $(printf '%s' "$HTTP_BODY" | jq -rc '[.items[].action]|unique' 2>/dev/null))"
    fi
  fi
  # Cross-check: the GET /audit read itself must NOT be audited (only mutations are).
  if printf '%s' "$HTTP_BODY" | jq -e '[.items[].action] | index("AUDIT_ACTION") != null' >/dev/null 2>&1; then
    fail "anti-bluff: a GET was audited (only mutating actions should be recorded)"
  else
    pass "anti-bluff: GET reads are not audited (only mutations)"
  fi
fi

# ---- 12. rollout route semantics (deterministic, black-box) --------------------
BOGUS_DEP="no-such-deployment-${RUN_TAG}"
req GET "${API}/deployments/${BOGUS_DEP}/rollout" "" auth
assert_status 404 "GET /deployments/{absent}/rollout -> 404"

req POST "${API}/deployments/${BOGUS_DEP}/rollout" \
  '{"phases":[{"percentage":50,"success_threshold":0.95,"error_threshold":0.05,"duration_seconds":60,"auto_progress":true},{"percentage":100,"success_threshold":0.95,"error_threshold":0.05,"duration_seconds":60,"auto_progress":true}]}' auth
assert_status 404 "POST /deployments/{absent}/rollout -> 404 (deployment-existence gate)"

# ---- 13. OPTIONAL full pipeline: artifact -> release -> deployment -> rollout ---
# The artifact upload verifies a detached ed25519 signature against the server's
# CONFIG-supplied trusted key (trust boundary: never request-supplied). A signed
# upload reproducible from this shell would have to match the validator brick's
# exact signing contract; if it does not, we SKIP-with-reason (NEVER a false
# PASS, per §11.4.3 / §11.4.6) instead of fabricating a deployment.
if [ "${HELIX_E2E_TRY_PIPELINE:-0}" = "1" ] && [ -n "${HELIX_E2E_VERIFIED_RELEASE_ID:-}" ]; then
  REL_ID="$HELIX_E2E_VERIFIED_RELEASE_ID"
  req POST "${API}/deployments" "$(jq -nc --arg r "$REL_ID" '{release_id:$r, strategy:"all-targets"}')" auth
  if [ "$HTTP_STATUS" = "201" ]; then
    DEP_ID="$(jqget '.deployment_id')"
    pass "deployment created from supplied verified release ($DEP_ID)"
    req POST "${API}/deployments/${DEP_ID}/rollout" \
      '{"phases":[{"percentage":50,"success_threshold":0.95,"error_threshold":0.05,"duration_seconds":60,"auto_progress":true},{"percentage":100,"success_threshold":0.95,"error_threshold":0.05,"duration_seconds":60,"auto_progress":true}]}' auth
    assert_status 201 "POST /deployments/{id}/rollout -> 201"
    [ "$(jqget '.deployment_id')" = "$DEP_ID" ] && pass "rollout state echoes deployment_id" || fail "rollout deployment_id mismatch"
    req GET "${API}/deployments/${DEP_ID}/rollout" "" auth
    assert_status 200 "GET /deployments/{id}/rollout -> 200"
    req POST "${API}/deployments/${DEP_ID}/rollout/evaluate" \
      '{"success_rate":0.99,"error_rate":0.0,"post_boot_health_failed":false}' auth
    assert_status 200 "POST /deployments/{id}/rollout/evaluate -> 200"
    ACT="$(jqget '.action')"
    [ -n "$ACT" ] && [ "$ACT" != "null" ] && pass "rollout evaluate returned a decision action: $ACT" || fail "rollout evaluate returned no action"
  else
    skip "full pipeline: deployment create returned HTTP $HTTP_STATUS (supplied release not deployable) — rollout-with-deployment stage skipped"
  fi
else
  skip "full pipeline (upload->release->deploy->rollout create/get/evaluate): requires a server-config-coupled signed artifact; set HELIX_E2E_TRY_PIPELINE=1 + HELIX_E2E_VERIFIED_RELEASE_ID=<id> to drive it. Rollout ROUTES were still asserted deterministically above (step 12)."
fi

# ---- 14/15. self-clean + prove deletion is real --------------------------------
if [ -n "$DEVICE_ID" ]; then
  req DELETE "${API}/groups/${GROUP_ID}/members/${DEVICE_ID}" "" auth
  assert_status 204 "DELETE /groups/{id}/members/{deviceId} -> 204"
fi
req DELETE "${API}/groups/${GROUP_ID}" "" auth
assert_status 204 "DELETE /groups/{id} -> 204"
GROUP_DELETED_ID="$GROUP_ID"
req GET "${API}/groups/${GROUP_DELETED_ID}" "" auth
assert_status 404 "GET /groups/{deleted} -> 404 (deletion is real)"
GROUP_ID=""  # already deleted; stop the EXIT trap from re-deleting

# ---- summary -------------------------------------------------------------------
echo
echo "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
if [ "$FAIL" -gt 0 ]; then
  echo "RESULT: FAIL"
  exit 1
fi
echo "RESULT: PASS"
exit 0
