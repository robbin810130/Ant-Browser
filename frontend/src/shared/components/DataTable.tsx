import { CSSProperties, MouseEvent as ReactMouseEvent, ReactNode, useEffect, useMemo, useState } from 'react'
import clsx from 'clsx'
import { ArrowDown, ArrowUp, ChevronLeft, ChevronRight, Columns, RotateCcw, Search } from 'lucide-react'
import { Button } from './Button'
import { Input } from './Form'

type SortOrder = 'asc' | 'desc'

export interface DataTableColumn<T> {
  key: string
  title: ReactNode
  width?: string | number
  minWidth?: number
  fixed?: 'left' | 'right'
  align?: 'left' | 'center' | 'right'
  render?: (value: any, record: T, index: number) => ReactNode
  sortValue?: (record: T) => string | number | null | undefined
  filterValue?: (record: T) => string | number | null | undefined
  sortable?: boolean
  filterable?: boolean
  resizable?: boolean
  hideable?: boolean
  defaultVisible?: boolean
}

type StoredVisibleColumns = string[] | { keys?: unknown; version?: number }

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
  selectable?: boolean
  fillHeight?: boolean
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

function parseColumnWidth(width: string | number | undefined, fallback = 140) {
  if (typeof width === 'number' && Number.isFinite(width)) return width
  if (typeof width === 'string') {
    const parsed = Number.parseInt(width, 10)
    if (Number.isFinite(parsed)) return parsed
  }
  return fallback
}

