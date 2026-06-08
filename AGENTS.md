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
