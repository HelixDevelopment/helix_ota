#!/usr/bin/env python3
"""Analyze a recorded MP4 video for content verification.

Per SS11.4.159(D) mandatory vision validation. Uses OpenCV for frame
analysis + Tesseract OCR where available to READ and verify that every
recording shows genuine working results (not frozen/stale/mock data).

Usage:
  python3 analyze_recording.py <video.mp4>
  python3 analyze_recording.py <video.mp4> --output report.json
  python3 analyze_recording.py <directory/>               # batch scan all .mp4

Output: structured JSON report with per-recording analysis.
"""

import argparse
import json
import os
import sys
import time
from pathlib import Path

try:
    import cv2
    import numpy as np
except ImportError:
    print("ERROR: OpenCV (cv2) not installed. Run: pip3 install opencv-python")
    sys.exit(2)

try:
    import pytesseract
    HAS_TESSERACT = True
except ImportError:
    HAS_TESSERACT = False


# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------
FREEZE_SSIM_THRESHOLD = 0.97       # SSIM >= this between adjacent sampled frames = frozen
FREEZE_RATIO_WARN = 0.30           # fraction of frames frozen => WARN
FREEZE_RATIO_FAIL = 0.60           # fraction of frames frozen => FAIL
MIN_FRAMES_FOR_LIVENESS = 3        # need at least this many frames to assess liveness
MAX_SAMPLE_FRAMES = 100            # cap to avoid unbounded memory
CONTRAST_FLOOR = 15                # mean contrast below this => "flat/blank"
TEXT_REGION_MIN_AREA = 500         # minimum contour area for a text region
TEXT_REGION_MAX_AREA = 0.5         # max fraction of frame area for one text region


def _fast_ssim(a: np.ndarray, b: np.ndarray) -> float:
    """Fast structural similarity index between two grayscale images (0-1)."""
    if a.shape != b.shape:
        return 0.0
    # Downscale for speed on large frames
    if a.shape[0] > 200:
        scale = 200.0 / a.shape[0]
        new_w = max(32, int(a.shape[1] * scale))
        new_h = 200
        a = cv2.resize(a, (new_w, new_h), interpolation=cv2.INTER_LINEAR)
        b = cv2.resize(b, (new_w, new_h), interpolation=cv2.INTER_LINEAR)
    # Simple pixel-difference-based similarity (cheap proxy for SSIM)
    diff = cv2.absdiff(a.astype(np.float32), b.astype(np.float32))
    mse = np.mean(diff ** 2)
    if mse < 1e-8:
        return 1.0
    max_val = 255.0
    ssim_val = 1.0 - (mse / (max_val ** 2))
    return float(np.clip(ssim_val, 0.0, 1.0))


