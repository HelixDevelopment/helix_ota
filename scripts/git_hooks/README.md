# Git Hooks

Automated quality gates that run during the git lifecycle.

## Installed Hooks

| Hook | Trigger | Action | Blocking? |
|------|---------|--------|-----------|
| `pre-commit` | Before commit | HelixTrack push sync of workable items (§11.4.148) | No — skips gracefully |
| `post-commit` | After commit | DB auto-sync: scans commit message for OTA-IDs, updates workable_items.db status (§11.4.93) | No — errors logged only |
| `post-merge` | After merge | CodeGraph knowledge-graph auto-rebuild for Go/TS/Kotlin changes | No — errors logged only |

## Installation

```bash
# Install all hooks
for hook in scripts/git_hooks/*; do
    [ -f "$hook" ] || continue
    name=$(basename "$hook")
    cp "$hook" ".git/hooks/$name"
    chmod +x ".git/hooks/$name"
done
```

Or install individually:

```bash
cp scripts/git_hooks/post-commit .git/hooks/post-commit
chmod +x .git/hooks/post-commit
```

## post-commit Detail

Extracts `OTA-NNN` patterns from the commit message body and runs
`scripts/sync_workable_items.sh --auto` to update the workable-items
SQLite database.  Transition mapping:

| Commit type | Bug | Feature | Task |
|-------------|-----|---------|------|
| `fix:` / `fix(...)` | Fixed | Implemented | Completed |
| `feat:` / `feat(...)` | Fixed | Implemented | Completed |
| `chore: close` / `close` | Fixed | Implemented | Completed |
| `revert` | Reopened | Reopened | Reopened |

The hook is non-blocking — a failure in the sync script is logged but
never prevents the commit from completing.
