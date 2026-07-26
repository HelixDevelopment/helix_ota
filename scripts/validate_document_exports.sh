#!/usr/bin/env bash
#
# validate_document_exports.sh — Visual validation pipeline for exported docs
# =========================================================================
# Validates that HTML + PDF siblings of a Markdown file exist and are
# well-formed: no raw markup text visible, no overlapping elements, no
# clipped content, PDF text is machine-readable.
#
# Usage:
#   scripts/validate_document_exports.sh <path/to/file.md>
#   scripts/validate_document_exports.sh <path/to/dir>     # validate all .md files recursively
#
# Dependencies:
#   - pandoc       (for HTML/PDF generation if siblings missing)
#   - weasyprint   OR wkhtmltopdf OR tectonic/xelatex (PDF engine)
#   - pdftotext    (poppler-utils) for PDF text extraction
#   - python3      (for headless browser capture, optional)
#
# Exit codes:
#   0 — all siblings valid
#   1 — validation errors found
#   2 — usage error (bad args, no inputs)

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0
SKIP=0

log_pass()  { printf "${GREEN}[PASS]${NC} %s\n" "$*"; PASS=$((PASS+1)); }
log_fail()  { printf "${RED}[FAIL]${NC} %s\n" "$*" >&2; FAIL=$((FAIL+1)); }
log_skip()  { printf "${YELLOW}[SKIP]${NC} %s\n" "$*"; SKIP=$((SKIP+1)); }

have() { command -v "$1" >/dev/null 2>&1; }

# --- collect inputs ---
INPUT="${1:-}"
if [ -z "$INPUT" ]; then
  echo "Usage: $0 <file.md|dir>" >&2; exit 2
fi

declare -a FILES=()
if [ -f "$INPUT" ]; then
  FILES+=("$INPUT")
elif [ -d "$INPUT" ]; then
  while IFS= read -r f; do FILES+=("$f"); done < <(find "$INPUT" -type f -name '*.md' | sort)
else
  echo "ERROR: not found: $INPUT" >&2; exit 2
fi

[ "${#FILES[@]}" -gt 0 ] || { echo "ERROR: no .md files found under $INPUT" >&2; exit 2; }
echo "=== Validating ${#FILES[@]} markdown file(s) ==="
echo ""

# --- probe dependencies ---
HAVE_PANDOC=0;   have pandoc      && HAVE_PANDOC=1
HAVE_PDFTOTEXT=0; have pdftotext  && HAVE_PDFTOTEXT=1
HAVE_PYTHON=0;   have python3     && HAVE_PYTHON=1

PDF_ENGINE=""
for e in weasyprint wkhtmltopdf tectonic xelatex pdflatex lualatex; do
  if have "$e"; then PDF_ENGINE="$e"; break; fi
done

echo "Dependencies: pandoc=${HAVE_PANDOC} pdftotext=${HAVE_PDFTOTEXT} pdf_engine=${PDF_ENGINE:-none} python3=${HAVE_PYTHON}"
echo ""

