#!/usr/bin/env bash
#
# migrate_workable_items.sh
#
# OTA-001: Idempotent migration from Issues.md / Fixed.md -> workable_items.db.
#   - Reads Issues.md headings, inserts any missing items into DB.
#   - Reads Fixed.md headings, updates status + migrates any DB item that
#     is now documented as fixed/closed.
#   - Safe to re-run (idempotent).
#
# IMPORTANT: An OTA number may appear in BOTH Issues.md and Fixed.md when
# the number was reused for a different item after the original was closed.
# In that situation, Issues.md takes precedence (the item is still open).
#
set -uo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DB="${PROJECT_ROOT}/docs/workable_items.db"
ISSUES="${PROJECT_ROOT}/docs/Issues.md"
FIXED="${PROJECT_ROOT}/docs/Fixed.md"

echo "=== Migration: Issues.md + Fixed.md -> workable_items.db ==="
echo "  DB:      ${DB}"
echo "  Issues:  ${ISSUES}"
echo "  Fixed:   ${FIXED}"
echo ""

for f in "${DB}" "${ISSUES}" "${FIXED}"; do
  if [[ ! -f "$f" ]]; then
    echo "ERROR: $f not found" >&2
    exit 1
  fi
done

if ! sqlite3 "${DB}" "SELECT 1;" >/dev/null 2>&1; then
  echo "ERROR: Cannot connect to ${DB}" >&2
  exit 1
fi

# ---- Determine which OTA numbers appear in both files (reused numbers) ----
ISSUES_OTAS="$(grep -oE '\[OTA-[0-9]+\]' "${ISSUES}" | tr -d '[]' | sort -u)"
FIXED_OTAS="$(grep -oE '\[OTA-[0-9]+\]' "${FIXED}" | tr -d '[]' | sort -u)"
# Reused: appear in both
REUSED_OTAS="$( (echo "${ISSUES_OTAS}" && echo "${FIXED_OTAS}") | sort | uniq -d)"
echo "  Reused OTA numbers (in both Issues.md and Fixed.md):"
if [[ -n "${REUSED_OTAS}" ]]; then
  for ota in ${REUSED_OTAS}; do
    issues_title="$(grep -E "^## §[0-9]+\. \[${ota}\]" "${ISSUES}" | sed 's/.*] //')"
    fixed_title="$(grep -E "^## §[0-9]+\. \[${ota}\]" "${FIXED}" | sed 's/.*] //')"
    echo "    ${ota}: Issues.md=\"${issues_title}\"  Fixed.md=\"${fixed_title}\""
  done
fi

# ---- Helper: extract field from a markdown section by label ----
extract_field() {
  local section="$1"
  local label="$2"
  # Match "**Label:** value" on the same line
  echo "${section}" | grep -oE "\*\*${label}:\*\*[[:space:]]*.*" | head -1 | sed "s/\*\*${label}:\*\*[[:space:]]*//" || true
}

# ---- Helper: process a section from Issues.md ----
migrate_from_issues() {
  local section="$1"
  local ota_id title status_text type_text description

  ota_id="$(echo "${section}" | grep -oE '\[OTA-[0-9]+\]' | head -1 | tr -d '[]' || true)"
  [[ -z "${ota_id}" ]] && return 0

  title="$(echo "${section}" | grep -oE '^## §[0-9]+\. \[OTA-[0-9]+\].*' | sed -E 's/^## §[0-9]+\. \[OTA-[0-9]+\] //' || true)"

  status_text="$(extract_field "${section}" "Status")"
  [[ -z "${status_text}" ]] && status_text="Queued"

  type_text="$(extract_field "${section}" "Type")"
  [[ -z "${type_text}" ]] && type_text="Task"

  description="$(extract_field "${section}" "Description")"
  [[ -z "${description}" ]] && description="(from Issues.md)"

  existing="$(sqlite3 "${DB}" "SELECT COUNT(*) FROM items WHERE ota_id='${ota_id}';")"
  if [[ "${existing}" -eq 0 ]]; then
    echo "  INSERT ${ota_id}: \"${title}\" [${type_text}, ${status_text}]"
    # Use single-quote safe substitution
    safe_title="$(echo "${title}" | sed "s/'/''/g")"
    safe_desc="$(echo "${description}" | sed "s/'/''/g")"
    sqlite3 "${DB}" "INSERT INTO items (ota_id, type, status, title, description)
      VALUES ('${ota_id}', '${type_text}', '${status_text}', '${safe_title}', '${safe_desc}');"
    sqlite3 "${DB}" "INSERT INTO item_history (ota_id, by, event, reason)
      VALUES ('${ota_id}', 'AI', 'Opened', 'Migrated from Issues.md');"
  else
    db_status="$(sqlite3 "${DB}" "SELECT status FROM items WHERE ota_id='${ota_id}';")"
    if [[ "${db_status}" != "${status_text}" ]]; then
      echo "  UPDATE ${ota_id}: status '${db_status}' -> '${status_text}'"
      sqlite3 "${DB}" "UPDATE items SET status='${status_text}', modified_at=datetime('now') WHERE ota_id='${ota_id}';"
      sqlite3 "${DB}" "INSERT INTO item_history (ota_id, by, event, reason)
        VALUES ('${ota_id}', 'AI', 'Status Update', 'Status synced from Issues.md');"
    else
      echo "  OK    ${ota_id}: already in DB with status '${db_status}'"
    fi
  fi
}

