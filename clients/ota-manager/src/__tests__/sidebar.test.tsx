import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Sidebar } from '@/features/layout/sidebar'

vi.mock('@tanstack/react-router', () => ({
  Link: 'a',
  useMatchRoute: () => () => false,
}))

vi.mock('@/stores/ui-store', () => ({
  useUiStore: (selector: any) => selector({
    sidebarCollapsed: false,
    toggleSidebar: vi.fn(),
  }),
}))

vi.mock('@/features/layout/project-switcher', () => ({
  ProjectSwitcher: () => null,
}))

describe('Sidebar', () => {
  it('renders navigation links', () => {
    render(<Sidebar />)
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
    expect(screen.getByText('Devices')).toBeInTheDocument()
    expect(screen.getByText('Releases')).toBeInTheDocument()
    expect(screen.getByText('Deployments')).toBeInTheDocument()
    expect(screen.getByText('Groups')).toBeInTheDocument()
    expect(screen.getByText('Audit Log')).toBeInTheDocument()
  })

  it('renders correctly when expanded', () => {
    render(<Sidebar />)
    expect(screen.getByRole('navigation')).toBeInTheDocument()
  })

  it('renders collapse toggle button', () => {
    render(<Sidebar />)
    const toggleButtons = screen.getAllByRole('button')
    expect(toggleButtons.length).toBeGreaterThan(0)
  })
})