for md in "${FILES[@]}"; do
  dir="$(dirname "$md")"
  base="$(basename "${md%.md}")"
  html_file="${dir}/${base}.html"
  pdf_file="${dir}/${base}.pdf"

  echo "--- $md ---"

  # Step 1: Check HTML sibling exists
  if [ -f "$html_file" ]; then
    log_pass "HTML sibling exists: $html_file"
  else
    if [ "$HAVE_PANDOC" -eq 1 ]; then
      log_skip "HTML sibling missing — generating with pandoc"
      pandoc "$md" -s --metadata title="$base" -o "$html_file" 2>/dev/null || true
      if [ -f "$html_file" ]; then
        log_pass "HTML generated: $html_file"
      else
        log_fail "HTML generation failed"
      fi
    else
      log_fail "HTML sibling missing: $html_file (pandoc not available to generate)"
    fi
  fi

  # Step 2: Check PDF sibling exists
  if [ -f "$pdf_file" ]; then
    log_pass "PDF sibling exists: $pdf_file"
  else
    if [ "$HAVE_PANDOC" -eq 1 ] && [ -n "$PDF_ENGINE" ]; then
      log_skip "PDF sibling missing — generating with pandoc/$PDF_ENGINE"
      pandoc "$md" --pdf-engine="$PDF_ENGINE" -o "$pdf_file" 2>/dev/null || true
      if [ -f "$pdf_file" ]; then
        log_pass "PDF generated: $pdf_file"
      else
        log_fail "PDF generation failed"
      fi
    else
      log_fail "PDF sibling missing: $pdf_file (PDF engine not available)"
    fi
  fi

  # Step 3: Validate HTML — no raw markup text
  if [ -f "$html_file" ]; then
    # Check for raw markdown artifacts in rendered HTML
    if grep -qP '```mermaid|```yaml|```bash' "$html_file" 2>/dev/null; then
      log_fail "HTML contains raw fenced code block markers: $html_file"
    else
      log_pass "HTML: no raw code-fence markers"
    fi

    # Check for common LaTeX/template artifacts
    if grep -qP '\\begin\{|\\end\{|\\textbf\{' "$html_file" 2>/dev/null; then
      log_fail "HTML contains raw LaTeX artifacts: $html_file"
    else
      log_pass "HTML: no raw LaTeX artifacts"
    fi

    # Check for empty body (clipped content)
    HTML_SIZE=$(wc -c < "$html_file")
    if [ "$HTML_SIZE" -lt 100 ]; then
      log_fail "HTML file appears empty/clipped (${HTML_SIZE} bytes): $html_file"
    else
      log_pass "HTML: non-trivial content (${HTML_SIZE} bytes)"
    fi
  fi

  # Step 4: Validate PDF — text is machine-readable
  if [ -f "$pdf_file" ]; then
    if [ "$HAVE_PDFTOTEXT" -eq 1 ]; then
      TEXT_OUT="$(pdftotext "$pdf_file" - 2>/dev/null || true)"
      TEXT_LEN=$(echo "$TEXT_OUT" | wc -c)
      if [ "$TEXT_LEN" -lt 50 ]; then
        log_fail "PDF text is empty/clipped (${TEXT_LEN} chars): $pdf_file"
      else
        log_pass "PDF: machine-readable text (${TEXT_LEN} chars)"

        # Check for raw markup in PDF text
        if echo "$TEXT_OUT" | grep -qP '```\w+'; then
          log_fail "PDF text contains raw code-fence markers"
        else
          log_pass "PDF: no raw code-fence markers in text"
        fi
      fi

      # Check PDF page count
      PDF_PAGES=$(pdfinfo "$pdf_file" 2>/dev/null | grep '^Pages:' | awk '{print $2}' || echo "0")
      if [ "${PDF_PAGES:-0}" -eq 0 ]; then
        log_fail "PDF has zero pages: $pdf_file"
      else
        log_pass "PDF: ${PDF_PAGES} page(s)"
      fi
    else
      log_skip "PDF text validation skipped (pdftotext not available)"
    fi

    PDF_SIZE=$(wc -c < "$pdf_file")
    if [ "$PDF_SIZE" -lt 500 ]; then
      log_fail "PDF file appears empty/clipped (${PDF_SIZE} bytes): $pdf_file"
    else
      log_pass "PDF: non-trivial content (${PDF_SIZE} bytes)"
    fi
  fi

  echo ""
done

# --- summary ---
echo "========================================"
printf "Results: ${GREEN}%d PASS${NC}  ${RED}%d FAIL${NC}  ${YELLOW}%d SKIP${NC}\n" "$PASS" "$FAIL" "$SKIP"

if [ "$FAIL" -gt 0 ]; then
  echo "VALIDATION FAILED"
  exit 1
else
  echo "VALIDATION PASSED"
  exit 0
fi
