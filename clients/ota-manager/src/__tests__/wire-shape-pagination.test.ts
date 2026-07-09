import { describe, it, expect } from "vitest";
import type {
  DeviceList,
  ReleaseList,
  GroupList,
  TelemetryHistory,
  DeploymentList,
  RollbackList,
} from "@/lib/api-client";

// §11.4.108/§11.4.115 regression guard — client/server wire-shape drift on
// cursor-paginated list responses (docs/qa/20260710-client-wire-shape-
// reconciliation/EVIDENCE.md).
//
// The client previously declared these list interfaces as `{ items, total,
// cursor? }`. The REAL Go server never sends `total` or a response-side
// `cursor` for any of these endpoints — it sends `{ items, next_cursor }`
// (cursor-paginated lists: DeviceList, ReleaseList, GroupList,
// TelemetryHistory, DeploymentList) or `{ items }` only (non-paginated:
// RollbackList). `total`/`cursor` were therefore ALWAYS `undefined` at
// runtime and `next_cursor` was untyped. Verified against:
//   - DeviceList      -> server/internal/api/wire.go:97-100
//   - ReleaseList     -> server/internal/api/wire.go:159-162
//   - GroupList       -> server/internal/api/handlers_group.go:62-67
//   - TelemetryHistory-> server/internal/api/handlers_telemetry.go:30-38
//   - DeploymentList  -> server/internal/api/wire.go:184-190 (carries
//                        next_cursor too — reserved, always null in the MVP,
//                        but the JSON key is never omitted; NOT items-only)
//   - RollbackList    -> server/internal/api/handlers_recall.go:36-39
//                        (items only, genuinely no cursor field)
//
// RED (pre-fix, captured 2026-07-10): with the OLD client shape
// (`{ items: T[]; total: number; cursor?: string }` for the five
// cursor-paginated lists), assigning a real-server-shaped fixture
// (`{ items: [], next_cursor: null }`) to a `DeviceList`-typed const failed
// `tsc --noEmit` with:
//   error TS2322: Type '{ items: never[]; next_cursor: null; }' is not
//   assignable to type 'DeviceList'.
//     Object literal may only specify known properties, and 'next_cursor'
//     does not exist in type 'DeviceList'.
// (five equivalent errors, one per cursor-paginated interface — full
// transcript in EVIDENCE.md). This test function only TYPE-CHECKS (never
// executed at runtime — `tsc --noEmit` is the oracle) so it stays a
// permanent, zero-cost compile-time guard against the drift recurring.
//
// GREEN (post-fix): the corrected interfaces declare `next_cursor: string |
// null` (cursor-paginated) / no cursor field at all (RollbackList), so the
// real-shaped fixtures below type-check, and the OLD fabricated
// `{ total, cursor }` shape is now a compile error if reintroduced (asserted
// via the `// @ts-expect-error` blocks).
function assertRealServerShapesTypeCheck(): void {
  const deviceList: DeviceList = { items: [], next_cursor: null };
  const releaseList: ReleaseList = { items: [], next_cursor: "cursor-abc" };
  const groupList: GroupList = { items: [], next_cursor: null };
  const telemetryHistory: TelemetryHistory = { items: [], next_cursor: null };
  const deploymentList: DeploymentList = { items: [], next_cursor: null };
  const rollbackList: RollbackList = { items: [] };

  // Silence noUnusedLocals — these consts exist purely so the assignment
  // above is what tsc type-checks (the actual assertion).
  void deviceList;
  void releaseList;
  void groupList;
  void telemetryHistory;
  void deploymentList;
  void rollbackList;

  // @ts-expect-error — `total` is NOT part of the real DeviceList wire shape;
  // if this stops erroring, the fabricated field has been reintroduced.
  const deviceListWithFabricatedTotal: DeviceList = { items: [], next_cursor: null, total: 5 };
  void deviceListWithFabricatedTotal;

  // @ts-expect-error — RollbackList genuinely has no cursor field at all; a
  // `next_cursor` here would mean the server added pagination this client
  // must be told about explicitly (never silently re-typed).
  const rollbackListWithFabricatedCursor: RollbackList = { items: [], next_cursor: null };
  void rollbackListWithFabricatedCursor;
}
void assertRealServerShapesTypeCheck;

describe("list/pagination wire-shape reconciliation (client interfaces match the real Go server)", () => {
  it("is a compile-time-only guard — tsc --noEmit is the oracle (see file header for RED/GREEN evidence)", () => {
    // This test body is intentionally a no-op at runtime: the assertion this
    // file makes is entirely in the type-level function above, checked by
    // `npx tsc --noEmit`. A single trivial runtime assertion keeps vitest's
    // "no tests in file" failure mode from masking a deleted guard.
    expect(true).toBe(true);
  });
});
