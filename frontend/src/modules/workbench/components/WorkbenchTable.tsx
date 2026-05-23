import { Link } from 'react-router-dom'
import { Badge, Button, Table } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import type { WorkbenchQueueKey, WorkbenchRow } from '../types'

const queueLabels: Record<WorkbenchQueueKey, string> = {
  ready: '可打开',
  manual: '待验证',
  credential: '凭据处理',
  failed: '失败',
  running: '运行中',
  reclaim: '待回收',
}

function queueVariant(queue: WorkbenchQueueKey) {
  if (queue === 'ready') return 'success' as const
  if (queue === 'failed' || queue === 'reclaim') return 'error' as const
  if (queue === 'running') return 'info' as const
  return 'warning' as const
}

function actionLabel(action: WorkbenchRow['recommendedAction']) {
  if (action === 'open') return '打开后台'
  if (action === 'bind') return '更新凭据'
  if (action === 'validate') return '本机验证'
  if (action === 'retry') return '重试'
  if (action === 'refresh') return '刷新同步'
  if (action === 'core_management') return '配置内核'
  if (action === 'diagnostics') return '查看诊断'
  return '不可执行'
}

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
      width: 260,
      render: (_, row) => (
        <div className="flex min-w-0 flex-col gap-1">
          <button
            type="button"
            className="max-w-[220px] truncate text-left text-sm font-medium text-[var(--color-accent)] hover:underline"
            title={row.shop.shopName || row.shop.shopId}
            onClick={(event) => {
              event.stopPropagation()
              onOpenDrawer(row)
            }}
          >
            {row.shop.shopName || row.shop.shopId}
          </button>
          <span className="max-w-[220px] truncate text-xs text-[var(--color-text-muted)]" title={row.shop.shopId}>
            {row.shop.shopId}
          </span>
        </div>
      ),
    },
    {
      key: 'queue',
      title: '执行状态',
      width: 112,
      render: (_, row) => (
        <Badge className="whitespace-nowrap" variant={queueVariant(row.queue)}>
          {queueLabels[row.queue]}
        </Badge>
      ),
    },
    {
      key: 'latestOpen',
      title: '最近打开',
      width: 170,
      render: (_, row) => (
        <span className="block max-w-[160px] truncate text-xs text-[var(--color-text-muted)]" title={row.evidence.latestOpen?.startedAt || ''}>
          {shortTime(row.evidence.latestOpen?.startedAt)}
        </span>
      ),
    },
    {
      key: 'latestValidation',
      title: '最近验证',
      width: 170,
      render: (_, row) => (
        <span className="block max-w-[160px] truncate text-xs text-[var(--color-text-muted)]" title={row.evidence.latestValidation?.startedAt || ''}>
          {shortTime(row.evidence.latestValidation?.startedAt)}
        </span>
      ),
    },
    {
      key: 'failure',
      title: '最近失败',
      render: (_, row) => (
        <div className="max-w-[220px]">
          <div className="truncate text-xs text-[var(--color-text-secondary)]" title={row.failureCode || '-'}>
            {row.failureCode || '-'}
          </div>
          {row.failureMessage ? (
            <div className="mt-1 truncate text-xs text-[var(--color-text-muted)]" title={row.failureMessage}>
              {row.failureMessage}
            </div>
          ) : null}
        </div>
      ),
    },
    {
      key: 'profile',
      title: '资料',
      width: 80,
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
      width: 128,
      render: (_, row) => {
        const isRunningThisRow = runningAction?.shopId === row.shop.shopId
        return (
          <Button
            size="sm"
            className="w-full whitespace-nowrap sm:w-auto"
            loading={isRunningThisRow}
            disabled={Boolean(runningAction && !isRunningThisRow)}
            onClick={(event) => {
              event.stopPropagation()
              onAction(row)
            }}
          >
            {actionLabel(row.recommendedAction)}
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
      className="min-w-0"
    />
  )
}
