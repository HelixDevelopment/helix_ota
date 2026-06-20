#!/usr/bin/env bash
#
# migrate_workable_items.sh
#
# OTA-001: Idempotent migration from Issues.md / Fixed.md -> workable_items.db.
# Uses Python3 for robust parsing (macOS/linux compatible).
#
set -uo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DB="${PROJECT_ROOT}/docs/workable_items.db"
ISSUES="${PROJECT_ROOT}/docs/Issues.md"
FIXED="${PROJECT_ROOT}/docs/Fixed.md"

for f in "${DB}" "${ISSUES}" "${FIXED}"; do
  if [[ ! -f "$f" ]]; then echo "ERROR: $f not found" >&2; exit 1; fi
done

echo "=== Migration: Issues.md + Fixed.md -> workable_items.db ==="
echo "  DB:      ${DB}"
echo "  Issues:  ${ISSUES}"
echo "  Fixed:   ${FIXED}"

python3 << PYEOF
import re, sqlite3, sys

db_path = "${DB}"
issues_path = "${ISSUES}"
fixed_path = "${FIXED}"

db = sqlite3.connect(db_path)
cur = db.cursor()

# ---- Parse markdown sections ----
def parse_sections(filepath):
    """Return dict: {ota_id: {'title':str, 'status':str, 'type':str, 'description':str}}"""
    with open(filepath) as f:
        content = f.read()

    sections = {}
    # Split on ## § headings
    lines = content.split('\n')
    current_id = None
    current_lines = []

    def flush():
        nonlocal current_id, current_lines
        if current_id is None:
            return
        block = '\n'.join(current_lines)
        title = ''
        heading_match = re.search(r'\[(OTA-\d+)\]\s*(.*)', block.split('\n')[0])
        if heading_match:
            current_id = heading_match.group(1)
            title = heading_match.group(2).strip()

        status_m = re.search(r'\*\*Status:\*\*\s*(.+?)(?:\s*\*\*|$)', block)
        status = status_m.group(1).strip() if status_m else 'Queued'

        type_m = re.search(r'\*\*Type:\*\*\s*(.+?)(?:\s*\*\*|$)', block)
        typ = type_m.group(1).strip() if type_m else 'Task'

        desc_m = re.search(r'\*\*Description:\*\*\s*(.+?)$', block, re.MULTILINE)
        desc = desc_m.group(1).strip() if desc_m else '(from doc)'

        sections[current_id] = {'title': title, 'status': status, 'type': typ, 'description': desc}
        current_id = None
        current_lines = []

    for line in lines:
        if re.match(r'^## §\d+\.\s+\[OTA-\d+\]', line):
            flush()
            # Extract ID from heading
            m = re.search(r'\[(OTA-\d+)\]', line)
            if m:
                current_id = m.group(1)
            current_lines = [line]
        elif current_id is not None:
            current_lines.append(line)
    flush()
    return sections

issues = parse_sections(issues_path)
fixed = parse_sections(fixed_path)

# Determine reused OTA numbers (appear in both)
reused = set(issues.keys()) & set(fixed.keys())
if reused:
    print("\n  Reused OTA numbers (Issues.md takes precedence):")
    for ota in sorted(reused):
        print(f"    {ota}: Issues=\"{issues[ota]['title']}\"  Fixed=\"{fixed[ota]['title']}\"")

# ---- Step 1: Process Issues.md ----
print("\n--- Step 1: Issues.md -> DB ---")
for ota_id in sorted(issues.keys()):
    info = issues[ota_id]
    row = cur.execute("SELECT status FROM items WHERE ota_id=?", (ota_id,)).fetchone()
    if row is None:
        print(f"  INSERT {ota_id}: \"{info['title']}\" [{info['type']}, {info['status']}]")
        cur.execute(
            "INSERT INTO items (ota_id, type, status, title, description) VALUES (?, ?, ?, ?, ?)",
            (ota_id, info['type'], info['status'], info['title'], info['description'])
        )
        cur.execute(
            "INSERT INTO item_history (ota_id, by, event, reason) VALUES (?, 'AI', 'Opened', 'Migrated from Issues.md')",
            (ota_id,)
        )
    else:
        db_status = row[0]
        if db_status != info['status']:
            print(f"  UPDATE {ota_id}: status '{db_status}' -> '{info['status']}'")
            cur.execute("UPDATE items SET status=?, modified_at=datetime('now') WHERE ota_id=?", (info['status'], ota_id))
            cur.execute("INSERT INTO item_history (ota_id, by, event, reason) VALUES (?, 'AI', 'Status Update', 'Synced from Issues.md')", (ota_id,))
        else:
            print(f"  OK    {ota_id}: status '{db_status}'")

# ---- Step 2: Process Fixed.md ----
print("\n--- Step 2: Fixed.md -> DB ---")
for ota_id in sorted(fixed.keys()):
    info = fixed[ota_id]

    # Skip reused OTA numbers — Issues.md takes precedence
    if ota_id in reused:
        print(f"  SKIP  {ota_id}: reused OTA# (Issues.md entry takes precedence)")
        continue

    row = cur.execute("SELECT status FROM items WHERE ota_id=?", (ota_id,)).fetchone()
    if row is None:
        print(f"  SKIP  {ota_id}: not in DB (already migrated)")
        continue

    db_status = row[0]
    # Check if already terminal
    if any(term in db_status for term in ['-> Fixed.md', 'Fixed.md']):
        print(f"  OK    {ota_id}: already terminal as '{db_status}'")
        continue

    # Close it
    print(f"  CLOSE {ota_id}: '{db_status}' -> '{info['status']}'")
    cur.execute("UPDATE items SET status=?, modified_at=datetime('now') WHERE ota_id=?", (info['status'], ota_id))
    cur.execute("INSERT INTO item_history (ota_id, by, event, reason) VALUES (?, 'AI', ?, 'Migrated from Fixed.md')",
                (ota_id, info['status']))

db.commit()
print()

# ---- Summary ----
rows = cur.execute("SELECT ota_id, type, status, title FROM items ORDER BY ota_id").fetchall()
print("=== Migration complete ===")
print(f"{'ota_id':<10} {'type':<6} {'status':<30} {'title'}")
print("-" * 80)
for r in rows:
    print(f"{r[0]:<10} {r[1]:<6} {r[2]:<30} {r[3]}")
PYEOF
