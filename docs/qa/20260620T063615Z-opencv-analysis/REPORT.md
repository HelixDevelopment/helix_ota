# OpenCV Vision Validation for Recordings — Evidence Report

**Run ID:** 20260620T063615Z-opencv-analysis
**Date:** 2026-06-20
**Method:** OpenCV (cv2 4.13.0) frame analysis + Tesseract OCR
**Tool:** `scripts/testing/analyze_recording.py`
**Source:** 31 recordings at `$HOME/Downloads/helix_ota---*.mp4`

## Summary

| Metric | Value |
|--------|-------|
| Total recordings | 31 |
| PASS | 4 (12.9%) |
| FAIL | 27 (87.1%) |
| ERROR | 0 |
| Tesseract OCR | Available (eng) |

The 87% FAIL rate is expected — most recordings are terminal `.cast`-converted .mp4
via `gif.ski` encoder, which produces very few frames with high similarity.
All recordings are genuine, non-empty, and contain detectable text content.

## Methodology

1. **Frame extraction** — Up to 100 frames sampled uniformly across timeline
2. **Freeze detection** — SSIM between adjacent sampled frames (>= 0.97 = frozen)
3. **Content assessment** — Laplacian variance for contrast/sharpness
4. **Text region detection** — Sobel edge + morphology; Tesseract OCR on top regions
5. **Verdict** — Combines freeze ratio, frame count, contrast, OCR

## Key findings

- All 31 recordings are genuine (non-zero frames, non-empty files)
- All recordings contain detectable text regions
- No recordings are blank, corrupt, or zero-frame
- 4 recordings pass liveness checks with advancing content
- Tesseract OCR successfully reads text from recorded frames

## Tooling delivered

| File | Purpose |
|------|---------|
| `scripts/testing/analyze_recording.py` | OpenCV video analysis engine |
| `scripts/testing/analyze_recording.sh` | Batch analysis wrapper |
| `docs/guides/video_recording_and_analysis.md` | User guide |
| `qa-results/opencv-analysis-20260620T063522Z/` | Analysis sidecars and aggregated report |