function clampColumnWidth(width: number, minWidth: number) {
  return Math.max(minWidth, Math.round(width))
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
  selectable = false,
  fillHeight = false,
  onRowClick,
}: DataTableProps<T>) {
  const columnKeys = useMemo(() => columns.map((column) => column.key), [columns])
  const widthStorageKey = storageKey ? `${storageKey}:widths` : ''
  const defaultVisibleKeys = useMemo(
    () => columns.filter((column) => column.defaultVisible !== false).map((column) => column.key),
    [columns],
  )
  const [visibleKeys, setVisibleKeys] = useState<string[]>(() => {
    if (!storageKey) return defaultVisibleKeys
    try {
      const parsed = JSON.parse(localStorage.getItem(storageKey) || '[]') as StoredVisibleColumns
      if (Array.isArray(parsed) && parsed.every((key) => columnKeys.includes(String(key)))) {
        const storedKeys = parsed.map(String)
        const mergedKeys = columnKeys.filter((key) => storedKeys.includes(key) || defaultVisibleKeys.includes(key))
        return parsed.length > 0 ? mergedKeys : defaultVisibleKeys
      }
      if (parsed && !Array.isArray(parsed) && Array.isArray(parsed.keys) && parsed.keys.every((key) => columnKeys.includes(String(key)))) {
        const storedKeys = parsed.keys.map(String)
        return storedKeys.length > 0 ? storedKeys : defaultVisibleKeys
      }
    } catch {
      // localStorage can be unavailable in restricted shells.
    }
    return defaultVisibleKeys
  })
  const [columnPanelOpen, setColumnPanelOpen] = useState(false)
  const [sortState, setSortState] = useState<{ key: string; order: SortOrder } | null>(null)
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(defaultPageSize)
  const [selectedKeys, setSelectedKeys] = useState<string[]>([])
  const [columnWidths, setColumnWidths] = useState<Record<string, number>>(() => {
    if (!widthStorageKey) return {}
    try {
      const parsed = JSON.parse(localStorage.getItem(widthStorageKey) || '{}')
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
      return Object.fromEntries(
        Object.entries(parsed)
          .filter(([key, value]) => columnKeys.includes(key) && typeof value === 'number' && Number.isFinite(value))
          .map(([key, value]) => [key, value as number]),
      )
    } catch {
      return {}
    }
  })

  useEffect(() => {
    if (!storageKey) return
    try {
      localStorage.setItem(storageKey, JSON.stringify({ version: 2, keys: visibleKeys }))
    } catch {
      // ignore storage write errors
    }
  }, [storageKey, visibleKeys])

  useEffect(() => {
    if (!widthStorageKey) return
    try {
      localStorage.setItem(widthStorageKey, JSON.stringify(columnWidths))
    } catch {
      // ignore storage write errors
    }
  }, [columnWidths, widthStorageKey])

  useEffect(() => {
    setPage(1)
  }, [keyword, pageSize, sortState])

  const visibleColumns = useMemo(
    () => columns.filter((column) => column.hideable === false || visibleKeys.includes(column.key)),
    [columns, visibleKeys],
  )

  const getColumnWidth = (column: DataTableColumn<T>) => {
    const minWidth = column.minWidth ?? 96
    return clampColumnWidth(columnWidths[column.key] ?? parseColumnWidth(column.width), minWidth)
  }

  const fixedLeftOffsets = useMemo(() => {
    let offset = selectable ? 48 : 0
    const offsets: Record<string, number> = {}
    for (const column of visibleColumns) {
      if (column.fixed === 'left') {
        offsets[column.key] = offset
        offset += getColumnWidth(column)
      }
    }
    return offsets
  }, [columnWidths, selectable, visibleColumns])

  const fixedRightOffsets = useMemo(() => {
    let offset = 0
    const offsets: Record<string, number> = {}
    for (let index = visibleColumns.length - 1; index >= 0; index -= 1) {
      const column = visibleColumns[index]
      if (column.fixed === 'right') {
        offsets[column.key] = offset
        offset += getColumnWidth(column)
      }
    }
    return offsets
  }, [columnWidths, visibleColumns])

  const tableWidth = useMemo(
    () => visibleColumns.reduce((total, column) => total + getColumnWidth(column), selectable ? 48 : 0),
    [columnWidths, selectable, visibleColumns],
  )

  const filteredRows = useMemo(() => {
    const normalizedKeyword = normalizeSearchValue(keyword)
    if (!normalizedKeyword) return data

    return data.filter((record) =>
      columns.some((column) => {
        const value = column.filterValue ? column.filterValue(record) : record[column.key]
        return normalizeSearchValue(value).includes(normalizedKeyword)
      }),
    )
  }, [columns, data, keyword])

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

  const getRowKey = (record: T, index: number) => {
    if (typeof rowKey === 'function') return rowKey(record)
    return String(record[rowKey] ?? index)
  }

  const pageRowKeys = useMemo(() => pageRows.map((record, index) => getRowKey(record, startIndex + index)), [pageRows, rowKey, startIndex])
  const selectedKeySet = useMemo(() => new Set(selectedKeys), [selectedKeys])
  const selectedPageCount = pageRowKeys.filter((key) => selectedKeySet.has(key)).length
  const allPageSelected = pageRowKeys.length > 0 && selectedPageCount === pageRowKeys.length
  const somePageSelected = selectedPageCount > 0 && selectedPageCount < pageRowKeys.length

  useEffect(() => {
    if (page !== safePage) setPage(safePage)
  }, [page, safePage])

  useEffect(() => {
    setSelectedKeys((current) => current.filter((key) => sortedRows.some((record, index) => getRowKey(record, index) === key)))
  }, [sortedRows, rowKey])

  const toggleSort = (column: DataTableColumn<T>) => {
    if (!column.sortable) return
    setSortState((current) => {
      if (current?.key !== column.key) return { key: column.key, order: 'asc' }
      if (current.order === 'asc') return { key: column.key, order: 'desc' }
      return null
    })
  }

  const toggleVisible = (key: string) => {
    const column = columns.find((item) => item.key === key)
    if (column?.hideable === false) return
    setVisibleKeys((current) => {
      if (current.includes(key)) {
        return current.length === 1 ? current : current.filter((item) => item !== key)
      }
      return columnKeys.filter((item) => item === key || current.includes(item))
    })
  }

  const resetTable = () => {
    setVisibleKeys(defaultVisibleKeys)
    setColumnWidths({})
    setKeyword('')
    setSortState(null)
    setPage(1)
    setSelectedKeys([])
  }

  const startColumnResize = (event: ReactMouseEvent, column: DataTableColumn<T>) => {
    if (column.resizable === false) return
    event.preventDefault()
    event.stopPropagation()

    const startX = event.clientX
    const startWidth = getColumnWidth(column)
    const minWidth = column.minWidth ?? 96

    const handleMouseMove = (moveEvent: MouseEvent) => {
      const nextWidth = clampColumnWidth(startWidth + moveEvent.clientX - startX, minWidth)
      setColumnWidths((current) => ({
        ...current,
        [column.key]: nextWidth,
      }))
    }

    const handleMouseUp = () => {
      document.body.classList.remove('client-data-table-resizing')
      window.removeEventListener('mousemove', handleMouseMove)
      window.removeEventListener('mouseup', handleMouseUp)
    }

    document.body.classList.add('client-data-table-resizing')
    window.addEventListener('mousemove', handleMouseMove)
    window.addEventListener('mouseup', handleMouseUp)
  }

  const getCellStyle = (column: DataTableColumn<T>): CSSProperties => {
    const width = getColumnWidth(column)
    const style: CSSProperties = {
      width,
      minWidth: width,
      maxWidth: width,
    }

    if (column.fixed === 'right') {
      style.position = 'sticky'
      style.right = fixedRightOffsets[column.key] ?? 0
    } else if (column.fixed === 'left') {
      style.position = 'sticky'
      style.left = fixedLeftOffsets[column.key] ?? 0
    }

    return style
  }

  const togglePageSelection = () => {
    if (pageRowKeys.length === 0) return
    setSelectedKeys((current) => {
      const next = new Set(current)
      if (allPageSelected) {
        pageRowKeys.forEach((key) => next.delete(key))
      } else {
        pageRowKeys.forEach((key) => next.add(key))
      }
      return Array.from(next)
    })
  }

  const toggleRowSelection = (key: string) => {
    setSelectedKeys((current) => {
      const next = new Set(current)
      if (next.has(key)) {
        next.delete(key)
      } else {
        next.add(key)
      }
      return Array.from(next)
    })
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
    <div className={clsx('client-data-table', fillHeight && 'client-data-table-fill')}>
      <div className="client-data-table-toolbar">
        <div className="relative min-w-0 flex-1 sm:max-w-md">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-muted)]" />
          <Input
            className="h-9 pl-9 text-sm"
            placeholder="搜索店铺名称、编码、运营、部门、类目..."
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
          />
        </div>
        <div className="relative flex shrink-0 items-center gap-2">
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
                      checked={column.hideable === false || visibleKeys.includes(column.key)}
                      disabled={column.hideable === false || (visibleKeys.includes(column.key) && visibleKeys.length === 1)}
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

      <div className="client-data-table-scroll" style={fillHeight ? undefined : { maxHeight }}>
        <table className="client-data-table-grid" style={{ width: tableWidth, minWidth: tableWidth }}>
          <thead className="sticky top-0 z-10">
            <tr>
              {selectable ? (
                <th
                  className="client-data-table-selection-cell client-data-table-cell-fixed-left client-data-table-cell-fixed-left-header bg-[var(--color-bg-muted)]"
                  style={{ left: 0 }}
                >
                  <input
                    type="checkbox"
                    aria-label="选择当前页店铺"
                    aria-checked={somePageSelected ? 'mixed' : allPageSelected}
                    checked={allPageSelected}
                    disabled={pageRowKeys.length === 0}
                    onChange={togglePageSelection}
                  />
                </th>
              ) : null}
              {visibleColumns.map((column) => (
                <th
                  key={column.key}
                  className={clsx(
                    'relative bg-[var(--color-bg-muted)] px-4 py-3 text-left align-top text-xs font-semibold text-[var(--color-text-muted)]',
                    column.fixed === 'left' && 'client-data-table-cell-fixed-left client-data-table-cell-fixed-left-header',
                    column.fixed === 'right' && 'client-data-table-cell-fixed-right client-data-table-cell-fixed-right-header',
                    column.align === 'center' && 'text-center',
                    column.align === 'right' && 'text-right',
                  )}
                  style={getCellStyle(column)}
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
                  </div>
                  {column.resizable !== false ? (
                    <button
                      type="button"
                      aria-label={`调整${String(column.title)}列宽`}
                      className="client-data-table-resize-handle"
                      onMouseDown={(event) => startColumnResize(event, column)}
                    />
                  ) : null}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--color-border-muted)] bg-[var(--color-bg-surface)]">
            {pageRows.length === 0 ? (
              <tr>
                <td colSpan={visibleColumns.length + (selectable ? 1 : 0)} className="px-4 py-16 text-center text-sm text-[var(--color-text-muted)]">
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
                  {selectable ? (
                    <td
                      className="client-data-table-selection-cell client-data-table-cell-fixed-left bg-[var(--color-bg-surface)]"
                      style={{ left: 0 }}
                      onClick={(event) => event.stopPropagation()}
                    >
                      <input
                        type="checkbox"
                        aria-label={`选择第 ${startIndex + index + 1} 行`}
                        checked={selectedKeySet.has(getRowKey(record, startIndex + index))}
                        onChange={() => toggleRowSelection(getRowKey(record, startIndex + index))}
                      />
                    </td>
                  ) : null}
                  {visibleColumns.map((column) => (
                    <td
                      key={column.key}
                      className={clsx(
                        'px-4 py-3.5 align-top text-sm text-[var(--color-text-secondary)]',
                        column.fixed === 'left' && 'client-data-table-cell-fixed-left',
                        column.fixed === 'right' && 'client-data-table-cell-fixed-right',
                        column.align === 'center' && 'text-center',
                        column.align === 'right' && 'text-right',
                      )}
                      style={getCellStyle(column)}
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
        <div className="flex min-w-0 flex-wrap items-center gap-3">
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
          <span className="text-xs text-[var(--color-text-muted)]">
            {sortedRows.length > 0 ? `第 ${startIndex + 1} - ${endIndex} 条，共 ${sortedRows.length} 条` : '第 0 - 0 条，共 0 条'}
          </span>
        </div>
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
