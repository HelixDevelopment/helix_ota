// Adapter shim: maps the REAL server telemetry/deployment responses onto the
// dashboard stat-card view model `{totalDevices, activeDeployments,
// pendingUpdates, failedDevices}` (same "raw react-query hook -> view-model
// adapter" pattern as `useDevices.ts` for the device list).
//
// BUG-1 (§11.4.6/§11.4.115 fix): this file used to be a bare re-export of
// `use-devices.ts`'s raw `useTelemetryOverview` —
//   export { useTelemetryOverview } from './use-devices';
// — with NO mapping. The client `TelemetryOverview` type it was cast to was
// itself a fabrication (camelCase fields matching the dashboard, not the
// wire). The REAL GET /telemetry/overview body
// (server/internal/api/handlers_telemetry.go:56-64, handleTelemetryOverview
// at :168-193) is:
//   { event_counts: Record<string, number>,
//     total: number,
//     failure_rate: number,
//     by_state: Record<string, number> }
// `by_state` is the fleet device count keyed by last-known update state
// (server/internal/store/memory.go DeviceStateCounts / postgres.go
// equivalent). Its possible keys, established as FACT from the source (never
// guessed, §11.4.6):
//   - "idle"                                 device-registration default
//     (server/internal/api/handlers_device.go:62,215) — no update in flight,
//     no known failure.
//   - "unknown"                              defensive bucket the store maps
//     an empty/never-set state to (memory.go:570-572) — should not occur in
//     practice since registration always defaults to "idle", but the real
//     store code path exists, so it is a real possible key.
//   - "download_started" | "installing" | "installed" | "verifying"
//     in-flight update states — handlers_client.go:207 sets
//     `dev.UpdateState = string(ev.Event)` verbatim from the device's last
//     reported TelemetryEvent (submodules/ota-protocol/enums.go), so these
//     four non-terminal event names ARE possible UpdateState / by_state keys.
//   - "success"                               last update completed
//     successfully — terminal, not pending.
//   - "failure"                               last known state is a failed
//     update — terminal.
//
// Mapping (every field either REAL-derived from a real source, or an honest
// documented fetch from the correct endpoint — never fabricated):
//   totalDevices      = sum of every by_state count (the fleet device total;
//                       by_state partitions the WHOLE registered-device set
//                       by construction, one bucket per device).
//   pendingUpdates    = by_state["download_started"] + ["installing"] +
//                       ["installed"] + ["verifying"] — the four non-terminal
//                       states, i.e. devices with an update currently in
//                       flight ("awaiting/undergoing an update", the
//                       dashboard's own stat-card description).
//   failedDevices     = by_state["failure"] — devices whose LAST KNOWN state
//                       is a failed update. (Distinct from
//                       `event_counts["failure"]` / `total`, which are
//                       cumulative EVENT counts across all history, not a
//                       current per-device state count — using event_counts
//                       here would double count devices that failed more
//                       than once.)
//   activeDeployments = GET /telemetry/overview carries NO deployment data at
//                       all (no source at this endpoint) — fetched instead
//                       from the correct endpoint, GET /deployments
//                       (`useDeployments()` in `use-deployments.ts`).
//                       server/internal/api/handlers_deployment.go:106-117
//                       `handleListDeployments` calls
//                       `s.repo.ListActiveDeployments` UNCONDITIONALLY (no
//                       status filter is read or applied) — so every item in
//                       that response IS already an active deployment, and
//                       `items.length` is the real, honest active-deployment
//                       count. No stat here is fabricated or guessed.
import { useTelemetryOverview as useTelemetryOverviewQuery } from './use-devices';
import { useDeployments } from './use-deployments';

/** Dashboard stat-card view model (consumed by dashboard-page.tsx). */
export interface TelemetryOverviewView {
  totalDevices: number;
  activeDeployments: number;
  pendingUpdates: number;
  failedDevices: number;
}

// The four non-terminal TelemetryEvent names that indicate an update is
// currently in flight for a device (submodules/ota-protocol/enums.go
// EventDownloadStarted/EventInstalling/EventInstalled/EventVerifying).
const IN_FLIGHT_STATES = ['download_started', 'installing', 'installed', 'verifying'] as const;

function sumByState(byState: Record<string, number> | undefined, keys: readonly string[]): number {
  if (!byState) return 0;
  return keys.reduce((sum, key) => sum + (byState[key] ?? 0), 0);
}

function sumAllStates(byState: Record<string, number> | undefined): number {
  if (!byState) return 0;
  return Object.values(byState).reduce((sum, n) => sum + n, 0);
}

export function useTelemetryOverview() {
  const overviewQuery = useTelemetryOverviewQuery();
  const deploymentsQuery = useDeployments();

  const byState = overviewQuery.data?.by_state;

  const data: TelemetryOverviewView | undefined = overviewQuery.data
    ? {
        totalDevices: sumAllStates(byState),
        activeDeployments: deploymentsQuery.data?.items.length ?? 0,
        pendingUpdates: sumByState(byState, IN_FLIGHT_STATES),
        failedDevices: byState?.failure ?? 0,
      }
    : undefined;

  return {
    data,
    // Gate loading on BOTH sources so the dashboard skeleton doesn't flash a
    // transient activeDeployments=0 while the second (deployments) query is
    // still in flight.
    isLoading: overviewQuery.isLoading || deploymentsQuery.isLoading,
    isError: overviewQuery.isError || deploymentsQuery.isError,
    error: overviewQuery.error ?? deploymentsQuery.error,
    refetch: () => {
      overviewQuery.refetch();
      deploymentsQuery.refetch();
    },
  };
}
