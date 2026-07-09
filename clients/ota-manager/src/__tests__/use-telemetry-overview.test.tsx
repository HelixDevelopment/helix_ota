import { describe, it, expect, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { useTelemetryOverview } from '@/hooks/useTelemetryOverview'

// §11.4.115 RED->GREEN regression guard for BUG-1 (telemetry-overview
// client/server model drift, docs/qa/20260710-bug1-telemetry-fix/).
//
// The REAL server response for GET /telemetry/overview
// (server/internal/api/handlers_telemetry.go:56-64, :168-193) is
// {event_counts, total, failure_rate, by_state} — NOT the fabricated
// camelCase {totalDevices, activeDeployments, pendingUpdates, failedDevices}
// the dashboard needs. This fixture is byte-for-byte the real wire shape.
//
// This test feeds that REAL-SHAPED fixture through the actual (unmocked)
// `use-devices.ts` + `use-deployments.ts` react-query hooks (only `apiGet` is
// mocked, at the transport boundary) into the `useTelemetryOverview` adapter
// under test, and asserts the mapped dashboard view model.
//
// RED (pre-fix, captured 2026-07-10): against the old bare re-export
// (`export { useTelemetryOverview } from './use-devices';`), `result.current
// .data` was the RAW server object — `data.totalDevices` was `undefined`,
// not `42`. This test FAILED with:
//   expect(received).toBe(expected)
//   Expected: 42
//   Received: undefined
// (full transcript: docs/qa/20260710-bug1-telemetry-fix/EVIDENCE.md)
//
// GREEN (post-fix): the new mapping adapter derives every stat from the real
// by_state fleet-device-state map (totalDevices/pendingUpdates/failedDevices)
// or the real GET /deployments active-deployments list (activeDeployments),
// per useTelemetryOverview.ts's documented field-by-field mapping.

const telemetryOverviewFixture = {
  // Cumulative event counts across all history — deliberately NOT the source
  // for failedDevices (see useTelemetryOverview.ts comment on why).
  event_counts: {
    download_started: 60,
    installing: 20,
    installed: 15,
    verifying: 10,
    success: 120,
    failure: 8,
  },
  total: 233,
  failure_rate: 8 / (120 + 8),
  // Fleet device count keyed by LAST-KNOWN update state (real
  // memory.go/postgres.go DeviceStateCounts shape). Sums to 42 — the total
  // registered-device count.
  by_state: {
    idle: 10,
    unknown: 1,
    download_started: 4,
    installing: 2,
    installed: 1,
    verifying: 1,
    success: 20,
    failure: 3,
  },
}

const deploymentsFixture = {
  items: [
    { deployment_id: 'dep-1', release_id: 'rel-1', group_ids: [], strategy: 'all-targets', rollout_percentage: 100, staged: false, status: 'active', created_at: '2026-07-01T00:00:00Z' },
    { deployment_id: 'dep-2', release_id: 'rel-2', group_ids: [], strategy: 'all-targets', rollout_percentage: 100, staged: false, status: 'active', created_at: '2026-07-02T00:00:00Z' },
    { deployment_id: 'dep-3', release_id: 'rel-3', group_ids: [], strategy: 'all-targets', rollout_percentage: 100, staged: false, status: 'active', created_at: '2026-07-03T00:00:00Z' },
  ],
  next_cursor: null,
}

vi.mock('@/lib/api-client', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api-client')>('@/lib/api-client')
  return {
    ...actual,
    apiGet: vi.fn((url: string) => {
      if (url === '/telemetry/overview') return Promise.resolve(telemetryOverviewFixture)
      if (url === '/deployments') return Promise.resolve(deploymentsFixture)
      return Promise.reject(new Error(`unexpected apiGet url in test: ${url}`))
    }),
  }
})

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

describe('useTelemetryOverview maps the REAL server shape to the dashboard view model (BUG-1)', () => {
  it('derives totalDevices from the sum of by_state (real by_state fixture sums to 42)', async () => {
    const { result } = renderHook(() => useTelemetryOverview(), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.data?.totalDevices).toBe(42)
  })

  it('derives pendingUpdates from the four non-terminal by_state buckets (4+2+1+1=8)', async () => {
    const { result } = renderHook(() => useTelemetryOverview(), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.data?.pendingUpdates).toBe(8)
  })

  it('derives failedDevices from by_state.failure (current-state count, NOT the cumulative event_counts.failure)', async () => {
    const { result } = renderHook(() => useTelemetryOverview(), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.data?.failedDevices).toBe(3)
    expect(result.current.data?.failedDevices).not.toBe(telemetryOverviewFixture.event_counts.failure)
  })

  it('derives activeDeployments from the real GET /deployments active-only list (3 items)', async () => {
    const { result } = renderHook(() => useTelemetryOverview(), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.data?.activeDeployments).toBe(3)
  })

  it('never surfaces the raw server field names on the returned view model', async () => {
    const { result } = renderHook(() => useTelemetryOverview(), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.data).not.toHaveProperty('event_counts')
    expect(result.current.data).not.toHaveProperty('by_state')
    expect(result.current.data).toEqual({
      totalDevices: 42,
      activeDeployments: 3,
      pendingUpdates: 8,
      failedDevices: 3,
    })
  })
})
