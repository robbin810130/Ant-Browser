import { Badge, Card } from '../../../shared/components'
import type { WorkbenchQueueKey, WorkbenchRow } from '../types'

type ActiveQueue = WorkbenchQueueKey | 'all'

const queueLabels: Record<WorkbenchQueueKey, string> = {
  ready: '可直接打开',
  manual: '待人工验证',
  credential: '凭据待处理',
  failed: '打开失败',
  running: '当前运行中',
  reclaim: '授权失效',
}

const queueOrder: WorkbenchQueueKey[] = ['ready', 'manual', 'credential', 'failed', 'running', 'reclaim']

function countRows(rows: WorkbenchRow[], queue: WorkbenchQueueKey) {
  return rows.filter((row) => row.queue === queue).length
}

function badgeVariant(queue: ActiveQueue) {
  if (queue === 'ready') return 'success' as const
  if (queue === 'failed' || queue === 'reclaim') return 'error' as const
  if (queue === 'manual' || queue === 'credential') return 'warning' as const
  if (queue === 'running') return 'info' as const
  return 'default' as const
}

export function WorkbenchQueues({
  rows,
  active,
  onSelect,
}: {
  rows: WorkbenchRow[]
  active: ActiveQueue
  onSelect: (queue: ActiveQueue) => void
}) {
  const itemClass = (selected: boolean) => [
    'flex min-h-10 w-full items-center justify-between gap-3 rounded-lg px-3 py-2 text-left text-sm transition-colors',
    selected
      ? 'bg-[var(--color-accent)] text-[var(--color-text-inverse)] shadow-sm'
      : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]',
  ].join(' ')

  return (
    <Card title="工作队列" subtitle="按当前可执行状态聚合" padding="sm">
      <div className="space-y-1">
        <button type="button" className={itemClass(active === 'all')} onClick={() => onSelect('all')}>
          <span className="min-w-0 truncate">全部店铺</span>
          <Badge className="min-w-8 shrink-0 justify-center" variant={active === 'all' ? 'default' : 'info'}>
            {rows.length}
          </Badge>
        </button>
        {queueOrder.map((queue) => (
          <button key={queue} type="button" className={itemClass(active === queue)} onClick={() => onSelect(queue)}>
            <span className="min-w-0 truncate">{queueLabels[queue]}</span>
            <Badge className="min-w-8 shrink-0 justify-center" variant={badgeVariant(queue)}>
              {countRows(rows, queue)}
            </Badge>
          </button>
        ))}
      </div>
    </Card>
  )
}
