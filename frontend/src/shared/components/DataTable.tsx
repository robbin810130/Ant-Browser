import { ReactNode, useEffect, useMemo, useState } from 'react'
import clsx from 'clsx'
import { ArrowDown, ArrowUp, ChevronLeft, ChevronRight, Columns, Filter, RotateCcw } from 'lucide-react'
import { Button } from './Button'
import { Input } from './Form'

type SortOrder = 'asc' | 'desc'

export interface DataTableColumn<T> {
  key: string
  title: ReactNode
  width?: string | number
  align?: 'left' | 'center' | 'right'
  render?: (value: any, record: T, index: number) => ReactNode
  sortValue?: (record: T) => string | number | null | undefined
  filterValue?: (record: T) => string | number | null | undefined
  sortable?: boolean
  filterable?: boolean
  defaultVisible?: boolean
}

interface DataTableProps<T> {
  columns: DataTableColumn<T>[]
  data: T[]
  rowKey: string | ((record: T) => string)
  loading?: boolean
  emptyText?: string
  storageKey?: string
  maxHeight?: string
  defaultPageSize?: number
  pageSizeOptions?: number[]
  onRowClick?: (record: T) => void
}

function normalizeSearchValue(value: unknown) {
  return String(value ?? '').trim().toLowerCase()
}

function compareValues(left: unknown, right: unknown, order: SortOrder) {
  const direction = order === 'asc' ? 1 : -1
  const leftNumber = typeof left === 'number' ? left : Number.NaN
  const rightNumber = typeof right === 'number' ? right : Number.NaN

  if (Number.isFinite(leftNumber) && Number.isFinite(rightNumber)) {
    return (leftNumber - rightNumber) * direction
  }

  return String(left ?? '').localeCompare(String(right ?? ''), 'zh-CN', { numeric: true }) * direction
}

