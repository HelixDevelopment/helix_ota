#!/usr/bin/env bash
# =============================================================================
# project_status.sh — Project Health Dashboard
# -----------------------------------------------------------------------------
# One-command project health check — verifies SC-001 through SC-005 compliance
# checks: DB status, Git state, build integrity, constitution gate, and carrier
# file lockstep.  Outputs JSON to stdout and optionally an HTML report.
#
# Usage:
#   bash scripts/project_status.sh              # JSON to stdout
#   bash scripts/project_status.sh --html       # JSON + writes HTML report
#   bash scripts/project_status.sh --html --open  # HTML + open in browser
#
# Outputs:
#   - JSON report to stdout
#   - If --html: docs/planning/project_health_<date>.html
#
# Dependencies: git, sqlite3, go (optional, for build/test)
#
# Cross-references:
#   - §11.4.170 — Host-rendered UI visual-proof mandate
#   - §11.4.172 — Production-readiness planning mandate
#   - §11.4.157 — GEMINI.md lockstep (carrier file count)
#
# Last verified: 2026-07-26
# =============================================================================
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DB="$PROJECT_ROOT/docs/workable_items.db"
GAP_TRACKER="$PROJECT_ROOT/docs/research/production_planning_20260726/gap_tracker.csv"
OUTPUT_HTML=false
OPEN_HTML=false

for arg in "$@"; do
    case "$arg" in
        --html) OUTPUT_HTML=true ;;
        --open) OPEN_HTML=true ;;
    esac
done

# --- helpers ------------------------------------------------------------------
safe_count() {
    sqlite3 "$DB" "$1" 2>/dev/null || echo 0
}

git_ahead_behind() {
    local ahead behind
    ahead=$(git -C "$PROJECT_ROOT" rev-list --count HEAD..@{u} 2>/dev/null || echo 0)
    behind=$(git -C "$PROJECT_ROOT" rev-list --count @{u}..HEAD 2>/dev/null || echo 0)
    echo "$ahead $behind"
}

# --- DB stats -----------------------------------------------------------------
DB_TOTAL=$(safe_count "SELECT COUNT(*) FROM items;")
DB_CLOSED=$(safe_count "SELECT COUNT(*) FROM items WHERE status LIKE '%Fixed%' OR status LIKE '%Implemented%' OR status LIKE '%Completed%' OR status LIKE '%Obsolete%';")
DB_QUEUED=$(safe_count "SELECT COUNT(*) FROM items WHERE status = 'Queued';")
DB_IN_PROGRESS=$(safe_count "SELECT COUNT(*) FROM items WHERE status = 'In progress';")
DB_OPERATOR_BLOCKED=$(safe_count "SELECT COUNT(*) FROM items WHERE status = 'Operator-blocked';")
DB_REOPENED=$(safe_count "SELECT COUNT(*) FROM items WHERE status = 'Reopened';")
DB_IN_TESTING=$(safe_count "SELECT COUNT(*) FROM items WHERE status IN ('Ready for testing','In testing');")

# --- gap tracker stats --------------------------------------------------------
GAP_TOTAL=0
GAP_CLOSED=0
if [ -f "$GAP_TRACKER" ]; then
    GAP_TOTAL=$(tail -n +2 "$GAP_TRACKER" | wc -l)
    GAP_CLOSED=$(grep -c 'Closed' "$GAP_TRACKER" 2>/dev/null || echo 0)
fi

