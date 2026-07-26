#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CSV_FILE="$PROJECT_ROOT/docs/research/production_planning_20260726/gap_tracker.csv"
TSV_FILE="$PROJECT_ROOT/docs/research/production_planning_20260726/velocity.tsv"
START_DATE="2026-07-26"

if [ ! -f "$CSV_FILE" ]; then
    echo "ERROR: gap_tracker.csv not found at $CSV_FILE" >&2
    exit 1
fi

TOTAL=$(tail -n +2 "$CSV_FILE" | wc -l)

COMPLETED=0
while IFS= read -r line; do
    status=$(echo "$line" | awk -F',' '{print $5}' | tr -d '"' | tr -d ' ')
    if [ "$status" != "Queued" ] && [ -n "$status" ]; then
        COMPLETED=$((COMPLETED + 1))
    fi
done < <(tail -n +2 "$CSV_FILE")

CURRENT_DATE=$(date +%Y-%m-%d)

START_EPOCH=$(date -d "$START_DATE" +%s 2>/dev/null || echo 0)
CURRENT_EPOCH=$(date -d "$CURRENT_DATE" +%s)
DIFF_DAYS=$(( (CURRENT_EPOCH - START_EPOCH) / 86400 ))
if [ "$DIFF_DAYS" -lt 1 ]; then
    DIFF_DAYS=1
fi

WEEKS=$(echo "scale=4; $DIFF_DAYS / 7" | bc)
ITEMS_PER_WEEK=$(echo "scale=2; $COMPLETED / $WEEKS" | bc)

TSV_DIR=$(dirname "$TSV_FILE")
mkdir -p "$TSV_DIR"

if [ ! -f "$TSV_FILE" ]; then
    printf "date\tcompleted_count\ttotal_count\titems_per_week\n" > "$TSV_FILE"
fi

printf "%s\t%d\t%d\t%.2f\n" "$CURRENT_DATE" "$COMPLETED" "$TOTAL" "$ITEMS_PER_WEEK" >> "$TSV_FILE"

echo "Velocity tracking: $CURRENT_DATE | Completed: $COMPLETED/$TOTAL | Items/week: $ITEMS_PER_WEEK | Days since start: $DIFF_DAYS"
