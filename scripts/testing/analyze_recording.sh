#!/usr/bin/env bash
# ============================================================================
# analyze_recording.sh — Batch OpenCV recording analysis wrapper
# ============================================================================
# Purpose:
#   Runs analyze_recording.py on every helix_ota-*.mp4 in the recording
#   directory, aggregates results, and generates a structured evidence report.
#
# Usage:
#   ./analyze_recording.sh                                  # Scan $HOME/Downloads
#   ./analyze_recording.sh /path/to/recordings/             # Scan custom dir
#   ./analyze_recording.sh /path/to/video.mp4               # Single file
#
# Outputs:
#   qa-results/opencv-analysis-<timestamp>/                 # Per-recording JSON
#   qa-results/opencv-analysis-<timestamp>/REPORT.json      # Aggregated report
#   qa-results/opencv-analysis-<timestamp>/SUMMARY.txt      # Human-readable summary
#
# Requirements:
#   pip3 install opencv-python          # cv2
#   pip3 install pytesseract            # optional (Tesseract OCR)
#   brew install tesseract              # optional (OCR engine)
#
# Side-effects:
#   Creates output directories under qa-results/.
#
# Dependencies:
#   scripts/testing/analyze_recording.py — the OpenCV analysis engine.
#
# Cross-references:
#   docs/guides/video_recording_and_analysis.md  — user guide
#   docs/qa/  — evidence reports
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ANALYZER="$SCRIPT_DIR/analyze_recording.py"

# Recording source
RECORDING_DIR="${1:-$HOME/Downloads}"
OUTPUT_DIR="$PROJECT_ROOT/qa-results/opencv-analysis-$TIMESTAMP"

# Verify analyzer exists
if [ ! -f "$ANALYZER" ]; then
    echo "ERROR: analyzer not found at $ANALYZER"
    exit 1
fi

# Verify opencv is available
if ! python3 -c "import cv2" 2>/dev/null; then
    echo "ERROR: OpenCV (cv2) not installed."
    echo "Run: pip3 install opencv-python"
    exit 2
fi

# Check for Tesseract
if command -v tesseract &>/dev/null && python3 -c "import pytesseract" 2>/dev/null; then
    echo "Tesseract OCR available"
else
    echo "Tesseract OCR NOT available (optional, install: brew install tesseract && pip3 install pytesseract)"
fi

echo "=== Helix OTA OpenCV Recording Analysis ==="
echo "Timestamp: $TIMESTAMP"
echo "Source:    $RECORDING_DIR"
echo "Output:    $OUTPUT_DIR"
echo ""

mkdir -p "$OUTPUT_DIR"

# Find recordings
if [ -f "$RECORDING_DIR" ]; then
    VIDEOS=("$RECORDING_DIR")
elif [ -d "$RECORDING_DIR" ]; then
    # Match helix_ota-*.mp4 pattern per SS11.4.155 naming convention
    mapfile -t VIDEOS < <(find "$RECORDING_DIR" -maxdepth 1 -name "helix_ota-*.mp4" -type f 2>/dev/null | sort)
    if [ ${#VIDEOS[@]} -eq 0 ]; then
        # Fallback to any helix_ota*.mp4
        mapfile -t VIDEOS < <(find "$RECORDING_DIR" -maxdepth 1 -name "helix_ota*.mp4" -type f 2>/dev/null | sort)
    fi
    if [ ${#VIDEOS[@]} -eq 0 ]; then
        mapfile -t VIDEOS < <(find "$RECORDING_DIR" -maxdepth 1 -name "*.mp4" -type f 2>/dev/null | sort)
    fi
    echo "Found ${#VIDEOS[@]} MP4 recording(s)"
else
    echo "ERROR: Path not found: $RECORDING_DIR"
    exit 1
fi

if [ ${#VIDEOS[@]} -eq 0 ]; then
    echo "No MP4 recordings found."
    exit 0
fi

# Run analysis per-video, writing sidecar JSON files to output dir
AGGREGATED_JSON="$OUTPUT_DIR/REPORT.json"
for vpath in "${VIDEOS[@]}"; do
    fname="$(basename "$vpath")"
    echo "  Processing: $fname ..."
    python3 "$ANALYZER" "$vpath" --output-dir "$OUTPUT_DIR" 2>&1 | tee -a "$OUTPUT_DIR/ANALYSIS_LOG.txt" || true
done

# Build aggregated JSON by combining sidecar files
echo "[" > "$AGGREGATED_JSON"
first=true
for vpath in "${VIDEOS[@]}"; do
    fname="$(basename "$vpath")"
    sidecar="${OUTPUT_DIR}/${fname%.mp4}.analysis.json"
    if [ -f "$sidecar" ]; then
        if [ "$first" = true ]; then
            first=false
        else
            echo "," >> "$AGGREGATED_JSON"
        fi
        cat "$sidecar" >> "$AGGREGATED_JSON"
    fi
done
echo "]" >> "$AGGREGATED_JSON"

# Generate summary
python3 -c "
import json
with open('$AGGREGATED_JSON') as f:
    results = json.load(f)
total = len(results)
if total == 0:
    print('No results to summarize.')
    exit(0)
passed = sum(1 for r in results if r.get('verdict','').startswith('PASS'))
failed = sum(1 for r in results if r.get('verdict','').startswith('FAIL'))
errors = sum(1 for r in results if r.get('verdict') == 'ERROR')
ocr_avail = any(r.get('text_regions',{}).get('ocr_available') for r in results)
print()
print('=' * 70)
print('OPENCV RECORDING ANALYSIS — AGGREGATED RESULTS')
print('=' * 70)
print(f'  Total recordings:    {total}')
print(f'  PASS:                {passed}')
print(f'  FAIL:                {failed}')
print(f'  ERROR:               {errors}')
print(f'  Tesseract OCR:       {\"AVAILABLE\" if ocr_avail else \"NOT AVAILABLE\"}')
print(f'  PASS rate:           {passed/total*100:.1f}%' if total > 0 else f'')
print()
print('  Per-recording results:')
for r in results:
    fname = r.get('file','?')
    verdict = r.get('verdict','?')
    frames = r.get('total_frames',0)
    dur = r.get('duration_s',0)
    res = r.get('resolution',{})
    freeze = r.get('freeze',{}).get('freeze_ratio',0)
    txt = r.get('text_regions',{}).get('max_count',0)
    print(f'    {fname:<45} {verdict:<30} {frames:>4}f {dur:>6.1f}s {res.get(\"width\",\"?\")}x{res.get(\"height\",\"?\")} f{freeze:.0%} t{txt}')
    if r.get('error'):
        print(f'      ERROR: {r[\"error\"]}')
print()
has_fail = any(r.get('verdict','').startswith('FAIL') or r.get('verdict') == 'ERROR' for r in results)
if has_fail:
    print('  FAIL details:')
    for r in results:
        if r.get('verdict','').startswith('FAIL') or r.get('verdict') == 'ERROR':
            print(f'    - {r.get(\"file\",\"?\")}: {r.get(\"verdict\",\"?\")}')
            if r.get('error'):
                print(f'      {r[\"error\"]}')
    print()
" 2>&1 | tee "$OUTPUT_DIR/SUMMARY.txt"

echo ""
echo "Analysis complete. Results in: $OUTPUT_DIR"