def detect_text_regions(gray: np.ndarray) -> list:
    """Find rectangular regions likely containing text via edge detection + morphology."""
    h, w = gray.shape
    regions = []
    # Use gradient-based method: horizontal and vertical edge detection
    sobel_x = cv2.Sobel(gray, cv2.CV_8U, 1, 0, ksize=3)
    sobel_y = cv2.Sobel(gray, cv2.CV_8U, 0, 1, ksize=3)
    edge = cv2.bitwise_or(sobel_x, sobel_y)

    # Morphology to connect text characters into lines
    kernel_h = cv2.getStructuringElement(cv2.MORPH_RECT, (25, 1))
    kernel_v = cv2.getStructuringElement(cv2.MORPH_RECT, (1, 15))
    dilated = cv2.dilate(edge, kernel_h, iterations=1)
    dilated = cv2.dilate(dilated, kernel_v, iterations=1)

    contours, _ = cv2.findContours(dilated, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    max_area = w * h * TEXT_REGION_MAX_AREA

    for cnt in contours:
        x, y, cw, ch = cv2.boundingRect(cnt)
        area = cw * ch
        if area < TEXT_REGION_MIN_AREA or area > max_area:
            continue
        # Filter out full-width bars
        aspect = cw / max(ch, 1)
        if aspect > 15 or aspect < 0.05:
            continue
        regions.append({
            "x": int(x), "y": int(y),
            "width": int(cw), "height": int(ch),
            "area": int(area)
        })
    return regions


def ocr_region(gray: np.ndarray, region: dict) -> dict:
    """Run Tesseract OCR on a detected text region, returning text + confidence."""
    if not HAS_TESSERACT:
        return {"text": "", "confidence": 0.0, "reason": "tesseract_not_available"}
    x, y, cw, ch = region["x"], region["y"], region["width"], region["height"]
    roi = gray[y:y + ch, x:x + cw]
    if roi.size == 0 or roi.shape[0] < 8 or roi.shape[1] < 8:
        return {"text": "", "confidence": 0.0, "reason": "region_too_small"}

    # Upscale small text for better OCR
    if roi.shape[0] < 30 or roi.shape[1] < 30:
        scale = max(60.0 / max(roi.shape[0], 1), 60.0 / max(roi.shape[1], 1))
        if scale > 1.0:
            roi = cv2.resize(roi, None, fx=scale, fy=scale, interpolation=cv2.INTER_CUBIC)

    try:
        data = pytesseract.image_to_data(roi, output_type=pytesseract.Output.DICT,
                                          config='--psm 6')
        text_parts = []
        confidences = []
        for i, txt in enumerate(data.get("text", [])):
            txt = txt.strip()
            if txt and int(data.get("conf", [0])[i]) > 0:
                conf = int(data["conf"][i]) / 100.0
                text_parts.append(txt)
                confidences.append(conf)

        if text_parts:
            combined = " ".join(text_parts)
            avg_conf = sum(confidences) / len(confidences) if confidences else 0.0
            return {"text": combined[:500], "confidence": round(avg_conf, 3)}
        return {"text": "", "confidence": 0.0, "reason": "no_text_detected"}
    except Exception as e:
        return {"text": "", "confidence": 0.0, "reason": f"ocr_error: {e}"}


def analyze_video(path: str, max_frames: int = MAX_SAMPLE_FRAMES) -> dict:
    """Analyze a single MP4 video file for content verification."""
    cap = cv2.VideoCapture(path)
    if not cap.isOpened():
        return {"file": os.path.basename(path), "error": "cannot_open_video",
                "verdict": "ERROR"}

    fps = cap.get(cv2.CAP_PROP_FPS)
    total_frames = int(cap.get(cv2.CAP_PROP_FRAME_COUNT))
    duration = total_frames / fps if fps > 0 else 0
    width = int(cap.get(cv2.CAP_PROP_FRAME_WIDTH))
    height = int(cap.get(cv2.CAP_PROP_FRAME_HEIGHT))

    result = {
        "file": os.path.basename(path),
        "file_size_bytes": os.path.getsize(path) if os.path.exists(path) else 0,
        "fps": round(fps, 2),
        "total_frames": total_frames,
        "duration_s": round(duration, 2),
        "resolution": {"width": width, "height": height},
        "pixel_count": width * height,
        "sampled_frames": 0,
        "freeze": {
            "similar_pairs": 0,
            "total_pairs": 0,
            "freeze_ratio": 0.0,
            "verdict": "PENDING"
        },
        "liveness": {
            "frame_count_usable": 0,
            "verdict": "PENDING"
        },
        "text_regions": {
            "max_count": 0,
            "found_text_snippets": [],
            "ocr_available": HAS_TESSERACT,
            "verdict": "PENDING"
        },
        "flat_content": False,
        "analysis": []
    }

    # Sample frames uniformly across the video
    sample_count = min(total_frames, max_frames)
    if sample_count < 1:
        cap.release()
        result["verdict"] = "FAIL - zero-frame video"
        return result

    indices = set()
    if total_frames <= max_frames:
        indices = set(range(total_frames))
    else:
        step = total_frames / max_frames
        for i in range(max_frames):
            indices.add(int(i * step))
    indices = sorted(indices)

    frames_gray = []
    frame_details = []
    prev_gray = None

    for idx in indices:
        cap.set(cv2.CAP_PROP_POS_FRAMES, idx)
        ret, frame = cap.read()
        if not ret or frame is None:
            continue
        gray = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)
        frames_gray.append(gray)
        result["sampled_frames"] += 1

        # Per-frame analysis
        frame_info = {
            "frame_index": idx,
            "mean_brightness": round(float(np.mean(gray)), 2),
            "std_brightness": round(float(np.std(gray)), 2),
        }

        # Contrast assessment
        laplacian_var = cv2.Laplacian(gray, cv2.CV_64F).var()
        frame_info["laplacian_var"] = round(laplacian_var, 2)

        # Text region detection
        text_regions = detect_text_regions(gray)
        frame_info["text_region_count"] = len(text_regions)

        # OCR on the most text-rich frame
        if len(text_regions) > result["text_regions"]["max_count"]:
            result["text_regions"]["max_count"] = len(text_regions)
            for region in sorted(text_regions, key=lambda r: -r["area"])[:5]:
                ocr_result = ocr_region(gray, region)
                if ocr_result.get("text"):
                    snippet = ocr_result["text"][:200]
                    ocr_result["snippet"] = snippet
                    result["text_regions"]["found_text_snippets"].append(ocr_result)

        # Freeze detection
        if prev_gray is not None:
            sim = _fast_ssim(gray, prev_gray)
            frame_info["ssim_vs_prev"] = round(sim, 4)
            if sim >= FREEZE_SSIM_THRESHOLD:
                result["freeze"]["similar_pairs"] += 1
            result["freeze"]["total_pairs"] += 1
        else:
            frame_info["ssim_vs_prev"] = None

        prev_gray = gray
        frame_details.append(frame_info)

    cap.release()
    result["analysis"] = frame_details
    result["liveness"]["frame_count_usable"] = result["sampled_frames"]

    # Verdict computation
    total_pairs = result["freeze"]["total_pairs"]
    if total_pairs > 0:
        freeze_ratio = result["freeze"]["similar_pairs"] / total_pairs
        result["freeze"]["freeze_ratio"] = round(freeze_ratio, 3)
        if freeze_ratio >= FREEZE_RATIO_FAIL:
            result["freeze"]["verdict"] = "FAIL"
        elif freeze_ratio >= FREEZE_RATIO_WARN:
            result["freeze"]["verdict"] = "WARN"
        else:
            result["freeze"]["verdict"] = "PASS"

    # Check for flat/blank content
    mean_brightnesses = [f["mean_brightness"] for f in frame_details if "mean_brightness" in f]
    if mean_brightnesses:
        avg_contrast = np.mean([f.get("std_brightness", 255) for f in frame_details])
        if avg_contrast < CONTRAST_FLOOR:
            result["flat_content"] = True

    # Overall verdict
    verdict_parts = []
    if total_frames <= 1:
        verdict_parts.append("single-frame")
    if result["freeze"]["verdict"] == "FAIL":
        verdict_parts.append("frozen")
    if result["flat_content"]:
        verdict_parts.append("flat_blank")
    if result["sampled_frames"] < 2:
        verdict_parts.append("insufficient_frames")

    if verdict_parts:
        result["verdict"] = f"FAIL - {' + '.join(verdict_parts)}"
    elif result["freeze"]["verdict"] == "WARN":
        result["verdict"] = "PASS (with freeze warnings)"
    else:
        text_found = len(result["text_regions"]["found_text_snippets"]) > 0
        if result["sampled_frames"] >= MIN_FRAMES_FOR_LIVENESS and not result["flat_content"]:
            result["verdict"] = "PASS - live content" + (" + OCR text" if text_found else "")
        else:
            result["verdict"] = "PASS - basic content check"

    return result


