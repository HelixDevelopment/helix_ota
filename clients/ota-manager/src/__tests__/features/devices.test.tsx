import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import DevicesPage from '@/features/devices/devices-page'

// ── Mocks ──────────────────────────────────────────────────────────────

const mockUseDevices = vi.hoisted(() => vi.fn())
const mockRefresh = vi.hoisted(() => vi.fn())

vi.mock('@/hooks/useDevices', () => ({
  useDevices: mockUseDevices,
}))

// Mock DataTable to avoid react-table v8 + vitest compatibility issue
// where cell.columnDef is undefined under vitest's module resolution.
vi.mock('@/components/data-table/data-table', () => ({
  DataTable: ({ columns, data, loading, onRowClick, emptyMessage }: any) => {
    if (loading) return <div data-testid="datatable-skeleton">Loading...</div>
    if (!data || data.length === 0) return <div>{emptyMessage || 'No results found.'}</div>
    return (
      <div data-testid="datatable-body">
        {data.map((row: any, i: number) => (
          <div
            key={row.id || i}
            data-testid={`datatable-row-${row.id}`}
            onClick={() => onRowClick?.(row)}
          >
            {columns.map((col: any) => {
              const key = col.accessorKey || col.id
              const val = row[key]
              return <span key={key}>{typeof val === 'object' ? JSON.stringify(val) : String(val ?? '')}</span>
            })}
          </div>
        ))}
      </div>
    )
  },
}))

const mockDevices = [
  {
    id: 'dev-001',
    hardwareId: 'RK3588-7A3F',
    os: 'Android 14',
    version: '2.1.0',
    status: 'online' as const,
    lastSeen: '2026-06-18T12:00:00Z',
    targetModel: 'Orange Pi 5 Max',
    currentSlot: 'a' as const,
    healthStatus: 'healthy' as const,
  },
  {
    id: 'dev-002',
    hardwareId: 'RK3588-9B2C',
    os: 'Android 14',
    version: '2.0.1',
    status: 'offline' as const,
    lastSeen: '2026-06-17T08:30:00Z',
    targetModel: 'Orange Pi 5 Max',
    currentSlot: 'b' as const,
    healthStatus: 'degraded' as const,
  },
  {
    id: 'dev-003',
    hardwareId: 'RK3588-4D1E',
    os: 'Android 13',
    version: '1.9.0',
    status: 'updating' as const,
    lastSeen: '2026-06-18T11:45:00Z',
    targetModel: 'Orange Pi 5',
    healthStatus: 'critical' as const,
  },
]

// ── Tests ──────────────────────────────────────────────────────────────

describe('DevicesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows loading state', () => {
    mockUseDevices.mockReturnValue({
      devices: undefined,
      loading: true,
      error: null,
      refresh: mockRefresh,
    })

    const { container } = render(<DevicesPage />)

    expect(screen.getByText('Devices')).toBeInTheDocument()
    const skeletons = container.querySelectorAll('.animate-pulse')
    expect(skeletons.length).toBeGreaterThan(0)
  })

  it('shows error state', () => {
    mockUseDevices.mockReturnValue({
      devices: undefined,
      loading: false,
      error: 'Failed to fetch device list',
      refresh: mockRefresh,
    })

    render(<DevicesPage />)

    expect(screen.getByText(/failed to load devices/i)).toBeInTheDocument()
    expect(screen.getByText(/failed to fetch device list/i)).toBeInTheDocument()
  })

  it('shows empty state when no devices', () => {
    mockUseDevices.mockReturnValue({
      devices: [],
      loading: false,
      error: null,
      refresh: mockRefresh,
    })

    render(<DevicesPage />)

    expect(screen.getByText(/no devices registered/i)).toBeInTheDocument()
  })

  it('renders device list with mock data', () => {
    mockUseDevices.mockReturnValue({
      devices: mockDevices,
      loading: false,
      error: null,
      refresh: mockRefresh,
    })

    render(<DevicesPage />)

    expect(screen.getByText('dev-001')).toBeInTheDocument()
    expect(screen.getByText('dev-002')).toBeInTheDocument()
    expect(screen.getByText('dev-003')).toBeInTheDocument()

    expect(screen.getByText('online')).toBeInTheDocument()
    expect(screen.getByText('offline')).toBeInTheDocument()
    expect(screen.getByText('updating')).toBeInTheDocument()
  })

  it('calls refresh when retry button is clicked in empty state', () => {
    mockUseDevices.mockReturnValue({
      devices: [],
      loading: false,
      error: null,
      refresh: mockRefresh,
    })

    render(<DevicesPage />)

    fireEvent.click(screen.getByRole('button', { name: /refresh/i }))
    expect(mockRefresh).toHaveBeenCalled()
  })
})
