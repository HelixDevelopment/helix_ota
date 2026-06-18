import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { DataTable } from '@/components/data-table/data-table'
import type { ColumnDef } from '@tanstack/react-table'

interface TestItem {
  id: string
  name: string
}

const columns: ColumnDef<TestItem>[] = [
  { accessorKey: 'id', header: 'ID' },
  { accessorKey: 'name', header: 'Name' },
]

const data: TestItem[] = [
  { id: '1', name: 'Alpha' },
  { id: '2', name: 'Beta' },
]

describe('DataTable', () => {
  it('renders column headers', () => {
    render(<DataTable columns={columns} data={data} />)
    expect(screen.getByText('ID')).toBeInTheDocument()
    expect(screen.getByText('Name')).toBeInTheDocument()
  })

  it('renders data rows', () => {
    render(<DataTable columns={columns} data={data} />)
    expect(screen.getByText('Alpha')).toBeInTheDocument()
    expect(screen.getByText('Beta')).toBeInTheDocument()
  })

  it('shows loading skeleton when loading', () => {
    const { container } = render(<DataTable columns={columns} data={[]} loading={true} />)
    // Should have skeleton rows (not the empty message)
    expect(screen.queryByText('No results found.')).not.toBeInTheDocument()
    expect(container.querySelector('.animate-pulse')).toBeTruthy()
  })

  it('shows empty message when no data', () => {
    render(<DataTable columns={columns} data={[]} />)
    expect(screen.getByText('No results found.')).toBeInTheDocument()
  })
})