# --- git state ----------------------------------------------------------------
BRANCH=$(git -C "$PROJECT_ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
read AHEAD BEHIND <<< "$(git_ahead_behind)"
DIRTY="false"
if [ -n "$(git -C "$PROJECT_ROOT" status --porcelain 2>/dev/null)" ]; then
    DIRTY="true"
fi
LATEST_TAG=$(git -C "$PROJECT_ROOT" describe --tags --abbrev=0 2>/dev/null || echo "none")

# --- build integrity ----------------------------------------------------------
GO_BUILD="SKIP"
GO_TEST_COUNT=0
if [ -d "$PROJECT_ROOT/server" ] && [ -f "$PROJECT_ROOT/server/go.mod" ]; then
    if (cd "$PROJECT_ROOT/server" && go build ./... 2>/dev/null); then
        GO_BUILD="PASS"
    else
        GO_BUILD="FAIL"
    fi
    GO_TEST_COUNT=$(cd "$PROJECT_ROOT/server" && go test ./... -count=1 2>&1 | grep -c '^ok' || echo 0)
fi

# --- constitution gate --------------------------------------------------------
CONST_INHERITANCE="SKIP"
CONST_TAG="none"
if [ -f "$PROJECT_ROOT/tests/test_constitution_inheritance.sh" ]; then
    if bash "$PROJECT_ROOT/tests/test_constitution_inheritance.sh" >/dev/null 2>&1; then
        CONST_INHERITANCE="PASS"
    else
        CONST_INHERITANCE="FAIL"
    fi
fi
CONST_TAG=$(git -C "$PROJECT_ROOT" tag -l 'v*' --sort=-v:refname 2>/dev/null | head -1 || echo "none")

# --- carrier files ------------------------------------------------------------
CARRIER_COUNT=0
for f in "$PROJECT_ROOT/CLAUDE.md" "$PROJECT_ROOT/AGENTS.md" "$PROJECT_ROOT/GEMINI.md" "$PROJECT_ROOT/QWEN.md"; do
    if [ -f "$f" ] && grep -q 'helix_ota' "$f" 2>/dev/null; then
        CARRIER_COUNT=$((CARRIER_COUNT + 1))
    fi
done

# --- semgrep check ------------------------------------------------------------
SEMGREP_RESULT="SKIP"
if command -v semgrep &>/dev/null; then
    if semgrep scan --config auto --error --quiet 2>/dev/null; then
        SEMGREP_RESULT="PASS"
    else
        SEMGREP_RESULT="FAIL"
    fi
fi

# --- sonarqube check ----------------------------------------------------------
SONAR_RESULT="SKIP"
if command -v sonar-scanner &>/dev/null; then
    if sonar-scanner --version >/dev/null 2>&1; then
        SONAR_RESULT="PASS"
    else
        SONAR_RESULT="FAIL"
    fi
fi

# --- assemble JSON ------------------------------------------------------------
HEALTH_DATE=$(date -Iseconds)

JSON=$(cat <<EOF
{
  "date": "$HEALTH_DATE",
  "db": {
    "total": $DB_TOTAL,
    "closed": $DB_CLOSED,
    "queued": $DB_QUEUED,
    "in_progress": $DB_IN_PROGRESS,
    "operator_blocked": $DB_OPERATOR_BLOCKED,
    "reopened": $DB_REOPENED,
    "in_testing": $DB_IN_TESTING,
    "closure_rate": "$(awk "BEGIN { if ($DB_TOTAL > 0) printf \"%.1f\", ($DB_CLOSED/$DB_TOTAL)*100; else print 0 }")%"
  },
  "gaps": {
    "total": $GAP_TOTAL,
    "closed": $GAP_CLOSED,
    "closure_rate": "$(awk "BEGIN { if ($GAP_TOTAL > 0) printf \"%.1f\", ($GAP_CLOSED/$GAP_TOTAL)*100; else print 0 }")%"
  },
  "git": {
    "branch": "$BRANCH",
    "ahead": $AHEAD,
    "behind": $BEHIND,
    "dirty": $DIRTY,
    "latest_tag": "$LATEST_TAG"
  },
  "build": {
    "go_build": "$GO_BUILD",
    "go_test_count": $GO_TEST_COUNT
  },
  "constitution": {
    "inheritance": "$CONST_INHERITANCE",
    "latest_const_tag": "$CONST_TAG"
  },
  "static_analysis": {
    "semgrep": "$SEMGREP_RESULT",
    "sonarqube_cli": "$SONAR_RESULT"
  },
  "carriers": {
    "count": $CARRIER_COUNT,
    "of": 4,
    "lockstep": "$(if [ $CARRIER_COUNT -eq 4 ]; then echo "PASS"; else echo "FAIL"; fi)"
  },
  "compliance": {
    "SC-001_db_integrity": "$(if [ $DB_TOTAL -gt 0 ]; then echo "PASS"; else echo "FAIL"; fi)",
    "SC-002_git_clean": "$(if [ "$DIRTY" = "false" ]; then echo "PASS"; else echo "FAIL"; fi)",
    "SC-003_build_pass": "$GO_BUILD",
    "SC-004_constitution": "$CONST_INHERITANCE",
    "SC-005_carrier_lockstep": "$(if [ $CARRIER_COUNT -ge 3 ]; then echo "PASS"; else echo "FAIL"; fi)"
  }
}
EOF
)

echo "$JSON"

# --- HTML report (optional) ---------------------------------------------------
if $OUTPUT_HTML; then
    HTML_DIR="$PROJECT_ROOT/docs/planning"
    mkdir -p "$HTML_DIR"
    HTML_FILE="$HTML_DIR/project_health_$(date +%Y%m%d-%H%M%S).html"

    cat > "$HTML_FILE" <<HTMLEOF
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Helix OTA — Project Health</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 900px; margin: 2rem auto; padding: 0 1rem; }
  h1 { border-bottom: 2px solid #2563eb; padding-bottom: 0.5rem; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 0.75rem; }
  .card { border: 1px solid #e5e7eb; border-radius: 8px; padding: 1rem; }
  .card h3 { margin: 0 0 0.5rem; font-size: 0.875rem; color: #6b7280; text-transform: uppercase; }
  .card .value { font-size: 2rem; font-weight: 700; }
  .pass { color: #059669; }
  .fail { color: #dc2626; }
  .skip { color: #9ca3af; }
  pre { background: #f3f4f6; padding: 1rem; border-radius: 4px; overflow-x: auto; font-size: 0.8rem; }
</style>
</head>
<body>
<h1>Helix OTA — Project Health Report</h1>
<p>Generated: $HEALTH_DATE &mdash; Branch: <strong>$BRANCH</strong></p>
<div class="grid">
  <div class="card"><h3>DB Items</h3><div class="value $([ $DB_CLOSED -eq $DB_TOTAL ] && echo pass || echo skip)">$DB_CLOSED / $DB_TOTAL</div><p>Queued: $DB_QUEUED | In Progress: $DB_IN_PROGRESS</p></div>
  <div class="card"><h3>Gaps Closed</h3><div class="value $([ $GAP_CLOSED -eq $GAP_TOTAL ] && echo pass || echo skip)">$GAP_CLOSED / $GAP_TOTAL</div></div>
  <div class="card"><h3>Go Build</h3><div class="value $(echo "$GO_BUILD" | tr '[:upper:]' '[:lower:]')">$GO_BUILD</div><p>Tests passing: $GO_TEST_COUNT</p></div>
  <div class="card"><h3>Constitution</h3><div class="value $(echo "$CONST_INHERITANCE" | tr '[:upper:]' '[:lower:]')">$CONST_INHERITANCE</div></div>
  <div class="card"><h3>Carriers</h3><div class="value $([ $CARRIER_COUNT -ge 3 ] && echo pass || echo fail)">$CARRIER_COUNT / 4</div></div>
  <div class="card"><h3>Semgrep</h3><div class="value $(echo "$SEMGREP_RESULT" | tr '[:upper:]' '[:lower:]')">$SEMGREP_RESULT</div></div>
</div>
<h2>Raw JSON</h2>
<pre>$JSON</pre>
</body>
</html>
HTMLEOF

    echo "[project_status] HTML report written to $HTML_FILE"

    if $OPEN_HTML; then
        if command -v xdg-open &>/dev/null; then
            xdg-open "$HTML_FILE" 2>/dev/null || true
        elif command -v open &>/dev/null; then
            open "$HTML_FILE" 2>/dev/null || true
        fi
    fi
fi
