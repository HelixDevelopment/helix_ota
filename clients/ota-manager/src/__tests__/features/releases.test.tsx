import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import ReleasesPage from '@/features/releases/releases-page'

// ── Mocks ──────────────────────────────────────────────────────────────

const mockUseReleases = vi.hoisted(() => vi.fn())
const mockUseCreateRelease = vi.hoisted(() => vi.fn())
const mockRefetch = vi.hoisted(() => vi.fn())

vi.mock('@/hooks/useReleases', () => ({
  useReleases: mockUseReleases,
}))

vi.mock('@/hooks/useCreateRelease', () => ({
  useCreateRelease: mockUseCreateRelease,
}))

const mockUseUploadArtifact = vi.hoisted(() => vi.fn())
const mockUseArtifact = vi.hoisted(() => vi.fn())

vi.mock('@/hooks/useUploadArtifact', () => ({
  useUploadArtifact: mockUseUploadArtifact,
}))

vi.mock('@/hooks/useArtifact', () => ({
  useArtifact: mockUseArtifact,
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

const mockReleases = [
  {
    id: 'rel-001',
    version: '2.1.0',
    os: 'Android 14',
    targetModel: 'Orange Pi 5 Max',
    status: 'active',
    createdAt: '2026-06-15T08:00:00Z',
  },
  {
    id: 'rel-002',
    version: '2.0.1',
    os: 'Android 14',
    targetModel: 'Orange Pi 5 Max',
    status: 'superseded',
    createdAt: '2026-06-10T10:00:00Z',
  },
  {
    id: 'rel-003',
    version: '1.9.0',
    os: 'Android 13',
    targetModel: 'Orange Pi 5',
    status: 'draft',
    createdAt: '2026-06-01T14:00:00Z',
  },
]

// ── Tests ──────────────────────────────────────────────────────────────

describe('ReleasesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    mockUseReleases.mockReturnValue({
      data: mockReleases,
      isLoading: false,
      isError: false,
      error: null,
      refetch: mockRefetch,
    })

    mockUseCreateRelease.mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    })

    mockUseUploadArtifact.mockReturnValue({
      data: [{ id: 'art-1', name: 'firmware-v2.1.0.zip' }],
    })

    mockUseArtifact.mockReturnValue({
      data: { size: 42_000_000, checksum: 'abc123' },
    })
  })

  it('shows loading state', () => {
    mockUseReleases.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      refetch: mockRefetch,
    })

    const { container } = render(<ReleasesPage />)

    expect(screen.getByText('Releases')).toBeInTheDocument()
    const skeletons = container.querySelectorAll('.animate-pulse')
    expect(skeletons.length).toBeGreaterThan(0)
  })

  it('shows empty state when no releases', () => {
    mockUseReleases.mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
      error: null,
      refetch: mockRefetch,
    })

    render(<ReleasesPage />)

    expect(screen.getByText(/no releases yet/i)).toBeInTheDocument()
  })

  it('renders release list with mock data', () => {
    render(<ReleasesPage />)

    expect(screen.getByText('2.1.0')).toBeInTheDocument()
    expect(screen.getByText('2.0.1')).toBeInTheDocument()
    expect(screen.getByText('1.9.0')).toBeInTheDocument()

    expect(screen.getByText('Android 13')).toBeInTheDocument()
    const android14Elements = screen.getAllByText('Android 14')
    expect(android14Elements.length).toBe(2)
  })

  it('shows create dialog on button click', () => {
    render(<ReleasesPage />)

    const createBtn = screen.getByRole('button', { name: /create release/i })
    expect(createBtn).toBeInTheDocument()

    fireEvent.click(createBtn)

    expect(screen.getByText(/step 1 of 3/i)).toBeInTheDocument()
  })

  it('shows error state when API fails', () => {
    mockUseReleases.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error('Server unreachable'),
      refetch: mockRefetch,
    })

    render(<ReleasesPage />)

    expect(screen.getByText(/failed to load releases/i)).toBeInTheDocument()
  })
})
