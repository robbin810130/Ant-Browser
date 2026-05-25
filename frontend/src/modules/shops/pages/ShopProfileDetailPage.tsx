import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Activity, ArrowLeft, ExternalLink, ListChecks } from 'lucide-react'
import { Badge, Button, Card, Loading, toast } from '../../../shared/components'
import { fetchOperationTasks, operationTaskStatusLabel } from '../../operations/api'
import type { OperationTask, OperationTaskStatus } from '../../operations/types'
import { fetchWorkspaceRuns } from '../../runEvidence'
import type { RunRecord } from '../../runEvidence'
import { asmStatusKind, asmStatusLabel, dataCompletenessLabel, fetchShopProfile, sourceLabel } from '../api'
import type { ShopProfile } from '../types'

function DetailRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="grid gap-1 border-b border-[var(--color-border-muted)] py-3 last:border-0 sm:grid-cols-[140px_minmax(0,1fr)] sm:gap-4">
      <span className="text-sm text-[var(--color-text-muted)]">{label}</span>
      <span className="min-w-0 break-all text-sm text-[var(--color-text-primary)] sm:text-right">
        {value || '-'}
      </span>
    </div>
  )
}

function platformLabel(platformCode: string) {
  if (!platformCode) return '-'
  if (platformCode === 'alibaba') return '1688 / Alibaba'
  return platformCode
}

function asmBadge(status: string) {
  const kind = asmStatusKind(status)
  if (kind === 'connected') return <Badge variant="success">{asmStatusLabel(status)}</Badge>
  if (kind === 'error') return <Badge variant="error">{asmStatusLabel(status)}</Badge>
  return <Badge variant="warning">{asmStatusLabel(status)}</Badge>
}

function authorizationBadge(profile: ShopProfile) {
  const label = profile.authorizationStatusLabel || profile.authorizationStatus || '未配置'
  if (profile.authorizationStatus === 'ready' || profile.authorizationStatus === 'valid') {
    return <Badge variant="success">{label}</Badge>
  }
  if (profile.authorizationStatus === 'disabled' || profile.authorizationStatus === 'revoked') {
    return <Badge variant="error">{label}</Badge>
  }
  if (profile.authorizationStatus) {
    return <Badge variant="warning">{label}</Badge>
  }
  return <Badge variant="default">{label}</Badge>
}

function completenessBadge(status: string) {
  if (status === 'complete') return <Badge variant="success">{dataCompletenessLabel(status)}</Badge>
  if (status === 'partial') return <Badge variant="warning">{dataCompletenessLabel(status)}</Badge>
  return <Badge variant="default">{dataCompletenessLabel(status)}</Badge>
}

function taskStatusBadge(status: OperationTaskStatus) {
  if (status === 'running') return <Badge variant="info">{operationTaskStatusLabel(status)}</Badge>
  if (status === 'blocked') return <Badge variant="warning">{operationTaskStatusLabel(status)}</Badge>
  if (status === 'failed') return <Badge variant="error">{operationTaskStatusLabel(status)}</Badge>
  if (status === 'completed') return <Badge variant="success">{operationTaskStatusLabel(status)}</Badge>
  return <Badge variant="default">{operationTaskStatusLabel(status)}</Badge>
}

function runStatusBadge(status: string, label: string) {
  const text = label || status || '-'
  if (status === 'succeeded' || status === 'completed') return <Badge variant="success">{text}</Badge>
  if (status === 'failed') return <Badge variant="error">{text}</Badge>
  if (status === 'accepted' || status === 'authorizing' || status === 'launching' || status === 'capturing') {
    return <Badge variant="info">{text}</Badge>
  }
  if (status === 'awaiting_verification') return <Badge variant="warning">{text}</Badge>
  return <Badge variant="default">{text}</Badge>
}

function EmptyState({ text }: { text: string }) {
  return (
    <div className="rounded-md border border-dashed border-[var(--color-border-muted)] px-4 py-6 text-center text-sm text-[var(--color-text-muted)]">
      {text}
    </div>
  )
}