export function DataTable<T extends Record<string, any>>({
  columns,
  data,
  rowKey,
  loading = false,
  emptyText = '暂无数据',
  storageKey,
  maxHeight = 'calc(100vh - 360px)',
  defaultPageSize = 25,
  pageSizeOptions = [25, 50, 100],
  onRowClick,
}: DataTableProps<T>) {
  const columnKeys = useMemo(() => columns.map((column) => column.key), [columns])
  const defaultVisibleKeys = useMemo(
    () => columns.filter((column) => column.defaultVisible !== false).map((column) => column.key),
    [columns],
  )
  const [visibleKeys, setVisibleKeys] = useState<string[]>(() => {
    if (!storageKey) return defaultVisibleKeys
    try {
      const parsed = JSON.parse(localStorage.getItem(storageKey) || '[]')
      if (Array.isArray(parsed) && parsed.every((key) => columnKeys.includes(String(key)))) {
        return parsed.length > 0 ? parsed.map(String) : defaultVisibleKeys
      }
    } catch {
      // localStorage can be unavailable in restricted shells.
    }
    return defaultVisibleKeys
  })
  const [columnPanelOpen, setColumnPanelOpen] = useState(false)
  const [sortState, setSortState] = useState<{ key: string; order: SortOrder } | null>(null)
  const [filters, setFilters] = useState<Record<string, string>>({})
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(defaultPageSize)

  useEffect(() => {
    if (!storageKey) return
    try {
      localStorage.setItem(storageKey, JSON.stringify(visibleKeys))
    } catch {
      // ignore storage write errors
    }
  }, [storageKey, visibleKeys])

  useEffect(() => {
    setPage(1)
  }, [filters, pageSize, sortState])

  const visibleColumns = useMemo(
    () => columns.filter((column) => visibleKeys.includes(column.key)),
    [columns, visibleKeys],
  )

  const filteredRows = useMemo(() => {
    return data.filter((record) =>
      columns.every((column) => {
        const filter = filters[column.key]?.trim()
        if (!filter) return true
        const value = column.filterValue ? column.filterValue(record) : record[column.key]
        return normalizeSearchValue(value).includes(normalizeSearchValue(filter))
      }),
    )
  }, [columns, data, filters])

  const sortedRows = useMemo(() => {
    if (!sortState) return filteredRows
    const column = columns.find((item) => item.key === sortState.key)
    if (!column) return filteredRows
    return [...filteredRows].sort((left, right) => {
      const leftValue = column.sortValue ? column.sortValue(left) : left[column.key]
      const rightValue = column.sortValue ? column.sortValue(right) : right[column.key]
      return compareValues(leftValue, rightValue, sortState.order)
    })
  }, [columns, filteredRows, sortState])

  const totalPages = Math.max(1, Math.ceil(sortedRows.length / pageSize))
  const safePage = Math.min(page, totalPages)
  const startIndex = (safePage - 1) * pageSize
  const endIndex = Math.min(startIndex + pageSize, sortedRows.length)
  const pageRows = sortedRows.slice(startIndex, endIndex)

  useEffect(() => {
    if (page !== safePage) setPage(safePage)
  }, [page, safePage])

  const getRowKey = (record: T, index: number) => {
    if (typeof rowKey === 'function') return rowKey(record)
    return String(record[rowKey] ?? index)
  }

  const toggleSort = (column: DataTableColumn<T>) => {
    if (!column.sortable) return
    setSortState((current) => {
      if (current?.key !== column.key) return { key: column.key, order: 'asc' }
      if (current.order === 'asc') return { key: column.key, order: 'desc' }
      return null
    })
  }

  const toggleVisible = (key: string) => {
    setVisibleKeys((current) => {
      if (current.includes(key)) {
        return current.length === 1 ? current : current.filter((item) => item !== key)
      }
      return columnKeys.filter((item) => item === key || current.includes(item))
    })
  }

  const resetTable = () => {
    setVisibleKeys(defaultVisibleKeys)
    setFilters({})
    setSortState(null)
    setPage(1)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16" style={{ maxHeight }}>
        <div className="flex flex-col items-center gap-3">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-[var(--color-border-default)] border-t-[var(--color-accent)]" />
          <span className="text-sm text-[var(--color-text-muted)]">加载中...</span>
        </div>
      </div>
    )
  }

  return (
    <div className="client-data-table">
      <div className="client-data-table-toolbar">
        <div className="text-xs text-[var(--color-text-muted)]">
          {sortedRows.length > 0 ? `第 ${startIndex + 1} - ${endIndex} 条，共 ${sortedRows.length} 条` : '第 0 - 0 条，共 0 条'}
        </div>
        <div className="relative flex items-center gap-2">
          <Button variant="secondary" size="sm" onClick={resetTable}>
            <RotateCcw className="h-3.5 w-3.5" />
            重置
          </Button>
          <Button variant="secondary" size="sm" onClick={() => setColumnPanelOpen((current) => !current)}>
            <Columns className="h-3.5 w-3.5" />
            列
          </Button>
          {columnPanelOpen ? (
            <div className="client-data-table-column-panel">
              <div className="mb-2 text-xs font-semibold text-[var(--color-text-primary)]">显示/隐藏列</div>
              <div className="space-y-1">
                {columns.map((column) => (
                  <label key={column.key} className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-xs text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]">
                    <input
                      type="checkbox"
                      checked={visibleKeys.includes(column.key)}
                      disabled={visibleKeys.includes(column.key) && visibleKeys.length === 1}
                      onChange={() => toggleVisible(column.key)}
                    />
                    <span>{column.title}</span>
                  </label>
                ))}
              </div>
            </div>
          ) : null}
        </div>
      </div>

      <div className="overflow-auto" style={{ maxHeight }}>
        <table className="min-w-full">
          <thead className="sticky top-0 z-10">
            <tr>
              {visibleColumns.map((column) => (
                <th
                  key={column.key}
                  className={clsx(
                    'bg-[var(--color-bg-muted)] px-4 py-3 text-left align-top text-xs font-semibold text-[var(--color-text-muted)]',
                    column.align === 'center' && 'text-center',
                    column.align === 'right' && 'text-right',
                  )}
                  style={{ width: column.width }}
                >
                  <div className="flex min-w-0 items-center gap-2">
                    <button
                      type="button"
                      className={clsx(
                        'flex min-w-0 items-center gap-1 text-left',
                        column.sortable && 'hover:text-[var(--color-text-primary)]',
                      )}
                      disabled={!column.sortable}
                      onClick={() => toggleSort(column)}
                    >
                      <span className="truncate">{column.title}</span>
                      {sortState?.key === column.key ? (
                        sortState.order === 'asc' ? <ArrowUp className="h-3.5 w-3.5" /> : <ArrowDown className="h-3.5 w-3.5" />
                      ) : null}
                    </button>
                    {column.filterable ? <Filter className="h-3.5 w-3.5 opacity-50" /> : null}
                  </div>
                  {column.filterable ? (
                    <Input
                      className="mt-2 h-7 text-xs"
                      placeholder="筛选"
                      value={filters[column.key] || ''}
                      onChange={(event) =>
                        setFilters((current) => ({
                          ...current,
                          [column.key]: event.target.value,
                        }))
                      }
                    />
                  ) : null}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--color-border-muted)] bg-[var(--color-bg-surface)]">
            {pageRows.length === 0 ? (
              <tr>
                <td colSpan={visibleColumns.length} className="px-4 py-16 text-center text-sm text-[var(--color-text-muted)]">
                  {emptyText}
                </td>
              </tr>
            ) : (
              pageRows.map((record, index) => (
                <tr
                  key={getRowKey(record, startIndex + index)}
                  className={clsx('transition-colors hover:bg-[var(--color-bg-muted)]/60', onRowClick && 'cursor-pointer')}
                  onClick={() => onRowClick?.(record)}
                >
                  {visibleColumns.map((column) => (
                    <td
                      key={column.key}
                      className={clsx(
                        'px-4 py-3.5 align-top text-sm text-[var(--color-text-secondary)]',
                        column.align === 'center' && 'text-center',
                        column.align === 'right' && 'text-right',
                      )}
                    >
                      {column.render ? column.render(record[column.key], record, startIndex + index) : record[column.key]}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="client-data-table-pagination">
        <select
          className="rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-2 py-1 text-xs text-[var(--color-text-secondary)]"
          value={pageSize}
          onChange={(event) => setPageSize(Number(event.target.value) || defaultPageSize)}
        >
          {pageSizeOptions.map((option) => (
            <option key={option} value={option}>
              {option} / 页
            </option>
          ))}
        </select>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="sm" disabled={safePage <= 1} onClick={() => setPage((current) => Math.max(1, current - 1))}>
            <ChevronLeft className="h-3.5 w-3.5" />
          </Button>
          <span className="text-xs text-[var(--color-text-muted)]">
            {safePage} / {totalPages}
          </span>
          <Button variant="secondary" size="sm" disabled={safePage >= totalPages} onClick={() => setPage((current) => Math.min(totalPages, current + 1))}>
            <ChevronRight className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </div>
  )
}
