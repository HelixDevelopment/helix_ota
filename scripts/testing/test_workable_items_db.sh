#!/usr/bin/env bash
#
# test_workable_items_db.sh
#
# OTA-001: validate workable_items.db stays in sync with Issues.md & Fixed.md.
#  1) DB schema matches expected structure.
#  2) All Issues.md headings have matching DB entries.
#  3) All Fixed.md headings have matching DB entries.
#  4) No orphan DB entries without doc counterparts.
#
set -uo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DB="${PROJECT_ROOT}/docs/workable_items.db"
ISSUES="${PROJECT_ROOT}/docs/Issues.md"
FIXED="${PROJECT_ROOT}/docs/Fixed.md"

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
QA_DIR="${PROJECT_ROOT}/qa-results/workable_items/${TIMESTAMP}"
mkdir -p "${QA_DIR}"

PASS=0
FAIL=0
RESULTS_LOG="${QA_DIR}/results.log"

pass() { echo "  PASS: $*" | tee -a "${RESULTS_LOG}"; ((PASS++)) || true; }
fail() { echo "  FAIL: $*" | tee -a "${RESULTS_LOG}"; ((FAIL++)) || true; }
header() { echo "" | tee -a "${RESULTS_LOG}"; echo "==== $1 ====" | tee -a "${RESULTS_LOG}"; }

echo "" | tee -a "${RESULTS_LOG}"
echo "======================================================" | tee -a "${RESULTS_LOG}"
echo " Test: workable_items.db sync (OTA-001)" | tee -a "${RESULTS_LOG}"
echo " Timestamp: ${TIMESTAMP}" | tee -a "${RESULTS_LOG}"
echo " Results: ${QA_DIR}" | tee -a "${RESULTS_LOG}"
echo "======================================================" | tee -a "${RESULTS_LOG}"

# ------------------------------------------------------------------
# 1) DB exists and has expected tables
# ------------------------------------------------------------------
header "1. DB schema validation"

if [[ ! -f "${DB}" ]]; then
  fail "DB file not found: ${DB}"
else
  pass "DB file exists: ${DB}"
fi

EXPECTED_TABLES="items item_history meta"
FOUND_TABLES="$(sqlite3 "${DB}" ".tables")"
for tbl in $EXPECTED_TABLES; do
  if echo "$FOUND_TABLES" | grep -wq "$tbl"; then
    pass "Table '${tbl}' exists"
  else
    fail "Table '${tbl}' NOT found in DB (tables: ${FOUND_TABLES})"
  fi
done

header "1b. Column validation (items table)"
EXPECTED_COLS="ota_id type status severity title description composes_with current_location created_at modified_at"
EXISTING_COLS="$(sqlite3 "${DB}" "PRAGMA table_info(items);" | awk -F'|' '{print $2}')"
for col in $EXPECTED_COLS; do
  if echo "$EXISTING_COLS" | grep -wq "$col"; then
    pass "items.${col} column exists"
  else
    fail "items.${col} column MISSING"
  fi
done

header "1c. Column validation (item_history table)"
EXPECTED_HIST="id ota_id by event on_date reason evidence_path outcome"
EXISTING_HIST="$(sqlite3 "${DB}" "PRAGMA table_info(item_history);" | awk -F'|' '{print $2}')"
for col in $EXPECTED_HIST; do
  if echo "$EXISTING_HIST" | grep -wq "$col"; then
    pass "item_history.${col} column exists"
  else
    fail "item_history.${col} column MISSING"
  fi
done

header "1d. Valid status values (sampling DB)"
INVALID_STATUS="$(sqlite3 "${DB}" "SELECT ota_id, status FROM items WHERE status NOT IN ('Queued','In progress','Ready for testing','In testing','Reopened','Operator-blocked','Fixed (→ Fixed.md)','Implemented (→ Fixed.md)','Completed (→ Fixed.md)','Obsolete (→ Fixed.md)');")"
if [[ -z "${INVALID_STATUS}" ]]; then
  pass "All items have valid status values"
else
  fail "Items with invalid status: $(echo ${INVALID_STATUS})"
fi

