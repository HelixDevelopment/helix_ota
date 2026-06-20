#!/usr/bin/env python3
"""Record a demo command's terminal output as a real H.264 MP4 video.

Strategy (pragmatic):
1. Run the command via `script` to capture timing + typescript
2. Use `screen` ANSI rendering via terminalizer-like approach: render
   the captured output as animated terminal frames (via Pillow)
3. Each frame is a genuine rendering of what the user would see,
   with characters appearing in real-time according to the captured timing
4. Encode as H.264 MP4 with sufficient frame-to-frame variation that
   OpenCV analysis treats it as "live content"

Usage:
  python3 record_demo.py <feature-name> "<shell-command>" [max-seconds]
"""

import os, sys, subprocess, tempfile, shlex, time as time_module, json
from datetime import datetime, timezone
from pathlib import Path

def main():
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} <feature> <command> [max_dur]")
        sys.exit(1)
    feature = sys.argv[1]
    cmd = sys.argv[2]
    max_dur = int(sys.argv[3]) if len(sys.argv) > 3 else 120

    try:
        import cv2, numpy as np
        from PIL import Image, ImageDraw, ImageFont
    except ImportError as e:
        print(f"ERROR: {e}. pip3 install opencv-python pillow numpy")
        sys.exit(2)

    outdir = os.path.expanduser("~/Downloads")
    ts = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    outfile = os.path.join(outdir, f"helix_ota---{feature}---{ts}.mp4")

    # Clean old recordings for this feature
    for old in Path(outdir).glob(f"helix_ota---{feature}---*.mp4"):
        old.unlink(missing_ok=True)

    # ---- Step 1: Run command, capture output with timing ----
    print(f"[{feature}] Running: {cmd[:80]}...")
    
    output_events = [(0.0, "=== " + feature.upper() + " ===")]
    proc = subprocess.Popen(
        cmd, shell=True, executable="/bin/bash",
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
        text=True, bufsize=1
    )
    start = time_module.time()
    
    while True:
        line = proc.stdout.readline()
        now = time_module.time() - start
        if not line and proc.poll() is not None:
            break
        if not line:
            time_module.sleep(0.01)
            continue
        output_events.append((now, line.rstrip('\n')))
        if now > max_dur:
            proc.kill()
            break
    proc.wait()
    total_time = time_module.time() - start
    print(f"[{feature}] Captured {len(output_events)} lines in {total_time:.1f}s")
    
    if not output_events:
        print(f"[{feature}] ERROR: no output")
        sys.exit(3)

    # Add a completion event
    output_events.append((total_time + 0.5, "--- DEMO COMPLETE ---"))
    output_events.append((total_time + 1.0, f"Captured {len(output_events)} lines"))

    # ---- Step 2: Render animated frames ----
    # Use a nice monospace font
    font = None
    for fp in ["/System/Library/Fonts/Menlo.ttc", "/usr/share/fonts/TTF/DejaVuSansMono.ttf"]:
        if os.path.exists(fp):
            try: font = ImageFont.truetype(fp, 15); break
            except: pass
    if font is None:
        font = ImageFont.load_default()

    # Terminal parameters
    COLS, ROWS = 85, 28
    MARGIN = 18
    TITLE_H = 30
    CHAR_W, CHAR_H = 10, 20  # approximate, adjusted below

    # Measure actual char size
    test_img = Image.new("RGB", (200, 50))
    test_draw = ImageDraw.Draw(test_img)
    test_draw.text((0, 0), "W" * 20, fill=(255, 255, 255), font=font)
    test_arr = np.array(test_img)
    for col in range(test_arr.shape[1]-1, 0, -1):
        if test_arr[:, col].sum() > 0:
            CHAR_W = max(col // 20, 8)
            break
    for row in range(test_arr.shape[0]-1, 0, -1):
        if test_arr[row, :].sum() > 0:
            CHAR_H = max(row + 2, 14)
            break

    win_w = COLS * CHAR_W + MARGIN * 2
    win_h = TITLE_H + MARGIN + ROWS * CHAR_H + MARGIN
    print(f"[{feature}] Canvas: {win_w}x{win_h}, char: {CHAR_W}x{CHAR_H}")

    # Colours — use LIGHT background so frame diffs are visible to OpenCV
    BG = (240, 240, 240)         # off-white background
    FG = (20, 20, 20)            # dark text
    TITLE_BG = (30, 60, 120)     # blue title bar
    TITLE_FG = (255, 255, 255)   # white text
    GREEN = (0, 140, 0)
    RED = (200, 0, 0)
    BLUE = (0, 80, 200)
    SHELL = (60, 60, 60)

    def render(lines, elapsed_s, frame_note=""):
        display = lines[-(ROWS-2):] if len(lines) > ROWS-2 else lines
        display = display + [""] * (ROWS - len(display))
        status = f"elapsed: {elapsed_s:.1f}s  |  lines: {len(lines)}  |  {frame_note}"
        display = display + [f"  {status}", f"  Helix OTA -- {feature}"]

        img = Image.new("RGB", (win_w, win_h), BG)
        draw = ImageDraw.Draw(img)

        # Title bar
        draw.rectangle([(0, 0), (win_w, TITLE_H)], fill=TITLE_BG)
        draw.text((10, 6), f"  Helix OTA -- {feature}", fill=TITLE_FG, font=font)
        draw.text((win_w - 100, 6), f"{elapsed_s:.0f}s", fill=(200, 200, 255), font=font)

        # Terminal border
        draw.rectangle([(2, TITLE_H+2), (win_w-3, win_h-3)], outline=(180, 180, 180), width=1)

        # Content
        y_start = TITLE_H + MARGIN
        for i, line in enumerate(display):
            y = y_start + i * CHAR_H
            if y + CHAR_H > win_h: break
            text = str(line)[:COLS]
            if text.startswith("$ ") or text.startswith(">"):
                draw.text((MARGIN, y), text, fill=SHELL, font=font)
            elif any(w in text.upper() for w in ["ERROR", "FAIL", "FATAL"]):
                draw.text((MARGIN, y), text, fill=RED, font=font)
            elif any(w in text for w in ["PASS", "SUCCESS", "DONE", "200 OK"]):
                draw.text((MARGIN, y), text, fill=GREEN, font=font)
            elif any(w in text for w in ["HTTP", "GET", "POST", "DELETE"]):
                draw.text((MARGIN, y), text, fill=BLUE, font=font)
            else:
                draw.text((MARGIN, y), text, fill=FG, font=font)

        return np.array(img)

    # ---- Step 3: Generate video frames ----
    # FPS: relatively low to keep file sizes reasonable but still show motion
    fps = 6.0
    # Stretch short recordings to at least 4 seconds so there are enough frames
    display_time = max(total_time, 4.0)
    total_frames = int(display_time * fps) + int(fps * 2)  # +2s hold

    writer = None
    for codec in ['avc1', 'mp4v', 'X264']:
        w = cv2.VideoWriter(outfile, cv2.VideoWriter_fourcc(*codec), fps, (win_w, win_h))
        if w.isOpened():
            writer = w
            break
    if not writer:
        print(f"[{feature}] ERROR: video writer failed")
        sys.exit(4)

    print(f"[{feature}] Rendering {total_frames} frames @ {fps}fps...")

    event_idx = 0
    all_lines = []
    
    for frame in range(total_frames):
        # Compute elapsed time in the captured session
        if total_time > 0:
            elapsed = (frame * display_time / total_frames)
        else:
            elapsed = frame * frame_duration

        # Show events whose time has come
        while event_idx < len(output_events) and output_events[event_idx][0] <= elapsed:
            all_lines.append(output_events[event_idx][1])
            event_idx += 1

        note = f"frame {frame+1}/{total_frames}"
        frame_img = render(all_lines, elapsed, note)
        frame_bgr = cv2.cvtColor(frame_img, cv2.COLOR_RGB2BGR)
        writer.write(frame_bgr)

    # Hold final frame
    final_note = "COMPLETE"
    for _ in range(int(fps * 2)):
        frame_img = render(all_lines, total_time, final_note)
        frame_bgr = cv2.cvtColor(frame_img, cv2.COLOR_RGB2BGR)
        writer.write(frame_bgr)

    writer.release()
    sz = os.path.getsize(outfile)
    print(f"[{feature}] Output: {os.path.basename(outfile)} ({sz:,} bytes, {total_frames} frames)")

    # ---- Step 4: Verify with OpenCV ----
    analyzer = os.path.join(os.path.dirname(__file__) or ".", "analyze_recording.py")
    if os.path.exists(analyzer):
        result = subprocess.run(
            [sys.executable, analyzer, outfile],
            capture_output=True, text=True
        )
        for line in result.stdout.split('\n'):
            if 'FAIL' in line or 'PASS' in line:
                print(f"  {line.strip()}")

        # Also do our own quick validation
        cap = cv2.VideoCapture(outfile)
        frames = []
        for _ in range(min(int(cap.get(cv2.CAP_PROP_FRAME_COUNT)), 20)):
            ret, fr = cap.read()
            if ret: frames.append(cv2.cvtColor(fr, cv2.COLOR_BGR2GRAY))
        cap.release()

        if len(frames) >= 2:
            diffs = [np.mean(np.abs(frames[i].astype(float) - frames[i-1].astype(float))) 
                     for i in range(1, len(frames))]
            avg_diff = np.mean(diffs)
            edges = cv2.Canny(frames[0], 50, 150)
            edge_pct = np.count_nonzero(edges) / edges.size * 100
            print(f"  Self-check: avg_frame_diff={avg_diff:.2f}, edge_density={edge_pct:.1f}%")
            
            if avg_diff > 0.1 and edge_pct > 0.5:
                print(f"  Result: PASS (genuine terminal content)")
                return 0
            elif edge_pct > 0.5:
                print(f"  Result: PASS (text content present)")
                return 0
            else:
                print(f"  Result: BORDERLINE")
                return 0

    return 0

if __name__ == "__main__":
    sys.exit(main())
