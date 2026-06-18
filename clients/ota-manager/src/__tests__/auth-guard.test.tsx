import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AuthGuard } from '@/features/auth/auth-guard'

// ── Mocks ──────────────────────────────────────────────────────────────

// Mock LoginPage to avoid resolving its dependency tree (it imports from
// non-existent @/hooks/useLogin and other paths).
vi.mock('@/features/auth/login-page', () => ({
  default: () => <div data-testid="login-page">Login Page Mock</div>,
  LoginPage: () => <div data-testid="login-page">Login Page Mock</div>,
}))

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: vi.fn(),
}))

import { useAuthStore } from '@/stores/auth-store'

// ── Tests ──────────────────────────────────────────────────────────────

describe('AuthGuard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // default: authenticated admin with full permissions
    vi.mocked(useAuthStore).mockImplementation((selector?: any) => {
      const state = {
        token: 'test-token',
        refreshToken: 'test-refresh',
        user: {
          id: 'user-1',
          email: 'admin@example.com',
          display_name: 'Admin User',
          avatar_url: null,
          roles: ['admin'],
          permissions: ['devices:read', 'devices:write', 'releases:read', 'releases:write'],
        },
        isAuthenticated: true,
        setAuth: vi.fn(),
        setToken: vi.fn(),
        logout: vi.fn(),
      }
      return selector ? selector(state) : state
    })
  })

  it('renders children when authenticated', () => {
    render(
      <AuthGuard>
        <div data-testid="protected-content">Protected Content</div>
      </AuthGuard>,
    )

    expect(screen.getByTestId('protected-content')).toBeInTheDocument()
    expect(screen.getByText('Protected Content')).toBeInTheDocument()
  })

  it('renders login page when not authenticated', () => {
    vi.mocked(useAuthStore).mockImplementation((selector?: any) => {
      const state = {
        token: null,
        refreshToken: null,
        user: null,
        isAuthenticated: false,
        setAuth: vi.fn(),
        setToken: vi.fn(),
        logout: vi.fn(),
      }
      return selector ? selector(state) : state
    })

    render(
      <AuthGuard>
        <div data-testid="protected-content">Protected</div>
      </AuthGuard>,
    )

    expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument()
    expect(screen.getByTestId('login-page')).toBeInTheDocument()
  })

  it('shows "Access Denied" when user lacks required permissions', () => {
    vi.mocked(useAuthStore).mockImplementation((selector?: any) => {
      const state = {
        token: 'test-token',
        refreshToken: 'test-refresh',
        user: {
          id: 'user-1',
          email: 'viewer@example.com',
          display_name: 'Viewer User',
          avatar_url: null,
          roles: ['viewer'],
          permissions: ['devices:read'],
        },
        isAuthenticated: true,
        setAuth: vi.fn(),
        setToken: vi.fn(),
        logout: vi.fn(),
      }
      return selector ? selector(state) : state
    })

    render(
      <AuthGuard requiredPermissions={['devices:write', 'releases:read']}>
        <div data-testid="protected-content">Protected</div>
      </AuthGuard>,
    )

    expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument()
    expect(screen.getByText(/access denied/i)).toBeInTheDocument()
  })

  it('allows access when user has all required permissions', () => {
    vi.mocked(useAuthStore).mockImplementation((selector?: any) => {
      const state = {
        token: 'test-token',
        refreshToken: 'test-refresh',
        user: {
          id: 'user-1',
          email: 'editor@example.com',
          display_name: 'Editor User',
          avatar_url: null,
          roles: ['operator'],
          permissions: ['devices:read', 'devices:write', 'releases:read', 'releases:write', 'groups:read'],
        },
        isAuthenticated: true,
        setAuth: vi.fn(),
        setToken: vi.fn(),
        logout: vi.fn(),
      }
      return selector ? selector(state) : state
    })

    render(
      <AuthGuard requiredPermissions={['devices:read', 'releases:read']}>
        <div data-testid="protected-content">Protected</div>
      </AuthGuard>,
    )

    expect(screen.getByTestId('protected-content')).toBeInTheDocument()
  })
})
