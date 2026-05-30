import { useEffect, useMemo, useState } from 'react'
import { ArrowRight, CheckCircle2, HardDrive, Link2, Monitor, RefreshCw, Store, WifiOff } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Button, Card, toast } from '../../../shared/components'
import {
  deriveWorkspaceDashboardStats,
  fetchWorkspaceAuthorizedShops,
  fetchWorkspaceSummary,
} from '../api'
import { WorkspaceSummaryCards } from '../components/WorkspaceSummaryCards'
import type { WorkspaceAuthorizedShop, WorkspaceSummary } from '../types'

function statusText(summary: WorkspaceSummary) {
  if (!summary.serverReachable) return '工作台服务不可达'
  if (!summary.antRuntimeReachable) return 'Ant Runtime 未连通'
  if (!summary.sessionReady) return '会话未就绪'
  return summary.deviceStatus || summary.agentStatus || summary.status || 'ready'
}

function statusTone(summary: WorkspaceSummary) {
  if (!summary.serverReachable || !summary.antRuntimeReachable) {
    return 'text-amber-600 bg-amber-50 border-amber-200'
  }
  if (!summary.sessionReady) {
    return 'text-slate-600 bg-slate-100 border-slate-200'
  }
  return 'text-emerald-600 bg-emerald-50 border-emerald-200'
}

export function WorkspaceDashboardPage() {
  const [summary, setSummary] = useState<WorkspaceSummary | null>(null)
  const [shops, setShops] = useState<WorkspaceAuthorizedShop[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const stats = useMemo(() => deriveWorkspaceDashboardStats(shops), [shops])

  async function load(silent = false) {
    if (silent) {
      setRefreshing(true)
    } else {
      setLoading(true)
    }

    try {
      const [nextSummary, nextShops] = await Promise.all([
        fetchWorkspaceSummary(),
        fetchWorkspaceAuthorizedShops(),
      ])
      setSummary(nextSummary)
      setShops(nextShops)
    } catch (error) {
      console.error('load workspace dashboard failed', error)
      toast.error('加载 Workspace 总览失败')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const derivedSummary = summary ?? {
    status: '',
    agentStatus: '',
    sessionReady: false,
    serverReachable: false,
    antRuntimeReachable: false,
    activeRunCount: 0,
    deviceId: '',
    deviceStatus: '',
  }

  const readyRate = stats.totalAccounts > 0
    ? `${Math.round((stats.readyShopCount / stats.totalAccounts) * 100)}%`
    : '0%'

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">控制台</h1>
          <p className="mt-1 text-sm text-[var(--color-text-muted)]">Workspace 总览页，只看状态，不塞店铺主表。</p>
        </div>
        <Button variant="secondary" size="sm" onClick={() => void load(true)} loading={refreshing}>
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </div>

      <WorkspaceSummaryCards stats={stats} loading={loading} />

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[1.4fr_1fr]">
        <Card
          title="设备与连接状态"
          subtitle="展示当前 Workspace Host 与 Ant Runtime 的整体可用性"
          actions={(
            <div className={`rounded-full border px-3 py-1 text-xs font-medium ${statusTone(derivedSummary)}`}>
              {loading ? '加载中' : statusText(derivedSummary)}
            </div>
          )}
        >
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] p-4">
              <div className="mb-3 flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
                <HardDrive className="h-4 w-4 text-[var(--color-text-secondary)]" />
                设备状态
              </div>
              <div className="space-y-2 text-sm">
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">设备 ID</span>
                  <span className="max-w-[60%] truncate text-right text-[var(--color-text-primary)]">
                    {loading ? '-' : (derivedSummary.deviceId || '未上报')}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">设备状态</span>
                  <span className="text-[var(--color-text-primary)]">{loading ? '-' : (derivedSummary.deviceStatus || 'unknown')}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">活跃任务</span>
                  <span className="text-[var(--color-text-primary)]">{loading ? '-' : derivedSummary.activeRunCount}</span>
                </div>
              </div>
            </div>

            <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] p-4">
              <div className="mb-3 flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
                <Link2 className="h-4 w-4 text-[var(--color-text-secondary)]" />
                连接状态
              </div>
              <div className="space-y-3 text-sm">
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">Workspace Host</span>
                  <span className={`inline-flex items-center gap-1 ${derivedSummary.serverReachable ? 'text-emerald-600' : 'text-amber-600'}`}>
                    {derivedSummary.serverReachable ? <CheckCircle2 className="h-4 w-4" /> : <WifiOff className="h-4 w-4" />}
                    {loading ? '-' : (derivedSummary.serverReachable ? '已连接' : '不可达')}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">Ant Runtime</span>
                  <span className={`inline-flex items-center gap-1 ${derivedSummary.antRuntimeReachable ? 'text-emerald-600' : 'text-amber-600'}`}>
                    {derivedSummary.antRuntimeReachable ? <CheckCircle2 className="h-4 w-4" /> : <WifiOff className="h-4 w-4" />}
                    {loading ? '-' : (derivedSummary.antRuntimeReachable ? '已连通' : '未连通')}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">Session Ready</span>
                  <span className={`inline-flex items-center gap-1 ${derivedSummary.sessionReady ? 'text-emerald-600' : 'text-slate-500'}`}>
                    <CheckCircle2 className="h-4 w-4" />
                    {loading ? '-' : (derivedSummary.sessionReady ? 'ready' : 'not ready')}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </Card>

        <Card title="业务摘要" subtitle="控制台只保留可操作授权实例与入口。">
          <div className="space-y-4">
            <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] p-4">
              <div className="mb-3 flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
                <Store className="h-4 w-4 text-[var(--color-text-secondary)]" />
                授权实例概况
              </div>
              <div className="space-y-2 text-sm">
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">授权实例</span>
                  <span className="text-[var(--color-text-primary)]">{loading ? '-' : stats.totalAccounts}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">Ready 占比</span>
                  <span className="text-[var(--color-text-primary)]">{loading ? '-' : readyRate}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">待人工处理</span>
                  <span className="text-[var(--color-text-primary)]">{loading ? '-' : stats.manualAttentionCount}</span>
                </div>
              </div>
            </div>

            <div className="rounded-xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-card)] p-4">
              <div className="mb-3 flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
                <Monitor className="h-4 w-4 text-[var(--color-text-secondary)]" />
                实例列表入口
              </div>
              <p className="mb-4 text-sm text-[var(--color-text-muted)]">
                实例详情、配置、启动与批量操作仍留在原有实例列表页，本页只做导航。
              </p>
              <Link
                to="/browser/list"
                className="group inline-flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-2 text-sm font-medium text-[var(--color-text-primary)] transition-all hover:border-[var(--color-border-strong)] hover:bg-[var(--color-bg-muted)]"
              >
                去实例列表
                <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-0.5" />
              </Link>
            </div>
          </div>
        </Card>
      </div>
    </div>
  )
}