export function ShopProfileDetailPage() {
  const { shopId = '' } = useParams()
  const [profile, setProfile] = useState<ShopProfile | null>(null)
  const [tasks, setTasks] = useState<OperationTask[]>([])
  const [runs, setRuns] = useState<RunRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [activityLoading, setActivityLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    const normalizedShopId = shopId.trim()

    if (!normalizedShopId) {
      setProfile(null)
      setLoading(false)
      return () => {
        cancelled = true
      }
    }

    async function load() {
      setLoading(true)
      setActivityLoading(true)
      try {
        const next = await fetchShopProfile(normalizedShopId)
        if (!cancelled) setProfile(next)
      } catch (error) {
        console.error('load shop profile failed', error)
        toast.error('加载店铺详情失败')
        if (!cancelled) setProfile(null)
      } finally {
        if (!cancelled) setLoading(false)
      }

      try {
        const [nextTasks, nextRuns] = await Promise.all([
          fetchOperationTasks({ shopId: normalizedShopId, limit: 5 }),
          fetchWorkspaceRuns({ shopId: normalizedShopId, limit: 5 }),
        ])
        if (!cancelled) {
          setTasks(nextTasks)
          setRuns(nextRuns.items)
        }
      } catch (error) {
        console.error('load shop activity failed', error)
        toast.error('加载店铺运营动态失败')
        if (!cancelled) {
          setTasks([])
          setRuns([])
        }
      } finally {
        if (!cancelled) setActivityLoading(false)
      }
    }

    void load()

    return () => {
      cancelled = true
    }
  }, [shopId])

  if (loading) {
    return (
      <div className="p-8">
        <Loading text="加载店铺资料..." />
      </div>
    )
  }

  if (!profile) {
    return (
      <div className="space-y-4 p-8">
        <Link
          to="/shops"
          className="inline-flex items-center gap-1 text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]"
        >
          <ArrowLeft className="h-4 w-4" />
          返回店铺资料
        </Link>
        <p className="text-sm text-[var(--color-text-muted)]">店铺资料不存在</p>
      </div>
    )
  }

  return (
    <div className="space-y-5 p-5 animate-fade-in">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0">
          <Link
            to="/shops"
            className="mb-2 inline-flex items-center gap-1 text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]"
          >
            <ArrowLeft className="h-4 w-4" />
            返回店铺资料
          </Link>
          <h1 className="break-words text-xl font-semibold text-[var(--color-text-primary)]">
            {profile.shopName || profile.shopId}
          </h1>
          <p className="mt-1 break-all text-sm text-[var(--color-text-muted)]">
            {platformLabel(profile.platformCode)} · {profile.shopId}
          </p>
          <div className="mt-3 flex flex-wrap gap-2">
            {asmBadge(profile.asmStatus)}
            {authorizationBadge(profile)}
            {completenessBadge(profile.dataCompleteness)}
            <Badge variant="default">{sourceLabel(profile.source)}</Badge>
          </div>
        </div>
        <Link className="w-full shrink-0 sm:w-auto" to={`/workbench?shopId=${encodeURIComponent(profile.shopId)}`}>
          <Button size="sm" className="w-full sm:w-auto">
            <ExternalLink className="h-4 w-4" />
            去工作台
          </Button>
        </Link>
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card title="基础资料" subtitle="ASM 店铺业务主数据">
          <DetailRow label="店铺名称" value={profile.shopName} />
          <DetailRow label="Shop ID" value={profile.shopId} />
          <DetailRow label="平台" value={platformLabel(profile.platformCode)} />
          <DetailRow label="负责人" value={profile.ownerName} />
          <DetailRow label="主营类目" value={profile.mainCategory} />
        </Card>
        <Card title="ASM 与执行摘要" subtitle="执行详情在店铺工作台查看">
          <DetailRow label="ASM 状态" value={asmBadge(profile.asmStatus)} />
          <DetailRow label="授权状态" value={authorizationBadge(profile)} />
          <DetailRow label="数据完整度" value={completenessBadge(profile.dataCompleteness)} />
          <DetailRow label="最近同步" value={profile.lastSyncedAt} />
          <DetailRow label="数据来源" value={sourceLabel(profile.source)} />
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <Card
          title="该店铺运营任务"
          subtitle={activityLoading ? '正在读取任务状态' : `当前 ${tasks.length} 个任务`}
          actions={(
            <Link to={`/operations?shopId=${encodeURIComponent(profile.shopId)}`}>
              <Button size="sm" variant="secondary">
                <ListChecks className="h-4 w-4" />
                查看全部
              </Button>
            </Link>
          )}
        >
          <div className="space-y-3">
            {activityLoading ? (
              <Loading text="加载运营任务..." />
            ) : tasks.length === 0 ? (
              <EmptyState text="暂无该店铺运营任务" />
            ) : (
              tasks.map((task) => (
                <div key={task.taskId} className="rounded-md border border-[var(--color-border-muted)] px-3 py-3">
                  <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium text-[var(--color-text-primary)]" title={task.title || task.taskId}>
                        {task.title || task.taskId}
                      </p>
                      <p className="mt-1 truncate text-xs text-[var(--color-text-muted)]" title={task.blockedReason || task.failureMessage || task.taskType}>
                        {task.blockedReason || task.failureMessage || task.taskType || '-'}
                      </p>
                    </div>
                    <div className="shrink-0">{taskStatusBadge(task.status)}</div>
                  </div>
                  <div className="mt-2 flex flex-wrap gap-2 text-xs text-[var(--color-text-muted)]">
                    <span>{task.taskType || '-'}</span>
                    <span>{task.updatedAt || '-'}</span>
                  </div>
                </div>
              ))
            )}
          </div>
        </Card>

        <Card
          title="最近执行记录"
          subtitle={activityLoading ? '正在读取执行证据' : `最近 ${runs.length} 条`}
          actions={(
            <Link to={`/workbench?shopId=${encodeURIComponent(profile.shopId)}`}>
              <Button size="sm" variant="secondary">
                <Activity className="h-4 w-4" />
                去处理
              </Button>
            </Link>
          )}
        >
          <div className="space-y-3">
            {activityLoading ? (
              <Loading text="加载执行记录..." />
            ) : runs.length === 0 ? (
              <EmptyState text="暂无该店铺执行记录" />
            ) : (
              runs.map((run) => (
                <div key={run.runId} className="rounded-md border border-[var(--color-border-muted)] px-3 py-3">
                  <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium text-[var(--color-text-primary)]" title={run.runId}>
                        {run.taskType || run.taskId || run.runId}
                      </p>
                      <p className="mt-1 truncate text-xs text-[var(--color-text-muted)]" title={run.failureMessage || run.runtime?.pageTitle || run.runId}>
                        {run.failureMessage || run.runtime?.pageTitle || run.runId}
                      </p>
                    </div>
                    <div className="shrink-0">{runStatusBadge(run.status, run.statusLabel)}</div>
                  </div>
                  <div className="mt-2 flex flex-wrap gap-2 text-xs text-[var(--color-text-muted)]">
                    <span>{run.startedAt || '-'}</span>
                    <span>{run.finishedAt || '进行中'}</span>
                  </div>
                </div>
              ))
            )}
          </div>
        </Card>
      </div>
    </div>
  )
}