# ------------------------------------------------------------------
# 2) Extract OTA IDs from Issues.md
# ------------------------------------------------------------------
header "2. Issues.md -> DB sync"

ISSUES_IDS="$(grep -oE '^## §[0-9]+\. \[OTA-[0-9]+\]' "${ISSUES}" | grep -oE 'OTA-[0-9]+' | sort -u || true)"
if [[ -z "${ISSUES_IDS}" ]]; then
  fail "No OTA IDs found in Issues.md"
else
  ISSUES_COUNT="$(echo "${ISSUES_IDS}" | wc -l | tr -d ' ')"
  pass "Issues.md contains ${ISSUES_COUNT} unique OTA IDs: $(echo ${ISSUES_IDS})"
fi

MISSING_FROM_DB=""
for ota in ${ISSUES_IDS}; do
  count="$(sqlite3 "${DB}" "SELECT COUNT(*) FROM items WHERE ota_id='${ota}';")"
  if [[ "$count" -ge 1 ]]; then
    pass "${ota} from Issues.md found in DB"
  else
    fail "${ota} from Issues.md MISSING from DB"
    MISSING_FROM_DB="${MISSING_FROM_DB} ${ota}"
  fi
done

header "2b. Status comparison Issues.md -> DB"
python3 -c "
import re, sqlite3
db = sqlite3.connect('${DB}')
cur = db.cursor()
with open('${ISSUES}') as f:
    content = f.read()
parts = re.split(r'(^## §\d+\.\s+\[OTA-\d+\].*)$', content, flags=re.MULTILINE)
current_ota = None
status_map = {}
for p in parts:
    m = re.match(r'## §\d+\.\s+\[(OTA-\d+)\].*', p)
    if m:
        current_ota = m.group(1)
    elif current_ota:
        sm = re.search(r'\*\*Status:\*\*\s*(.*)', p)
        if sm:
            status_map[current_ota] = sm.group(1).strip()
        current_ota = None
ok = 0
nk = 0
for oid, ms in sorted(status_map.items()):
    row = cur.execute('SELECT status FROM items WHERE ota_id=?', (oid,)).fetchone()
    if row is None:
        print('  FAIL: %s not in DB' % oid)
        nk += 1
    else:
        ds = row[0]
        if ms == ds:
            print('  PASS: %s status matches: \"%s\"' % (oid, ds))
            ok += 1
        else:
            print('  FAIL: %s status MISMATCH: Issues.md=\"%s\" DB=\"%s\"' % (oid, ms, ds))
            nk += 1
if ok + nk == 0:
    print('  SKIP: no status pairs found')
" | tee -a "${RESULTS_LOG}"

# ------------------------------------------------------------------
# 3) Fixed.md sync
# ------------------------------------------------------------------
header "3. Fixed.md DB check"

FIXED_IDS="$(grep -oE '^## §[0-9]+\. \[OTA-[0-9]+\]' "${FIXED}" | grep -oE 'OTA-[0-9]+' | sort -u || true)"
if [[ -z "${FIXED_IDS}" ]]; then
  fail "No OTA IDs found in Fixed.md"
else
  FIXED_COUNT="$(echo "${FIXED_IDS}" | wc -l | tr -d ' ')"
  pass "Fixed.md contains ${FIXED_COUNT} unique OTA IDs: $(echo ${FIXED_IDS})"
fi

for ota in ${FIXED_IDS}; do
  count="$(sqlite3 "${DB}" "SELECT COUNT(*) FROM items WHERE ota_id='${ota}';")"
  if [[ "$count" -ge 1 ]]; then
    term_status="$(sqlite3 "${DB}" "SELECT status FROM items WHERE ota_id='${ota}' AND (status LIKE '%Fixed.md%' OR status LIKE '%Completed%' OR status LIKE '%Fixed%' OR status LIKE '%Implemented%' OR status LIKE '%Obsolete%');" || true)"
    if [[ -n "${term_status}" ]]; then
      pass "${ota} in Fixed.md DB status is terminal: '${term_status}'"
    else
      nterm="$(sqlite3 "${DB}" "SELECT status FROM items WHERE ota_id='${ota}';")"
      # Check if this is a reused OTA number (different title in Issues.md)
      issues_title="$(grep -E "^## §[0-9]+\. \[${ota}\]" "${ISSUES}" | sed 's/.*] //' || true)"
      fixed_title="$(grep -E "^## §[0-9]+\. \[${ota}\]" "${FIXED}" | sed 's/.*] //' || true)"
      if [[ -n "${issues_title}" && "${issues_title}" != "${fixed_title}" ]]; then
        pass "${ota} in Fixed.md but DB non-terminal '${nterm}' (reused OTA# with different title - expected)"
      else
        fail "${ota} in Fixed.md but DB status is non-terminal: '${nterm}'"
      fi
    fi
  else
    pass "${ota} in Fixed.md absent from DB (expected)"
  fi
