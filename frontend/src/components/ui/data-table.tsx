import * as React from 'react'
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
  type ColumnFiltersState,
} from '@tanstack/react-table'
import { ChevronDown, ChevronUp, ChevronsUpDown } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'

interface Props<T> {
  columns: ColumnDef<T, any>[]
  data: T[]
  /** When provided, shows a search box that filters by `globalFilterFn` (default: substring on visible columns). */
  searchable?: boolean
  searchPlaceholder?: string
  /** Page size — set to 0 to disable pagination. */
  pageSize?: number
  /** Render-prop for an empty data set. */
  emptyState?: React.ReactNode
  className?: string
  /** Optional row click handler. */
  onRowClick?: (row: T) => void
}

/**
 * DataTable is a thin wrapper around TanStack Table tuned for our
 * dark theme. It supports sorting, search, pagination, and row clicks.
 *
 * Pass columns + data and the rest is handled. For specialised use
 * (sticky headers, virtualisation), drop down to TanStack directly.
 */
export function DataTable<T>({
  columns,
  data,
  searchable,
  searchPlaceholder = 'Search…',
  pageSize = 25,
  emptyState,
  className,
  onRowClick,
}: Props<T>) {
  const [sorting, setSorting] = React.useState<SortingState>([])
  const [columnFilters, setColumnFilters] = React.useState<ColumnFiltersState>([])
  const [globalFilter, setGlobalFilter] = React.useState('')

  const table = useReactTable({
    data,
    columns,
    state: { sorting, columnFilters, globalFilter },
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: pageSize > 0 ? getPaginationRowModel() : undefined,
    initialState: { pagination: { pageSize: pageSize || 100 } },
  })

  const rows = table.getRowModel().rows

  return (
    <div className={cn('flex flex-col', className)}>
      {searchable && (
        <div className="mb-3 flex items-center gap-2">
          <input
            type="text"
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            placeholder={searchPlaceholder}
            className="h-8 w-full max-w-sm rounded-sm border border-border bg-bg-elevated px-3 text-sm text-fg-primary placeholder:text-fg-tertiary focus:border-accent focus:outline-none focus:bg-bg-surface"
          />
          <span className="text-xs text-fg-tertiary">
            {rows.length} of {data.length}
          </span>
        </div>
      )}

      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            {table.getHeaderGroups().map((hg) => (
              <tr key={hg.id} className="border-b border-border-subtle">
                {hg.headers.map((h) => {
                  const sortable = h.column.getCanSort()
                  const sortDir = h.column.getIsSorted()
                  return (
                    <th
                      key={h.id}
                      onClick={sortable ? h.column.getToggleSortingHandler() : undefined}
                      className={cn(
                        'px-3 py-2 text-left text-2xs font-semibold uppercase tracking-wider text-fg-tertiary',
                        sortable && 'cursor-pointer select-none hover:text-fg-primary',
                      )}
                      style={{ width: h.getSize() === 150 ? undefined : h.getSize() }}
                    >
                      <span className="inline-flex items-center gap-1">
                        {flexRender(h.column.columnDef.header, h.getContext())}
                        {sortable && (
                          sortDir === 'asc' ? (
                            <ChevronUp size={11} />
                          ) : sortDir === 'desc' ? (
                            <ChevronDown size={11} />
                          ) : (
                            <ChevronsUpDown size={11} className="opacity-40" />
                          )
                        )}
                      </span>
                    </th>
                  )
                })}
              </tr>
            ))}
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td colSpan={columns.length}>
                  {emptyState ?? (
                    <div className="px-3 py-12 text-center text-sm text-fg-tertiary">
                      No data
                    </div>
                  )}
                </td>
              </tr>
            ) : (
              rows.map((row) => (
                <tr
                  key={row.id}
                  onClick={() => onRowClick?.(row.original)}
                  className={cn(
                    'border-b border-border-subtle text-sm transition-colors last:border-0',
                    onRowClick && 'cursor-pointer hover:bg-bg-hover',
                  )}
                >
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id} className="px-3 py-2.5">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {pageSize > 0 && rows.length > 0 && (
        <div className="mt-3 flex items-center justify-between border-t border-border-subtle pt-3 text-xs text-fg-secondary">
          <div>
            Page {table.getState().pagination.pageIndex + 1} of {table.getPageCount()}
          </div>
          <div className="flex gap-1.5">
            <Button
              variant="default"
              size="sm"
              onClick={() => table.previousPage()}
              disabled={!table.getCanPreviousPage()}
            >
              ← Prev
            </Button>
            <Button
              variant="default"
              size="sm"
              onClick={() => table.nextPage()}
              disabled={!table.getCanNextPage()}
            >
              Next →
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
