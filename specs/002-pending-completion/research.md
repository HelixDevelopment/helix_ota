# Research: Pending Work Completion

## Resolved Decisions

### DB Sync Strategy
- **Decision**: Update the SQLite workable-items DB directly with SQL UPDATE statements. No migration needed — this is a data update.
- **Rationale**: The DB schema is stable. All closures have code evidence committed. Status transitions are deterministic (Queued→Completed/Fixed/Implemented based on item type).
- **Alternatives considered**: Rebuild DB from Issues/Fixed docs (rejected — docs may be stale); manual per-item edits (rejected — 16 items need bulk update).

### Merge Strategy
- **Decision**: Merge main into feature/production-readiness first (to bring in any main changes), then merge feature→main. Both as `git merge` (never rebase per §11.4.113).
- **Rationale**: Main has only had doc/tooling commits since the feature branch was created. The merge should be clean or have trivial conflicts. Per §11.4.188 the feature branch must regularly merge main into itself during the work.
- **Alternatives considered**: Cherry-pick individual commits (rejected — loses history); rebase (forbidden per §11.4.113).

### Hardware-Gated Items
- **Decision**: Document unblock conditions, required hardware specs, and estimated effort for each hardware-gated item. Do NOT write code that cannot be tested.
- **Rationale**: Per §11.4.6 no-guessing mandate — writing device code without a device to test on produces untestable claims. Items remain Queued with Operator-blocked sub-status.
