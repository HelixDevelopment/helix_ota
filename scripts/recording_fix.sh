#!/usr/bin/env bash
# =============================================================================
# recording_fix.sh — Post-process asciinema .cast → proper H.264 MP4
# -----------------------------------------------------------------------------
# Purpose: Convert asciinema terminal recordings to §11.4.159-compliant
#          H.264 yuv420p MP4 files.  agg only produces GIF; this pipeline
#          converts the agg GIF output to a proper H.264 video.
#
# Usage:
#   scripts/recording_fix.sh <cast-file> [output.mp4]
#
#   If output.mp4 is omitted, replaces the .cast file's extension with .mp4
#   in the same directory.
#
# Dependencies: agg, ffmpeg
# Context: §11.4.159 (window-specific video recording + vision validation)
#   + §11.4.160 (vision-verified recording bridge)
# =============================================================================
set -euo pipefail

if [ $# -lt 1 ]; then
    echo "Usage: $0 <cast-file> [output.mp4]"
    exit 1
fi

CAST="$1"
OUT="${2:-${CAST%.cast}.mp4}"

# Phase 1: Check dependencies
for cmd in agg ffmpeg; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "ERROR: $cmd not found — install via brew / apt / dnf"
        exit 1
    fi
done

if [ ! -f "$CAST" ]; then
    echo "ERROR: cast file not found: $CAST"
    exit 1
fi

TMP_GIF=$(mktemp /tmp/recording_fix_XXXXXX.gif)
trap 'rm -f "$TMP_GIF"' EXIT

echo "Converting $(basename "$CAST") → $(basename "$OUT")..."

# Step 1: agg → GIF (intermediate)
if ! agg "$CAST" "$TMP_GIF" 2>/dev/null; then
    echo "ERROR: agg failed on $CAST"
    exit 1
fi

# Step 2: ffmpeg GIF → H.264 yuv420p MP4
if ! ffmpeg -y -i "$TMP_GIF" \
    -c:v libx264 -pix_fmt yuv420p -preset ultrafast -crf 23 \
    -movflags +faststart \
    "$OUT" 2>/dev/null; then
    echo "ERROR: ffmpeg conversion failed"
    exit 1
fi

echo "OK: $OUT ($(du -h "$OUT" | awk '{print $1}'))"