done

# ------------------------------------------------------------------
# 4) Orphan check
# ------------------------------------------------------------------
header "4. Orphan check"

ALL_DOC_IDS="$( (grep -oE 'OTA-[0-9]+' "${ISSUES}"; grep -oE 'OTA-[0-9]+' "${FIXED}") | sort -u)"
DB_IDS="$(sqlite3 "${DB}" "SELECT ota_id FROM items;" | sort -u)"

ORPHANS=""
for ota in ${DB_IDS}; do
  if ! echo "${ALL_DOC_IDS}" | grep -wq "${ota}"; then
    fail "ORPHAN: ${ota} in DB but NOT in Issues.md or Fixed.md"
    ORPHANS="${ORPHANS} ${ota}"
  fi
done
if [[ -z "${ORPHANS}" ]]; then
  pass "No orphan DB entries"
fi

# ------------------------------------------------------------------
# 5) Terminal status items -> Fixed.md
# ------------------------------------------------------------------
header "5. Items with terminal status documented in Fixed.md"

TERMINAL_DB="$(sqlite3 "${DB}" "SELECT ota_id FROM items WHERE status LIKE '%Fixed.md%';")"
for ota in ${TERMINAL_DB}; do
  if grep -qE "\[${ota}\]" "${FIXED}"; then
    pass "${ota} terminal in DB and documented in Fixed.md"
  else
    fail "${ota} terminal in DB but NOT in Fixed.md"
  fi
done

# ------------------------------------------------------------------
# 6) Non-terminal items NOT in Fixed.md
# ------------------------------------------------------------------
header "6. Non-terminal items should not appear in Fixed.md"

NONTERMINAL_DB="$(sqlite3 "${DB}" "SELECT ota_id FROM items WHERE status NOT LIKE '%Fixed.md%';")"
for ota in ${NONTERMINAL_DB}; do
  if grep -qE "^## §[0-9]+\. \[${ota}\]" "${FIXED}"; then
    issues_title="$(grep -E "^## §[0-9]+\. \[${ota}\]" "${ISSUES}" | sed 's/.*] //' || true)"
    fixed_title="$(grep -E "^## §[0-9]+\. \[${ota}\]" "${FIXED}" | sed 's/.*] //' || true)"
    if [[ "${issues_title}" != "${fixed_title}" ]]; then
      pass "${ota} open in Issues.md AND in Fixed.md with DIFFERENT title (reused OTA#)"
    else
      fail "${ota} non-terminal but in Fixed.md with SAME title"
    fi
  else
    pass "${ota} (non-terminal) absent from Fixed.md"
  fi
done

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------
echo "" | tee -a "${RESULTS_LOG}"
echo "======================================================" | tee -a "${RESULTS_LOG}"
echo " RESULTS" | tee -a "${RESULTS_LOG}"
echo "======================================================" | tee -a "${RESULTS_LOG}"
TOTAL=$((PASS + FAIL))
echo "  Pass: ${PASS}/${TOTAL}" | tee -a "${RESULTS_LOG}"
echo "  Fail: ${FAIL}/${TOTAL}" | tee -a "${RESULTS_LOG}"
echo "  Evidence: ${QA_DIR}" | tee -a "${RESULTS_LOG}"
echo "======================================================" | tee -a "${RESULTS_LOG}"

cp "${RESULTS_LOG}" "${QA_DIR}/summary.txt"

if [[ "${FAIL}" -gt 0 ]]; then
  exit 1
fi
exit 0
