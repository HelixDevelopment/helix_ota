# Documentation Standards

| Field | Value |
| --- | --- |
| Revision | 1 |
| Created | 2026-06-07 |
| Last modified | 2026-06-07 |
| Status | active |
| Status summary | Initial documentation standards for the Helix OTA corpus. Defines mandatory metadata, ToC, file naming, multi-format export targets, diagram conventions, cross-reference style, and the anti-bluff/UNVERIFIED convention. |
| Issues | None known at Revision 1. |
| Fixed | N/A (initial revision). |
| Continuation | Validate export tooling (renderable vs source-only matrix) against an actual build; reconcile §11.4.x clause numbers against the authoritative HelixConstitution once available (UNVERIFIED). |

## Table of contents

1. [Purpose and scope](#1-purpose-and-scope)
2. [Mandatory metadata table](#2-mandatory-metadata-table)
3. [Table of contents requirement](#3-table-of-contents-requirement)
4. [File naming](#4-file-naming)
5. [Multi-format export targets](#5-multi-format-export-targets)
6. [Diagram conventions](#6-diagram-conventions)
7. [Cross-reference style](#7-cross-reference-style)
8. [Anti-bluff and UNVERIFIED convention](#8-anti-bluff-and-unverified-convention)
9. [Submodule catalogue (canonical names)](#9-submodule-catalogue-canonical-names)
10. [Authoring checklist](#10-authoring-checklist)

> The table-of-contents requirement is mandated by HelixConstitution §11.4.61. Every corpus document MUST carry a ToC immediately after its metadata table.

---

## 1. Purpose and scope

This document defines the documentation standards for the Helix OTA documentation corpus
(rooted at `docs/research/main_specs/`). It is normative: every document authored or revised
in the corpus MUST conform to the rules below. Where a rule references a HelixConstitution
clause (e.g. §11.4.29, §11.4.61, §7.1, §11.4.6, §11.4.123), that clause is the controlling
authority and this document is its corpus-local restatement.

The exact wording and numbering of the cited HelixConstitution clauses has not been
cross-checked against the authoritative constitution text in this revision and is marked
UNVERIFIED (see §8).

## 2. Mandatory metadata table

Every document MUST begin with a metadata table as its first content element, before any
prose, the ToC, or any heading other than the document title. The table MUST contain, at
minimum, the following rows in this order:

| Field | Meaning |
| --- | --- |
| Revision | Monotonically increasing integer, starting at 1. Bump on every substantive change. |
| Created | ISO-8601 date (`YYYY-MM-DD`) the document was first created. Never changes. |
| Last modified | ISO-8601 date (`YYYY-MM-DD`) of the most recent substantive change. |
| Status | One of: `draft`, `active`, `deprecated`, `superseded`. |
| Status summary | One- to three-sentence description of the document's current state and intent. |
| Issues | Known problems, gaps, or open questions. Use `None known at Revision N.` when empty. |
| Fixed | What was corrected since the previous revision. Use `N/A (initial revision).` at Revision 1. |
| Continuation | Concrete follow-up work required. Forward-looking TODOs live here, not in prose. |

Rules:

- The metadata table format is fixed (two columns: `Field`, `Value`). Do not reorder or rename the mandatory rows.
- Additional rows MAY be appended after `Continuation` (for example `Owner`, `Reviewers`), but the mandatory rows MUST all be present.
- `Created` is immutable. `Last modified` and `Revision` MUST be updated together on every change.

## 3. Table of contents requirement

Per HelixConstitution §11.4.61, a Table of contents is mandatory.

- The ToC MUST appear immediately after the metadata table and before the first body section.
- ToC entries MUST be ordered, numbered, and link to in-document anchors.
- The ToC MUST be kept in sync with the document's headings; a heading without a ToC entry, or a ToC entry without a heading, is a conformance defect.
- Documents short enough to fit on a single screen still require a ToC; there is no length exemption.

## 4. File naming

Per HelixConstitution §11.4.29, file names MUST use `lowercase_snake_case`.

- Allowed characters: lowercase ASCII letters `a-z`, digits `0-9`, and underscore `_`.
- Words are separated by single underscores. No spaces, hyphens, camelCase, or uppercase letters.
- The extension is `.md` for Markdown sources.
- Examples (conformant): `documentation_standards.md`, `ota_update_flow.md`, `event_bus_contract.md`.
- Examples (non-conformant): `Documentation-Standards.md`, `otaUpdateFlow.md`, `event bus contract.md`.
- Numeric ordering prefixes for directories (for example `00-master/`) are an established corpus convention and are permitted at the directory level; file basenames themselves remain `lowercase_snake_case`.

## 5. Multi-format export targets

The corpus targets the following export and diagram formats. Each is classified as
**renderable** (produced by an export/render step today) or **source-only** (authored
artifact that is the source of truth but not auto-rendered in this revision).

| Target | Category | Status | Notes |
| --- | --- | --- | --- |
| PDF | Document export | Renderable (UNVERIFIED) | Print/archival output derived from Markdown. Pipeline not verified in this revision. |
| HTML | Document export | Renderable (UNVERIFIED) | Web/browsable output derived from Markdown. Pipeline not verified in this revision. |
| DOCX | Document export | Renderable (UNVERIFIED) | Word-compatible output for external distribution. Pipeline not verified in this revision. |
| Mermaid (`.mmd`) | Diagram source | Source-only (source of truth) | Authored, version-controlled diagram source. See §6. |
| draw.io (`.drawio`) | Diagram source | Source-only | Editable diagram source where Mermaid is insufficient. |
| SVG (`.svg`) | Diagram render | Renderable | Vector render, typically generated from Mermaid or draw.io. |
| UML (`.uml`/PlantUML) | Diagram source | Source-only | Authored UML source. |
| PNG (`.png`) | Diagram render | Renderable | Raster render for embedding where vector is not supported. |

Notes:

- The renderability of PDF/HTML/DOCX depends on the corpus build tooling. Until that tooling is exercised and confirmed, these three are marked **Renderable (UNVERIFIED)** per §8.
- Diagram **sources** (Mermaid, draw.io, UML) are never deleted in favor of their rendered outputs; the rendered SVG/PNG are derived artifacts.

## 6. Diagram conventions

- **Mermaid is the source of truth** for diagrams. Author diagrams in Mermaid wherever the
  diagram type is expressible in Mermaid (flowcharts, sequence, class, state, ER, etc.).
- SVG and PNG outputs are **generated** from the Mermaid source; do not hand-edit the
  rendered SVG/PNG. If a render is wrong, fix the Mermaid source and re-render.
- Use draw.io or UML/PlantUML sources only when Mermaid cannot express the required diagram;
  in that case the draw.io/UML file is the source of truth for that specific diagram and the
  same "edit source, not render" rule applies.
- Embed diagrams in Markdown either as fenced ```mermaid``` blocks (preferred, so the source
  travels with the prose) or by referencing the generated SVG/PNG with a path to the
  committed render.
- Diagram source files follow the §4 naming rules.

## 7. Cross-reference style

- **Intra-document** references use Markdown anchor links to numbered sections,
  e.g. `[see §5](#5-multi-format-export-targets)`.
- **Inter-document** references within the corpus use relative paths from the referencing
  file, e.g. `[OTA update flow](../additions/ota_update_flow.md)`, optionally with a section
  anchor.
- **Clause references** to the HelixConstitution use the `§` notation with the dotted clause
  number, e.g. `§11.4.29`, `§11.4.61`, `§7.1`. Always cite the clause when restating a
  normative rule it controls.
- Do not use bare URLs in prose; wrap them in descriptive Markdown link text.
- A reference whose target has not been confirmed to exist MUST be marked UNVERIFIED (§8)
  rather than presented as a confirmed link.

## 8. Anti-bluff and UNVERIFIED convention

Per HelixConstitution §7.1, §11.4.6, and §11.4.123, the corpus enforces an anti-bluff rule:

- **No fabricated facts.** Do not state numbers, behaviors, file paths, API signatures, or
  capabilities that have not been confirmed from a real source.
- **No fabricated citations.** Do not invent references, clause numbers, document titles, or
  external sources. If a citation cannot be confirmed, mark it UNVERIFIED.
- **Mark the unconfirmed.** Any claim, figure, citation, or cross-reference that has not been
  verified MUST be tagged inline with the literal token `UNVERIFIED`, e.g.
  "the export pipeline emits DOCX (UNVERIFIED)".
- **Prefer omission or marking over guessing.** When in doubt, either leave the item out or
  mark it UNVERIFIED; never present a guess as fact.
- Open verification work belongs in the `Continuation` row of the metadata table (§2).

In this revision, the following are explicitly UNVERIFIED: the exact text/numbering of all
cited HelixConstitution clauses, and the renderable status of the PDF/HTML/DOCX export
pipelines (§5).

## 9. Submodule catalogue (canonical names)

Documents MUST refer to submodules using only the verified canonical names below. Do not
invent, abbreviate, or rename submodules.

Core / backend submodules:

- `auth`
- `security`
- `database`
- `Storage`
- `observability`
- `eventbus`
- `ratelimiter`
- `middleware`
- `http3`
- `mdns`
- `recovery`
- `Herald`
- `config`
- `discovery`
- `cache`
- `docs_chain` / `Document` / `Formatters`
- `containers`

KMP (Kotlin Multiplatform) submodules:

- `Auth-KMP`
- `Security-KMP`
- `Storage-KMP`
- `Config-KMP`

Any submodule not on this list MUST NOT be referenced as if it exists; if one is needed,
add it here first (and mark it UNVERIFIED until confirmed).

## 10. Authoring checklist

Before committing a corpus document, confirm:

- [ ] Metadata table is the first element and contains all mandatory rows (§2).
- [ ] `Revision` and `Last modified` were bumped together for this change (§2).
- [ ] Table of contents is present, numbered, and in sync with headings (§3, §11.4.61).
- [ ] File name is `lowercase_snake_case.md` (§4, §11.4.29).
- [ ] Diagrams are authored in Mermaid (or justified draw.io/UML) as source of truth; renders are generated, not hand-edited (§6).
- [ ] Cross-references use the correct intra/inter-document and `§` clause styles (§7).
- [ ] Every unconfirmed fact, figure, or citation is tagged `UNVERIFIED` (§8).
- [ ] Only canonical submodule names from §9 are used.