# ---- Helper: process a section from Fixed.md ----
migrate_from_fixed() {
  local section="$1"
  local ota_id title fixed_status

  ota_id="$(echo "${section}" | grep -oE '\[OTA-[0-9]+\]' | head -1 | tr -d '[]' || true)"
  [[ -z "${ota_id}" ]] && return 0

  title="$(echo "${section}" | grep -oE '^## §[0-9]+\. \[OTA-[0-9]+\].*' | sed -E 's/^## §[0-9]+\. \[OTA-[0-9]+\] //' || true)"

  fixed_status="$(extract_field "${section}" "Status")"
  [[ -z "${fixed_status}" ]] && fixed_status="Completed (-> Fixed.md)"

  # Skip if this OTA number is reused (appears in both Issues.md and Fixed.md)
  if echo "${REUSED_OTAS}" | grep -wq "${ota_id}"; then
    echo "  SKIP  ${ota_id}: reused OTA# (Issues.md entry takes precedence)"
    return 0
  fi

  existing="$(sqlite3 "${DB}" "SELECT COUNT(*) FROM items WHERE ota_id='${ota_id}';")"
  if [[ "${existing}" -eq 0 ]]; then
    echo "  SKIP  ${ota_id}: not in DB (already migrated)"
    return 0
  fi

  db_status="$(sqlite3 "${DB}" "SELECT status FROM items WHERE ota_id='${ota_id}';")"
  if echo "${db_status}" | grep -q -E '-> Fixed\.md$'; then
    echo "  OK    ${ota_id}: already has terminal status '${db_status}'"
  else
    echo "  CLOSE ${ota_id}: '${db_status}' -> '${fixed_status}'"
    safe_title="$(echo "${title}" | sed "s/'/''/g")"
    sqlite3 "${DB}" "UPDATE items SET status='${fixed_status}', modified_at=datetime('now') WHERE ota_id='${ota_id}';"
    sqlite3 "${DB}" "INSERT INTO item_history (ota_id, by, event, reason)
      VALUES ('${ota_id}', 'AI', '${fixed_status}', 'Migrated from Fixed.md');"
  fi
}

# ---- Main ----------------------------------------------------------------

echo ""
echo "--- Step 1: Reading Issues.md (inserting/updating DB) ---"
# Read Issues.md sections via awk
awk '
/^## §/ { if (buf != "") print buf; buf = $0 "\n"; next }
      { buf = buf $0 "\n" }
END   { if (buf != "") print buf }
' "${ISSUES}" | while IFS= read -r block; do
  migrate_from_issues "${block}"
done

echo ""
echo "--- Step 2: Reading Fixed.md (updating terminal statuses) ---"
awk '
/^## §/ { if (buf != "") print buf; buf = $0 "\n"; next }
      { buf = buf $0 "\n" }
END   { if (buf != "") print buf }
' "${FIXED}" | while IFS= read -r block; do
  migrate_from_fixed "${block}"
done

echo ""
echo "=== Migration complete ==="
sqlite3 "${DB}" "SELECT ota_id, type, status, title FROM items ORDER BY ota_id;" -column -header