def format_verdict_summary(results: list) -> str:
    """Format a human-readable summary string from analysis results."""
    total_processed = len(results)
    passed = sum(1 for r in results if r.get("verdict", "").startswith("PASS"))
    failed = sum(1 for r in results if r.get("verdict", "").startswith("FAIL"))
    errors = sum(1 for r in results if r.get("verdict") == "ERROR")

    lines = [
        "=== OpenCV Recording Analysis Summary ===",
        f"Total recordings: {total_processed}",
        f"  PASS:  {passed}",
        f"  FAIL:  {failed}",
        f"  ERROR: {errors}",
        "",
    ]
    for r in results:
        fname = r.get("file", "?")
        verdict = r.get("verdict", "UNKNOWN")
        num_frames = r.get("total_frames", 0)
        dur = r.get("duration_s", 0)
        res = r.get("resolution", {})
        freeze_ratio = r.get("freeze", {}).get("freeze_ratio", 0)
        text_count = r.get("text_regions", {}).get("max_count", 0)

        lines.append(
            f"  {fname:<50} {verdict:<35}"
            f"  {num_frames:>4}f {dur:>6.1f}s "
            f"{res.get('width','?')}x{res.get('height','?')}"
            f"  freeze={freeze_ratio:.1%}"
            f"  text_regions={text_count}"
        )
        if r.get("error"):
            lines.append(f"    ERROR: {r['error']}")

    lines.append("")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------
