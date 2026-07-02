import { useEffect, useMemo, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { Alert, Badge, Button, Card, DataTable, Select, toast } from '../../../shared/components'
import type { DataTableColumn } from '../../../shared/components'
import {
  fetchShopSpecialistTasks,
  fetchSpecialistTaskDetail,
  fetchTodaySpecialistTasks,
} from '../api'
import {
  specialistTaskDeadlineTone,
  specialistTaskPriorityLabel,
  specialistTaskStatusLabel,
} from '../presentation'
import type { SpecialistTaskListResponse, SpecialistTaskRecord } from '../types'
import { SpecialistTaskDrawer } from '../components/SpecialistTaskDrawer'

function statusVariant(status: string): 'default' | 'success' | 'error' | 'warning' | 'info' {
  if (status === 'completed') return 'success'
  if (status === 'appeal_in_review' || status === 'overdue') return 'warning'
  if (status === 'validation_failed_penalty' || status === 'rejected_rework') return 'error'
  if (status === 'submitted_pending_validation' || status === 'in_progress') return 'info'
  return 'default'
}

function deadlineClassName(deadlineAt: string | null) {
  const tone = specialistTaskDeadlineTone(deadlineAt)
  if (tone === 'danger') return 'text-[var(--color-error)]'
  if (tone === 'warning') return 'text-[var(--color-warning)]'
  return 'text-[var(--color-text-secondary)]'
}

function formatTime(value: string | null | undefined) {
  if (!value) return '-'
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

function emptyOverview(): SpecialistTaskListResponse {
  return {
    items: [],
    pagination: { page: 1, pageSize: 100, total: 0 },
    summary: {
      total: 0,
      pending: 0,
      inProgress: 0,
      submittedPendingValidation: 0,
      appealInReview: 0,
      overdue: 0,
      completed: 0,
    },
  }
}

function buildSummary(items: SpecialistTaskRecord[]): SpecialistTaskListResponse['summary'] {
  return {
    total: items.length,
    pending: items.filter((item) => item.status === 'pending').length,
    inProgress: items.filter((item) => item.status === 'in_progress').length,
    submittedPendingValidation: items.filter((item) => item.status === 'submitted_pending_validation').length,
    appealInReview: items.filter((item) => item.status === 'appeal_in_review').length,
    overdue: items.filter((item) => item.status === 'overdue').length,
    completed: items.filter((item) => item.status === 'completed').length,
  }
}

export function SpecialistTaskPanelPage() {
  const [searchParams] = useSearchParams()
  const shopId = searchParams.get('shopId')?.trim() || ''
  const [statusFilter, setStatusFilter] = useState('')
  const [overview, setOverview] = useState<SpecialistTaskListResponse>(emptyOverview)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState('')
  const [selectedTaskId, setSelectedTaskId] = useState('')
  const [selectedTask, setSelectedTask] = useState<SpecialistTaskRecord | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)

  async function load(silent = false) {
    if (silent) setRefreshing(true)
    else setLoading(true)
    setError('')
    try {
      const query = { pageSize: 100, status: statusFilter || undefined }
      const nextOverview = shopId
        ? await fetchShopSpecialistTasks(shopId, query)
        : await fetchTodaySpecialistTasks(query)
      setOverview(nextOverview)
    } catch (loadError) {
      const message = loadError instanceof Error ? loadError.message : '专员任务加载失败'
      setError(message)
      toast.error(`任务读取失败：${message}`)
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  async function openTask(task: SpecialistTaskRecord) {
    setSelectedTaskId(task.id)
    setSelectedTask(task)
    setDetailLoading(true)
    try {
      setSelectedTask(await fetchSpecialistTaskDetail(task.id))
    } catch (detailError) {
      toast.error(detailError instanceof Error ? `任务详情加载失败：${detailError.message}` : '任务详情加载失败，请稍后重试')
    } finally {
      setDetailLoading(false)
    }
  }

  async function refreshSelectedTask(taskId: string): Promise<SpecialistTaskRecord | null> {
    const normalizedTaskId = taskId.trim()
    if (!normalizedTaskId) return null
    const nextTask = await fetchSpecialistTaskDetail(normalizedTaskId)
    setSelectedTask(nextTask)
    return nextTask
  }

  function applyTaskMutation(nextTask: SpecialistTaskRecord) {
    setSelectedTask(nextTask)
    setOverview((current) => {
      let replaced = false
      const items = current.items.map((item) => {
        if (item.id !== nextTask.id) return item
        replaced = true
        return nextTask
      })
      if (!replaced) return current
      return {
        ...current,
        items,
        summary: buildSummary(items),
      }
    })
  }

  useEffect(() => {
    void load()
  }, [shopId, statusFilter])

  const columns = useMemo<DataTableColumn<SpecialistTaskRecord>[]>(() => [
    {
      title: '任务',
      key: 'title',
      width: 320,
      minWidth: 260,
      fixed: 'left',
      filterable: true,
      filterValue: (row) => `${row.title} ${row.description} ${row.id}`,
      render: (_, row) => (
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-[var(--color-text-primary)]">
            {row.title || row.id}
          </div>
          <div className="mt-1 truncate text-xs text-[var(--color-text-muted)]">
            {row.description || row.id}
          </div>
        </div>
      ),
    },
    {
      title: '店铺',
      key: 'shopName',
      width: 220,
      minWidth: 180,
      filterable: true,
      filterValue: (row) => `${row.shopName} ${row.shopId}`,
      render: (_, row) => (
        <div className="min-w-0">
          <div className="truncate text-sm text-[var(--color-text-primary)]">{row.shopName || row.shopId || '-'}</div>
          <div className="mt-1 truncate text-xs text-[var(--color-text-muted)]">{row.shopId || '-'}</div>
        </div>
      ),
    },
    {
      title: '状态',
      key: 'status',
      width: 130,
      minWidth: 110,
      filterable: true,
      render: (status: string) => (
        <Badge variant={statusVariant(String(status))} dot>{specialistTaskStatusLabel(String(status))}</Badge>
      ),
    },
    {
      title: '优先级',
      key: 'priority',
      width: 96,
      minWidth: 86,
      render: (priority: string) => specialistTaskPriorityLabel(priority),
    },
    {
      title: '截止时间',
      key: 'deadlineAt',
      width: 150,
      minWidth: 130,
      sortable: true,
      sortValue: (row) => row.deadlineAt ? new Date(row.deadlineAt).getTime() : 0,
      render: (deadlineAt: string | null) => (
        <span className={deadlineClassName(deadlineAt)}>{formatTime(deadlineAt)}</span>
      ),
    },
    {
      title: '更新时间',
      key: 'updatedAt',
      width: 150,
      minWidth: 130,
      sortable: true,
      sortValue: (row) => row.updatedAt ? new Date(row.updatedAt).getTime() : 0,
      render: (value: string) => (
        <span className="text-[var(--color-text-muted)]">{formatTime(value)}</span>
      ),
    },
  ], [])

  const counts = overview.summary
  const metrics = [
    ['任务数', counts.total],
    ['待开始', counts.pending],
    ['处理中', counts.inProgress],
    ['待校验', counts.submittedPendingValidation],
    ['申诉中', counts.appealInReview],
    ['已完成', counts.completed],
  ] as const

  return (
    <div className="specialist-task-panel flex h-full min-h-0 flex-col gap-4 overflow-hidden animate-fade-in">
      <div className="flex shrink-0 flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h1 className="truncate text-xl font-semibold text-[var(--color-text-primary)]">专员任务台</h1>
          <p className="mt-1 text-sm text-[var(--color-text-muted)]">
            {shopId ? `当前仅看店铺 ${shopId} 的专员任务。` : '按 SOP 执行、回传证据并提交验收。'}
          </p>
        </div>
        <div className="flex shrink-0 flex-col gap-2 sm:flex-row sm:items-center">
          <Select
            aria-label="任务状态"
            value={statusFilter}
            className="w-full sm:w-48"
            options={[
              { value: '', label: '全部状态' },
              { value: 'pending', label: '待开始' },
              { value: 'in_progress', label: '处理中' },
              { value: 'submitted_pending_validation', label: '已提交待校验' },
              { value: 'appeal_in_review', label: '申诉中' },
              { value: 'completed', label: '已完成' },
            ]}
            onChange={(event) => setStatusFilter(event.target.value)}
          />
          <Button
            variant="secondary"
            size="sm"
            className="w-full sm:w-auto"
            loading={refreshing}
            onClick={() => void load(true)}
          >
            <RefreshCw className="h-4 w-4" />
            刷新任务
          </Button>
        </div>
      </div>

      {shopId ? (
        <Alert
          type="info"
          title="店铺级任务视图"
          message="仅展示当前店铺的专员任务，适合同屏打开 1688 后台后逐项处理。"
        />
      ) : null}
      {error ? <Alert type="error" title="任务读取失败" message={error} /> : null}

      <div className="grid shrink-0 grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
        {metrics.map(([title, value]) => (
          <Card key={title} padding="sm">
            <p className="text-xs text-[var(--color-text-muted)]">{title}</p>
            <p className="mt-2 text-2xl font-semibold text-[var(--color-text-primary)]">{loading ? '-' : value}</p>
          </Card>
        ))}
      </div>

      <Card padding="none" className="flex min-h-0 flex-1 flex-col" bodyClassName="flex min-h-0 flex-1 flex-col">
        <DataTable
          rowKey="id"
          columns={columns}
          data={overview.items}
          loading={loading}
          emptyText={shopId ? '该店铺暂无专员任务' : '今日暂无专员任务'}
          fillHeight
          selectable
          storageKey="client-specialist-task-table-columns"
          onRowClick={(row) => void openTask(row)}
        />
      </Card>

      <SpecialistTaskDrawer
        open={Boolean(selectedTaskId)}
        task={selectedTask}
        loading={detailLoading}
        onClose={() => {
          setSelectedTaskId('')
          setSelectedTask(null)
        }}
        onTaskUpdated={applyTaskMutation}
        onRefreshTask={refreshSelectedTask}
        onReload={() => void load(true)}
      />
    </div>
  )
}
