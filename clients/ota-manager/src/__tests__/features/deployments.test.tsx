import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import DeploymentsPage from '@/features/deployments/deployments-page'

// ── Mocks ──────────────────────────────────────────────────────────────

const mockNavigate = vi.hoisted(() => vi.fn())
const mockUseDeployments = vi.hoisted(() => vi.fn())
const mockUseCreateDeployment = vi.hoisted(() => vi.fn())
const mockRefetch = vi.hoisted(() => vi.fn())

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mockNavigate,
}))

vi.mock('@/hooks/useDeployments', () => ({
  useDeployments: mockUseDeployments,
}))

vi.mock('@/hooks/useCreateDeployment', () => ({
  useCreateDeployment: mockUseCreateDeployment,
}))

vi.mock('@/hooks/use-toast', () => ({
  useToast: () => ({ toast: vi.fn() }),
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
              // React can't render objects directly — stringify them
              return <span key={key}>{typeof val === 'object' ? JSON.stringify(val) : String(val ?? '')}</span>
            })}
          </div>
        ))}
      </div>
    )
  },
}))

// Mock form and select to avoid engaging Radix UI in tests
vi.mock('@/components/ui/form', () => ({
  Form: ({ children }: any) => <>{children}</>,
  FormField: ({ render }: any) => render({ field: { value: '', onChange: vi.fn(), onBlur: vi.fn(), name: '' } }),
  FormItem: ({ children }: any) => <div>{children}</div>,
  FormLabel: ({ children }: any) => <label>{children}</label>,
  FormControl: ({ children }: any) => <>{children}</>,
  FormDescription: ({ children }: any) => <p>{children}</p>,
  FormMessage: () => null,
}))

vi.mock('@/components/ui/select', () => ({
  Select: ({ children }: any) => <div data-testid="select">{children}</div>,
  SelectTrigger: ({ children }: any) => <button data-testid="select-trigger">{children}</button>,
  SelectValue: ({ placeholder }: any) => <span>{placeholder}</span>,
  SelectContent: ({ children }: any) => <div>{children}</div>,
  SelectItem: ({ children, value }: any) => <div data-value={value}>{children}</div>,
}))

const mockDeployments = [
  {
    id: 'dep-001',
    targetGroupName: 'Production Devices',
    releaseVersion: '2.1.0',
    strategy: 'rolling',
    status: 'active',
    deviceStats: { succeeded: 15, failed: 0, pending: 35 },
    createdAt: '2026-06-18T08:00:00Z',
  },
  {
    id: 'dep-002',
    targetGroupName: 'Staging Devices',
    releaseVersion: '2.1.0-rc1',
    strategy: 'full',
    status: 'completed',
    deviceStats: { succeeded: 5, failed: 0, pending: 0 },
    createdAt: '2026-06-17T14:00:00Z',
  },
  {
    id: 'dep-003',
    targetGroupName: 'Canary Group',
    releaseVersion: '2.0.1',
    strategy: 'canary',
    status: 'failed',
    deviceStats: { succeeded: 1, failed: 2, pending: 0 },
    createdAt: '2026-06-16T10:00:00Z',
  },
]

// ── Tests ──────────────────────────────────────────────────────────────

describe('DeploymentsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    mockUseDeployments.mockReturnValue({
      data: mockDeployments,
      isLoading: false,
      isError: false,
      error: null,
      refetch: mockRefetch,
    })

    mockUseCreateDeployment.mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    })
  })

  it('shows loading state', () => {
    mockUseDeployments.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      refetch: mockRefetch,
    })

    const { container } = render(<DeploymentsPage />)

    expect(screen.getByText('Deployments')).toBeInTheDocument()
    const skeletons = container.querySelectorAll('.animate-pulse')
    expect(skeletons.length).toBeGreaterThan(0)
  })

  it('shows empty state when no deployments', () => {
    mockUseDeployments.mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
      error: null,
      refetch: mockRefetch,
    })

    render(<DeploymentsPage />)

    expect(screen.getByText(/no deployments yet/i)).toBeInTheDocument()
  })

  it('renders deployment list with mock data', () => {
    render(<DeploymentsPage />)

    expect(screen.getByText('Production Devices')).toBeInTheDocument()
    expect(screen.getByText('Staging Devices')).toBeInTheDocument()
    expect(screen.getByText('Canary Group')).toBeInTheDocument()

    expect(screen.getByText('2.1.0')).toBeInTheDocument()
    expect(screen.getByText('2.0.1')).toBeInTheDocument()
  })

  it('clicking a deployment row navigates to detail page', () => {
    render(<DeploymentsPage />)

    fireEvent.click(screen.getByText('Production Devices'))

    expect(mockNavigate).toHaveBeenCalledWith({
      to: '/deployments/$deploymentId',
      params: { deploymentId: 'dep-001' },
    })
  })

  it('shows error state when API fails', () => {
    mockUseDeployments.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error('Failed to fetch'),
      refetch: mockRefetch,
    })

    render(<DeploymentsPage />)

    expect(screen.getByText(/failed to load deployments/i)).toBeInTheDocument()
  })

  it('shows create deployment dialog on button click', () => {
    render(<DeploymentsPage />)

    const createBtn = screen.getByRole('button', { name: /create deployment/i })
    expect(createBtn).toBeInTheDocument()

    fireEvent.click(createBtn)

    // After clicking, both the button AND the dialog title show "Create Deployment"
    const elements = screen.getAllByText('Create Deployment')
    expect(elements.length).toBeGreaterThanOrEqual(2)
  })
})
