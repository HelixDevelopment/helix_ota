import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import DashboardPage from '@/features/dashboard/dashboard-page'

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
}))

// §11.4.115: mock at the module dashboard-page.tsx ACTUALLY imports
// (`@/hooks/useTelemetryOverview`, the mapping adapter) rather than the raw
// `@/hooks/use-devices` react-query hook it wraps. Mocking `use-devices`
// directly with an already-camelCase shape used to pass ONLY because the old
// `useTelemetryOverview.ts` was a bare, un-mapping re-export of it — the
// mapping logic itself is now covered separately (and RED->GREEN, per BUG-1)
// by `src/__tests__/use-telemetry-overview.test.tsx`. This test stays a pure
// "dashboard renders given an already-mapped view model" component test.
vi.mock('@/hooks/useTelemetryOverview', () => ({
  useTelemetryOverview: () => ({
    data: { totalDevices: 42, activeDeployments: 3, pendingUpdates: 7, failedDevices: 1 },
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
  }),
}))

vi.mock('@/hooks/use-audit', () => ({
  useAuditLog: () => ({
    data: [],
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
  }),
}))

function Wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

describe('DashboardPage', () => {
  it('renders stat cards with data', () => {
    render(<DashboardPage />, { wrapper: Wrapper })
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('7')).toBeInTheDocument()
    expect(screen.getByText('1')).toBeInTheDocument()
  })

  it('renders quick action buttons', () => {
    render(<DashboardPage />, { wrapper: Wrapper })
    expect(screen.getByText(/create release/i)).toBeInTheDocument()
    expect(screen.getByText(/create deployment/i)).toBeInTheDocument()
    expect(screen.getByText(/register device/i)).toBeInTheDocument()
    expect(screen.getByText(/manage groups/i)).toBeInTheDocument()
  })

  it('renders activity feed section', () => {
    render(<DashboardPage />, { wrapper: Wrapper })
    expect(screen.getByText('Recent Activity')).toBeInTheDocument()
  })
})
