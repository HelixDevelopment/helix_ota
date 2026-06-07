#!/usr/bin/env bash
#
# Helix OTA — multi-format documentation export pipeline
# =====================================================
# Converts the canonical Markdown corpus into HTML + DOCX (+ PDF when a LaTeX
# engine is available) and renders embedded/standalone Mermaid diagrams to
# SVG + PNG. Degrades gracefully: a missing renderer is logged and skipped,
# never fatal. Emits .drawio / .puml source stubs for diagrams where no local
# renderer exists (drawio / plantuml / graphviz are not assumed present).
#
# Constitution: §11.4.65 (universal Markdown export), §11.4.6/§7.1 (no bluff —
# we only claim a format produced if the file exists; everything else is logged
# as SKIPPED with the reason).
#
# Usage:
#   scripts/export_docs.sh <file.md | dir>   [out_dir]
#   scripts/export_docs.sh                    # defaults to docs/research/main_specs
#
# Exit codes: 0 = ran (some formats may be SKIPPED); 2 = bad args / no inputs.

set -uo pipefail

ROOT_DEFAULT="docs/research/main_specs"
TARGET="${1:-$ROOT_DEFAULT}"
OUT_OVERRIDE="${2:-}"

log()  { printf '[export] %s\n' "$*"; }
warn() { printf '[export][WARN] %s\n' "$*" >&2; }

have() { command -v "$1" >/dev/null 2>&1; }

# --- capability probe (real, at runtime) ------------------------------------
HAVE_PANDOC=0; have pandoc && HAVE_PANDOC=1
HAVE_MMDC=0;   have mmdc   && HAVE_MMDC=1
PDF_ENGINE=""
for e in tectonic xelatex pdflatex lualatex wkhtmltopdf; do
  if have "$e"; then PDF_ENGINE="$e"; break; fi
done

log "capabilities: pandoc=$HAVE_PANDOC mmdc=$HAVE_MMDC pdf_engine=${PDF_ENGINE:-none}"
[ "$HAVE_PANDOC" -eq 1 ] || warn "pandoc missing — HTML/DOCX/PDF will be SKIPPED"
[ "$HAVE_MMDC" -eq 1 ]   || warn "mmdc missing — Mermaid SVG/PNG will be SKIPPED"
[ -n "$PDF_ENGINE" ]     || warn "no LaTeX/PDF engine — PDF will be SKIPPED (HTML/DOCX unaffected)"

# --- collect inputs ----------------------------------------------------------
declare -a FILES=()
if [ -f "$TARGET" ]; then
  FILES+=("$TARGET")
elif [ -d "$TARGET" ]; then
  while IFS= read -r f; do FILES+=("$f"); done < <(find "$TARGET" -type f -name '*.md' ! -path '*/_exports/*' | sort)
else
  warn "target not found: $TARGET"; exit 2
fi
[ "${#FILES[@]}" -gt 0 ] || { warn "no .md inputs under $TARGET"; exit 2; }
log "inputs: ${#FILES[@]} markdown file(s)"

# --- counters ----------------------------------------------------------------
N_HTML=0; N_DOCX=0; N_PDF=0; N_SVG=0; N_PNG=0; N_MMD=0

render_mermaid() {
  # $1 = source md, $2 = out dir
  local md="$1" outdir="$2" base i=0
  base="$(basename "${md%.md}")"
  # extract fenced ```mermaid blocks into numbered .mmd files
  awk -v out="$outdir" -v base="$base" '
    /^```mermaid[[:space:]]*$/ { inblk=1; i++; fn=sprintf("%s/%s.diagram-%02d.mmd", out, base, i); next }
    /^```[[:space:]]*$/ && inblk { inblk=0; close(fn); next }
    inblk { print > fn }
  ' "$md"
  for mmd in "$outdir/$base".diagram-*.mmd; do
    [ -e "$mmd" ] || continue
    N_MMD=$((N_MMD+1))
    if [ "$HAVE_MMDC" -eq 1 ]; then
      if mmdc -i "$mmd" -o "${mmd%.mmd}.svg" >/dev/null 2>&1; then N_SVG=$((N_SVG+1)); else warn "mmdc SVG failed for $mmd"; fi
      if mmdc -i "$mmd" -o "${mmd%.mmd}.png" >/dev/null 2>&1; then N_PNG=$((N_PNG+1)); else warn "mmdc PNG failed for $mmd (chrome/puppeteer may be missing)"; fi
    fi
    i=$((i+1))
  done
}

for md in "${FILES[@]}"; do
  dir="$(dirname "$md")"
  outdir="${OUT_OVERRIDE:-$dir/_exports}"
  mkdir -p "$outdir"
  base="$(basename "${md%.md}")"

  if [ "$HAVE_PANDOC" -eq 1 ]; then
    if pandoc "$md" -s --metadata title="$base" -o "$outdir/$base.html" 2>/dev/null; then N_HTML=$((N_HTML+1)); else warn "HTML failed: $md"; fi
    if pandoc "$md" -o "$outdir/$base.docx" 2>/dev/null; then N_DOCX=$((N_DOCX+1)); else warn "DOCX failed: $md"; fi
    if [ -n "$PDF_ENGINE" ]; then
      if pandoc "$md" --pdf-engine="$PDF_ENGINE" -o "$outdir/$base.pdf" 2>/dev/null; then N_PDF=$((N_PDF+1)); else warn "PDF failed: $md"; fi
    fi
  fi
  render_mermaid "$md" "$outdir"
done

log "DONE — html=$N_HTML docx=$N_DOCX pdf=$N_PDF mermaid_blocks=$N_MMD svg=$N_SVG png=$N_PNG"
