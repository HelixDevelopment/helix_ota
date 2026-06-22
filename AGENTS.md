# Helix OTA — AGENTS.md

> Base agent rules live at `constitution/AGENTS.md` and the
> `constitution/Constitution.md` it references. **READ THOSE FIRST.**
> The base file is authoritative for any topic not covered here. This
> file extends them with project-specific rules; it never weakens them.

## Critical base rules restated (for agents that don't follow @imports)

- **No bluffing.** Every PASS carries positive evidence. Constitution §11.4.
- **Mutation-paired gates.** Every new gate has a paired mutation
  proving it catches regressions. Constitution §1.1.
- **No guessing language.** `likely`, `probably`, `maybe`, `seems`,
  `appears` etc. are forbidden when reporting causes. Constitution §11.4.6.
- **Credentials never tracked.** `.env` patterns git-ignored;
  runtime-load only; per-service file separation. Constitution §11.4.10.
- **Never force-push.** Force-push requires explicit per-session
  authorization. Constitution §11.4.113.
- **Hardlinked backup before any destructive op.** Constitution §9.
- **CONTINUATION document kept in sync.** Constitution §12.10. The live
  handoff for this project is `docs/research/main_specs/CONTINUATION.md`
  (and `.remember/remember.md` when present).
- **60% RAM cap.** Heavy work wrapped in bounded execution scope.
  Constitution §12.6.
- **Per-feature Status + video confirmation.** Constitution §11.4.153.
  `docs/features/Status.md` + `Status_Summary.md` enumerate every feature
  with per-row video-recording confirmation; HTML+PDF+DOCX exports in sync.
- **Window-scoped capture + fresh-corpus rotation.** Constitution §11.4.154.
  Recordings capture only the app window/surface, not whole desktop; a new
  run removes its own prior in-scope recordings first.
- **Project-prefixed recording filenames.** Constitution §11.4.155. Every
  recording filename starts with `<PREFIX>---` (resolved from
  `HELIX_RELEASE_PREFIX` env var or project-root dir name), not a single
  hyphen.
- **All CI/CD disabled.** Constitution §11.4.156. No GitHub Actions or
  GitLab pipelines active; enforcement is local-only via pre-build gates
  and git hooks.
- **GEMINI.md lockstep.** Constitution §11.4.157. GEMINI.md kept in sync
  with CLAUDE.md/AGENTS.md/QWEN.md at all times.
- **Intensive recording + read-the-screen content verification.**
  Constitution §11.4.158. Every feature/flow/edge-case recorded; every
  recording's on-screen content machine-read and verified as a genuine
  working result.
- **Window-specific MP4 + vision validation.** §11.4.159. Every recording must be window-specific MP4 with expected-content specification BEFORE recording, vision validation AFTER recording, terminal window cleanup, and content-verification workflow.
- **Vision-verified recording + HelixQA bridge.** Constitution §11.4.160.
  Recordings processed through a vision/OCR bridge confirming expected
  results before acceptance.
- **Rootless containers.** Constitution §11.4.161. The `containers/` infra
  runs Podman rootless via the `vasic-digital/containers` submodule; no
  Docker-rootful / sudo / root escalation.
- **OpenDesign UI system.** Constitution §11.4.162. Any user-facing UI uses
  OpenDesign design tokens (light+dark, visual-regression covered); latent
  until a UI surface ships.
- **Media validation pipeline.** Constitution §11.4.163. Every recorded
  artifact validated (OCR / transcription / parse vs SPECIFY-phase patterns,
  self-validated analyzer) before acceptance.
- **Constitution auto-propagation hook.** Constitution §11.4.164. After
  every constitution pull run `constitution/scripts/post_update_hook.sh`
  (inherited by reference) to register/install changed skills/MCP/hooks.
- **Independent verification agent.** Constitution §11.4.165. Every code
  change OR media artifact passes an independent verifier iterating to a
  zero-finding GO.
- **Semgrep static analysis.** Constitution §11.4.166. Semgrep installed +
  on PATH; `semgrep scan --config auto --error` runs before every commit /
  push and blocks on findings; scripts inherited by reference from
  `constitution/scripts/semgrep/*`.

## Project-specific agent rules

### Allowed CLI tools

- `go`, `gofmt`, `go vet` (server + Go bricks)
- `gradle` (Android bricks), `git`, `gh`, `glab`
- `redocly`, `kubeconform`, `mmdc`, `pandoc` (spec/doc validation)

### Project-specific workflow

- Commit + push to ALL four upstreams regularly; merge to `main` when a
  milestone is done.
- New reusable submodule repos are created PUBLIC on GitHub + GitLab
  under the HelixDevelopment / vasic-digital orgs (pre-authorized).
- `docs/research/main_specs/additions/` files are authoritative input —
  always analyze + fold them in.

### Inheritance verification

Before a build/merge the inheritance gate MUST pass:
`bash tests/test_constitution_inheritance.sh` (gate + §1.1 paired
mutation via `constitution/meta_test_inheritance.sh`).
