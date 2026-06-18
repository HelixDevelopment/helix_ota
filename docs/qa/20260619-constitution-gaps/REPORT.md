# §11.4.153–§11.4.158 Constitution Audit — Gap Analysis

**Date:** 2026-06-19
**Auditor:** Automated constitution-audit agent
**Scope:** Feature video recording and Status doc mandates in Helix OTA.

## Executive Summary

6 mandates audited. Significant gaps in 5/6. Status document set is well-maintained
but mechanical-enforcement and propagation layers are missing. CI/CD actively violates §11.4.156.

## Per-Mandate Findings

### §11.4.153 — Per-feature Status + video confirmation
| Check | Result |
|-------|--------|
| Status.md exists with Video Recorded+Analysis columns | PASS |
| Status_Summary.md with two-audience format | PASS |
| HTML+PDF exports | PASS |
| DOCX export | FAIL — no `.docx`, docs_chain only has HTML+PDF |
| docs_chain wired | PASS — `.docs_chain/contexts/features-status.yaml` exists |
| Drift-proof fingerprint | FAIL — no sha256 roster fingerprint |
| Propagation in CLAUDE.md | FAIL — no `11.4.153` anchor |
| Propagation in AGENTS.md | FAIL — no `11.4.153` anchor |
| `CM-COVENANT-114-153-PROPAGATION` gate | FAIL — not in pre_build_verification.sh |

### §11.4.154 — Window-scoped + fresh-corpus rotation
| Check | Result |
|-------|--------|
| Window-scoped recordings | PASS |
| Fresh-corpus rotation enforced | FAIL |
| Propagation in CLAUDE.md | FAIL |
| Propagation in AGENTS.md | FAIL |
| Propagation gate | FAIL |

### §11.4.155 — Project-prefixed filenames
| Check | Result |
|-------|--------|
| Prefix present (`helix_ota-`) | PASS |
| Triple-hyphen separator `---` | FAIL — uses single hyphen |
| `.env.example` documents HELIX_RELEASE_PREFIX | FAIL — no .env.example exists |
| Propagation in CLAUDE.md | FAIL |
| Propagation in AGENTS.md | FAIL |
| Propagation gate | FAIL |

### §11.4.156 — All CI/CD disabled
| Check | Result |
|-------|--------|
| CI workflows disabled | CRITICAL FAIL — 3 active: ci.yml, ota-manager.yml, ota-manager-tauri.yml |
| Disabled copies exist | FAIL — no `.disabled-local-only` copies |
| Pre-push gate | FAIL |
| Propagation in CLAUDE.md | FAIL |
| Propagation in AGENTS.md | FAIL |
| Propagation gate | FAIL |

### §11.4.157 — GEMINI.md lockstep
| Check | Result |
|-------|--------|
| constitution/GEMINI.md exists | PASS |
| constitution/QWEN.md exists | PASS |
| Highest § in constitution mirrors matched | PASS — both at §11.4.158 |
| Project GEMINI.md | N/A — project may not use Gemini |
| Propagation in CLAUDE.md | FAIL |
| Propagation in AGENTS.md | FAIL |
| Propagation gate | FAIL |

### §11.4.158 — Recording + read-the-screen verification
| Check | Result |
|-------|--------|
| Content analysis done | PASS — REPORT.md present, 5 recordings analysed |
| Status.md cites §11.4.158 | PASS |
| Save path = `$HOME/Downloads` | FAIL — uses `/Volumes/T7/Downloads/Recordings/` |
| Recording path override declared | FAIL — no CLAUDE.md declaration |
| Propagation in CLAUDE.md | FAIL |
| Propagation in AGENTS.md | FAIL |
| Propagation gate | FAIL |

## Remediation Plan

### HIGH (release blocker)
1. Disable 3 CI workflows per §11.4.156
2. Add propagation gate checks to pre_build_verification.sh
3. Add §11.4.153-158 literal anchors to AGENTS.md
4. Add §11.4.153-158 project constraints to CLAUDE.md

### MEDIUM (process violation)
5. Create `.env.example` with HELIX_RELEASE_PREFIX documented
6. Wire DOCX export into docs_chain features-status context
7. Declare recording path override in CLAUDE.md
8. Add drift-proof fingerprint to docs_chain context

### LOW (best practice)
9. Rename existing recordings to triple-hyphen convention
10. Document recording procedures

## Files to be Modified
- `.github/workflows/*.yml` → rename to `.disabled-local-only` (3 files)
- `tests/pre_build_verification.sh` — add 6 propagation gates
- `AGENTS.md` — add literal anchors
- `CLAUDE.md` — add project constraints
- `.docs_chain/contexts/features-status.yaml` — add DOCX + fingerprint
- `.env.example` — create with HELIX_RELEASE_PREFIX

constitution/ is untouched (read-only submodule).
