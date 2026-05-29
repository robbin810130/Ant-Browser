import { Link } from 'react-router-dom'
import { Badge, Button, Table } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import { workbenchActionLabel, workbenchQueueLabels, workbenchQueueVariant } from '../presentation'
import type { WorkbenchRow } from '../types'

function shortTime(value: string | undefined) {
  if (!value) return '-'
  return value
}

export function WorkbenchTable({
  rows,
  loading,
  runningAction,
  onOpenDrawer,
  onAction,
}: {
  rows: WorkbenchRow[]
  loading: boolean
  runningAction: { shopId: string; action: WorkbenchRow['recommendedAction'] } | null
  onOpenDrawer: (row: WorkbenchRow) => void
  onAction: (row: WorkbenchRow) => void
}) {
  const columns: TableColumn<WorkbenchRow>[] = [
    {
      key: 'shop',
      title: '店铺',
      width: 190,
      render: (_, row) => (
        <div className="flex min-w-0 flex-col gap-1">
          <button
            type="button"
            className="max-w-[160px] truncate text-left text-sm font-medium text-[var(--color-accent)] hover:underline"
            title={row.shop.shopName || row.shop.shopId}
            onClick={(event) => {
              event.stopPropagation()
              onOpenDrawer(row)
            }}
          >
            {row.shop.shopName || row.shop.shopId}
          </button>
          <span className="max-w-[160px] truncate text-xs text-[var(--color-text-muted)]" title={row.shop.shopId}>
            {row.shop.shopId}
          </span>
        </div>
      ),
    },
    {
      key: 'queue',
      title: '执行状态',
      width: 96,
      render: (_, row) => (
        <Badge className="whitespace-nowrap" variant={workbenchQueueVariant(row.queue)}>
          {workbenchQueueLabels[row.queue]}
        </Badge>
      ),
    },
    {
      key: 'latestOpen',
      title: '最近打开',
      width: 128,
      render: (_, row) => (
        <span className="block max-w-[108px] truncate text-xs text-[var(--color-text-muted)]" title={row.evidence.latestOpen?.startedAt || ''}>
          {shortTime(row.evidence.latestOpen?.startedAt)}
        </span>
      ),
    },
    {
      key: 'latestValidation',
      title: '最近验证',
      width: 128,
      render: (_, row) => (
        <span className="block max-w-[108px] truncate text-xs text-[var(--color-text-muted)]" title={row.evidence.latestValidation?.startedAt || ''}>
          {shortTime(row.evidence.latestValidation?.startedAt)}
        </span>
      ),
    },
    {
      key: 'failure',
      title: '最近失败',
      width: 180,
      render: (_, row) => {
        const summary = row.workbenchState.evidenceText || '-'
        const title = [row.workbenchState.failureCode, row.failureMessage].filter(Boolean).join('\n') || '-'
        return (
          <span
            className="block max-w-[150px] truncate text-xs text-[var(--color-text-secondary)]"
            title={title}
          >
            {summary}
          </span>
        )
      },
    },
    {
      key: 'profile',
      title: '资料',
      width: 64,
      render: (_, row) => (
        <Link
          className="inline-flex text-sm text-[var(--color-accent)] hover:underline"
          to={`/shops/${encodeURIComponent(row.shop.shopId)}`}
          onClick={(event) => event.stopPropagation()}
        >
          查看
        </Link>
      ),
    },
    {
      key: 'actions',
      title: '推荐动作',
      align: 'right',
      width: 116,
      render: (_, row) => {
        const isRunningThisRow = runningAction?.shopId === row.shop.shopId
        return (
          <Button
            size="sm"
            className="w-full whitespace-nowrap px-3 sm:w-auto"
            loading={isRunningThisRow}
            disabled={Boolean(runningAction && !isRunningThisRow)}
            onClick={(event) => {
              event.stopPropagation()
              onAction(row)
            }}
            >
            {workbenchActionLabel(row.recommendedAction)}
          </Button>
        )
      },
    },
  ]

  return (
    <Table
      columns={columns}
      data={rows}
      rowKey={(row) => row.shop.shopId}
      loading={loading}
      emptyText="暂无可处理店铺"
      onRowClick={onOpenDrawer}
      maxHeight="calc(100vh - 240px)"
      className="min-w-0 [&_table]:table-fixed [&_td]:px-3 [&_th]:px-3 max-2xl:[&_td:nth-child(3)]:hidden max-2xl:[&_td:nth-child(4)]:hidden max-2xl:[&_td:nth-child(6)]:hidden max-2xl:[&_th:nth-child(3)]:hidden max-2xl:[&_th:nth-child(4)]:hidden max-2xl:[&_th:nth-child(6)]:hidden"
    />
  )
}
