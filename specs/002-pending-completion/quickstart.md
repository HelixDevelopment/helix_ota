# Quickstart: Pending Work Completion

## Prerequisites
- Working Go toolchain
- Git with all remotes configured
- SQLite3 CLI available on PATH

## Validation Scenarios

### 1. Sync Workable-Items DB
```bash
# Before: count Queued items
sqlite3 docs/workable_items.db "SELECT COUNT(*) FROM items WHERE status = 'Queued';"
# Expected before: 22
# Run the DB sync (to be implemented)
# After: should be ≤6 (only hardware-gated remain)
sqlite3 docs/workable_items.db "SELECT COUNT(*) FROM items WHERE status = 'Queued';"
```

### 2. Fix Build Error
```bash
cd server && go build ./...
# Expected: zero errors (currently: unused import in wire.go)
```

### 3. Merge to Main
```bash
# Merge main into feature branch first
git checkout feature/production-readiness
git fetch origin main && git merge origin/main
# Then merge feature into main
git checkout main && git merge feature/production-readiness
# Push to all upstreams
git push github main && git push gitlab main && git push gitflic main && git push gitverse main
```

### 4. Update Carrier Files
```bash
# Verify all five carrier files contain 1.0.0 reference
grep -l "helix_ota-1.0.0" CLAUDE.md AGENTS.md QWEN.md GEMINI.md GEMINI.md
```

### 5. Full Verification
```bash
# Constitution sweep
bash tests/pre_build_verification.sh
# Constitution inheritance
bash tests/test_constitution_inheritance.sh
# Core test suite
cd server && go test ./... -count=1
```