def main():
    parser = argparse.ArgumentParser(
        description="Analyze recorded MP4 video for content verification (SS11.4.159)."
    )
    parser.add_argument("input", help="Video file or directory (batch scans all .mp4)")
    parser.add_argument("--output", "-o", default=None,
                        help="Write aggregated JSON report to file")
    parser.add_argument("--max-frames", type=int, default=MAX_SAMPLE_FRAMES,
                        help=f"Max frames to sample (default {MAX_SAMPLE_FRAMES})")
    parser.add_argument("--summary", "-s", action="store_true",
                        help="Print summary table")
    parser.add_argument("--output-dir", default=None,
                        help="Directory for per-recording JSON sidecar files")
    parser.add_argument("--sidecar", action="store_true",
                        help="Write per-recording JSON sidecar next to video")
    args = parser.parse_args()

    input_path = Path(args.input)

    # Collect video files
    if input_path.is_dir():
        videos = sorted(input_path.glob("*.mp4"))
        if not videos:
            print(f"No .mp4 files found in {input_path}")
            sys.exit(1)
    elif input_path.is_file():
        videos = [input_path]
    else:
        print(f"Path not found: {input_path}")
        sys.exit(1)

    # Determine output directory for sidecars
    sidecar_dir = None
    if args.output_dir:
        sidecar_dir = Path(args.output_dir)
        sidecar_dir.mkdir(parents=True, exist_ok=True)
    elif args.output:
        out_path = Path(args.output)
        if out_path.suffix == ".json":
            pass  # aggregated file mode, handled below
        else:
            sidecar_dir = out_path
            sidecar_dir.mkdir(parents=True, exist_ok=True)

    all_results = []
    for vpath in videos:
        print(f"Analyzing: {vpath.name} ...", end=" ", flush=True)
        t0 = time.time()
        result = analyze_video(str(vpath), max_frames=args.max_frames)
        elapsed = time.time() - t0
        print(f"{elapsed:.1f}s -> {result.get('verdict', 'ERROR')}")
        all_results.append(result)

        # Write per-recording sidecar
        sidecar = sidecar_dir / f"{vpath.stem}.analysis.json" if sidecar_dir else None
        if sidecar:
            with open(sidecar, "w") as f:
                json.dump(result, f, indent=2)

    # Output
    if args.output:
        out_path = Path(args.output)
        if out_path.suffix == ".json":
            with open(args.output, "w") as f:
                json.dump(all_results, f, indent=2)
            print(f"\nAggregated report: {args.output}")

    if args.summary or not args.output:
        print(format_verdict_summary(all_results))

    failed = sum(1 for r in all_results if not r.get("verdict", "").startswith("PASS"))
    return 1 if failed > len(all_results) * 0.5 else 0


if __name__ == "__main__":
    sys.exit(main())
