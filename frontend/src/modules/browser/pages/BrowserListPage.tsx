import { useEffect, useMemo, useState } from 'react'
import { ChevronRight, ChevronUp, LayoutGrid, List, RefreshCw, Search, Store } from 'lucide-react'
import { Badge, Button, Card, Input, StatCard, Table, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import {
  deriveWorkspaceDashboardStats,
  fetchWorkspaceAuthorizedShops,
} from '../../workspace/api'
import { ShopInstanceActionCell } from '../../workspace/components/ShopInstanceActionCell'
import { ShopInstanceDrawer } from '../../workspace/components/ShopInstanceDrawer'
import { ShopInstanceStatusBadge } from '../../workspace/components/ShopInstanceStatusBadge'
import type { WorkspaceAuthorizedShop } from '../../workspace/types'

type ViewMode = 'table' | 'card'
type StatusFilter = 'all' | 'ready' | 'attention' | 'running'

const PLACEHOLDER_TIME = '-'

function platformLabel(platformCode: string) {
  if (!platformCode) return '-'
  if (platformCode === 'alibaba') return '1688 / Alibaba'
  return platformCode
}

function lastValidationLabel() {
  return PLACEHOLDER_TIME
}

function lastOpenLabel() {
  return PLACEHOLDER_TIME
}

function actionPendingMessage(action: 'open' | 'bind' | 'validate', shop: WorkspaceAuthorizedShop) {
  if (action === 'open') {
    return `店铺 ${shop.shopName || shop.shopId} 的“一键打开后台”将在 Task 4 接入底层打开流程`
  }
  if (action === 'bind') {
    return `店铺 ${shop.shopName || shop.shopId} 的“更新凭据”将在后续任务接入共享登录绑定流程`
  }
  return `店铺 ${shop.shopName || shop.shopId} 的“本机验证”将在后续任务接入本机验证流程`
}

export function BrowserListPage() {
  const [shops, setShops] = useState<WorkspaceAuthorizedShop[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [headerCollapsed, setHeaderCollapsed] = useState(false)
  const [viewMode, setViewMode] = useState<ViewMode>('table')
  const [keyword, setKeyword] = useState('')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [selectedShop, setSelectedShop] = useState<WorkspaceAuthorizedShop | null>(null)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())

  async function load(silent = false) {
    if (silent) {
      setRefreshing(true)
    } else {
      setLoading(true)
    }

    try {
      const next = await fetchWorkspaceAuthorizedShops()
      setShops(next)
    } catch (error) {
      console.error('load workspace shops failed', error)
      toast.error('加载授权店铺列表失败')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const filteredShops = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase()

    return shops.filter((shop) => {
      const matchesKeyword = !normalizedKeyword
        || shop.shopName.toLowerCase().includes(normalizedKeyword)
        || shop.shopId.toLowerCase().includes(normalizedKeyword)
        || shop.platformCode.toLowerCase().includes(normalizedKeyword)

      if (!matchesKeyword) return false

      if (statusFilter === 'ready') return shop.sharedLoginStatus === 'ready'
      if (statusFilter === 'attention') return shop.sharedLoginStatus !== 'ready'
      if (statusFilter === 'running') return shop.instanceRunning
      return true
    }).sort((a, b) => a.shopName.localeCompare(b.shopName, 'zh-CN'))
  }, [keyword, shops, statusFilter])

  const stats = useMemo(() => deriveWorkspaceDashboardStats(shops), [shops])

  const toggleSelect = (shopId: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(shopId)) {
        next.delete(shopId)
      } else {
        next.add(shopId)
      }
      return next
    })
  }

  const handleSelectAll = () => {
    setSelectedIds(new Set(filteredShops.map((shop) => shop.shopId)))
  }

  const handleDeselectAll = () => {
    setSelectedIds(new Set())
  }

  const handlePlaceholderAction = (action: 'open' | 'bind' | 'validate', shop: WorkspaceAuthorizedShop) => {
    if (action === 'open') {
      toast.info(actionPendingMessage(action, shop))
      return
    }
    toast.warning(actionPendingMessage(action, shop))
  }

  const columns: TableColumn<WorkspaceAuthorizedShop>[] = [
    {
      key: 'selection',
      title: (
        <input
          type="checkbox"
          className="h-4 w-4 cursor-pointer rounded accent-[var(--color-accent)]"
          checked={filteredShops.length > 0 && selectedIds.size === filteredShops.length}
          ref={(input) => {
            if (input) {
              input.indeterminate = selectedIds.size > 0 && selectedIds.size < filteredShops.length
            }
          }}
          onChange={(event) => {
            if (event.target.checked) handleSelectAll()
            else handleDeselectAll()
          }}
        />
      ),
      width: 48,
      render: (_, record) => (
        <input
          type="checkbox"
          className="h-4 w-4 cursor-pointer rounded accent-[var(--color-accent)]"
          checked={selectedIds.has(record.shopId)}
          onChange={() => toggleSelect(record.shopId)}
          onClick={(event) => event.stopPropagation()}
        />
      ),
    },
    {
      key: 'shopName',
      title: '店铺名称',
      render: (value, record) => (
        <div className="flex flex-col gap-1">
          <button
            type="button"
            className="truncate text-left text-sm font-medium text-[var(--color-accent)] hover:underline"
            onClick={(event) => {
              event.stopPropagation()
              setSelectedShop(record)
            }}
          >
            {value || record.shopId}
          </button>
          <span className="text-xs text-[var(--color-text-muted)]">{record.shopId}</span>
        </div>
      ),
    },
    {
      key: 'platformCode',
      title: '平台',
      render: (value) => (
        <Badge variant="default">{platformLabel(String(value || ''))}</Badge>
      ),
    },
    {
      key: 'sharedLoginStatus',
      title: '当前状态',
      render: (_, record) => <ShopInstanceStatusBadge shop={record} />,
    },
    {
      key: 'lastValidation',
      title: '最近验证',
      render: () => <span className="text-xs text-[var(--color-text-muted)]">{lastValidationLabel()}</span>,
    },
    {
      key: 'lastOpen',
      title: '最近打开',
      render: () => <span className="text-xs text-[var(--color-text-muted)]">{lastOpenLabel()}</span>,
    },
    {
      key: 'actions',
      title: '操作',
      align: 'right',
      render: (_, record) => (
        <ShopInstanceActionCell
          shop={record}
          onOpen={() => handlePlaceholderAction('open', record)}
          onBind={() => handlePlaceholderAction('bind', record)}
          onValidate={() => handlePlaceholderAction('validate', record)}
          onDetail={() => setSelectedShop(record)}
        />
      ),
    },
  ]

  return (
    <div className="h-full animate-fade-in space-y-5 overflow-auto p-5">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">实例列表</h1>
          <p className="mt-1 text-sm text-[var(--color-text-muted)]">
            当前已授权店铺 {shops.length}
            {filteredShops.length !== shops.length && (
              <span className="ml-1 text-[var(--color-accent)]">（已筛选 {filteredShops.length}）</span>
            )}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="sm" onClick={() => setHeaderCollapsed((prev) => !prev)}>
            {headerCollapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronUp className="h-4 w-4" />}
            {headerCollapsed ? '展开面板' : '收起面板'}
          </Button>
          <Button variant="secondary" size="sm" onClick={() => void load(true)} loading={refreshing}>
            <RefreshCw className="h-4 w-4" />
            刷新
          </Button>
          <div className="ml-2 flex items-center rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] p-0.5">
            <button
              className={`rounded p-1.5 text-[var(--color-text-muted)] transition-colors hover:text-[var(--color-text-primary)] ${viewMode === 'card' ? 'bg-[var(--color-bg-surface)] text-[var(--color-accent)] shadow-sm' : ''}`}
              onClick={() => setViewMode('card')}
              title="卡片视图"
            >
              <LayoutGrid className="h-4 w-4" />
            </button>
            <button
              className={`rounded p-1.5 text-[var(--color-text-muted)] transition-colors hover:text-[var(--color-text-primary)] ${viewMode === 'table' ? 'bg-[var(--color-bg-surface)] text-[var(--color-accent)] shadow-sm' : ''}`}
              onClick={() => setViewMode('table')}
              title="表格视图"
            >
              <List className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>

      {!headerCollapsed && (
        <>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
            <StatCard title="授权店铺" value={loading ? '-' : stats.totalAccounts} icon={<Store className="h-5 w-5" />} />
            <StatCard title="Ready 店铺" value={loading ? '-' : stats.readyShopCount} icon={<Badge variant="success">R</Badge>} />
            <StatCard title="待人工处理" value={loading ? '-' : stats.manualAttentionCount} icon={<Badge variant="warning">!</Badge>} />
            <StatCard title="本机运行中" value={loading ? '-' : stats.runningInstanceCount} icon={<Badge variant="info">On</Badge>} />
          </div>

          <Card title="筛选" subtitle="沿用实例列表的操作骨架，但数据已经切到授权店铺视角。">
            <div className="grid grid-cols-1 gap-3 md:grid-cols-[1.4fr_0.8fr]">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-muted)]" />
                <Input
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  placeholder="搜索店铺名称 / shopId / 平台"
                  className="pl-9"
                />
              </div>
              <select
                value={statusFilter}
                onChange={(event) => setStatusFilter(event.target.value as StatusFilter)}
                className="h-10 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-input)] px-3 text-sm text-[var(--color-text-primary)] outline-none focus:border-[var(--color-accent)]"
              >
                <option value="all">全部状态</option>
                <option value="ready">仅 Ready</option>
                <option value="attention">待人工处理</option>
                <option value="running">本机运行中</option>
              </select>
            </div>
          </Card>
        </>
      )}

      {selectedIds.size > 0 ? (
        <div className="flex items-center gap-3 rounded-lg border border-[var(--color-accent)]/20 bg-[var(--color-accent)]/10 px-4 py-2.5">
          <span className="text-sm font-medium text-[var(--color-accent)]">
            已选 {selectedIds.size} / {filteredShops.length}
          </span>
          <div className="ml-auto flex gap-1.5">
            <Button size="sm" variant="ghost" onClick={handleSelectAll}>全选</Button>
            <Button size="sm" variant="ghost" onClick={handleDeselectAll}>取消</Button>
            <Button size="sm" onClick={() => toast.warning('批量打开后台将在后续任务接入真实流程')}>批量打开</Button>
          </div>
        </div>
      ) : null}

      <Card padding="none">
        <div className="overflow-auto" style={{ maxHeight: 'calc(100vh - 320px)' }}>
          {viewMode === 'table' ? (
            <Table
              columns={columns}
              data={filteredShops}
              rowKey="shopId"
              loading={loading}
              emptyText="暂无授权店铺，请先在 Workspace 侧完成授权同步"
              onRowClick={(record) => setSelectedShop(record)}
            />
          ) : loading ? (
            <div className="flex items-center justify-center py-16 text-sm text-[var(--color-text-muted)]">加载中...</div>
          ) : filteredShops.length === 0 ? (
            <div className="flex items-center justify-center py-16 text-sm text-[var(--color-text-muted)]">
              暂无授权店铺，请先在 Workspace 侧完成授权同步
            </div>
          ) : (
            <div className="grid min-h-[500px] grid-cols-1 content-start items-start gap-4 p-4 lg:grid-cols-2">
              {filteredShops.map((shop) => {
                const selected = selectedIds.has(shop.shopId)
                return (
                  <div
                    key={shop.shopId}
                    className={`flex h-[300px] flex-col rounded-xl border bg-[var(--color-bg-surface)] p-4 shadow-[0_1px_4px_rgba(0,0,0,0.08)] transition-all duration-200 ${selected ? 'border-[var(--color-accent)] ring-1 ring-[var(--color-accent)]/20' : 'border-[var(--color-border-default)] hover:border-[var(--color-accent)]'}`}
                  >
                    <div className="flex items-start justify-between gap-3 border-b border-[var(--color-border-muted)]/50 pb-3">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <input
                            type="checkbox"
                            className="h-4 w-4 cursor-pointer rounded accent-[var(--color-accent)]"
                            checked={selected}
                            onChange={() => toggleSelect(shop.shopId)}
                            onClick={(event) => event.stopPropagation()}
                          />
                          <button
                            type="button"
                            className="truncate text-left text-sm font-medium text-[var(--color-accent)] hover:underline"
                            onClick={() => setSelectedShop(shop)}
                          >
                            {shop.shopName || shop.shopId}
                          </button>
                        </div>
                        <p className="mt-1 text-xs text-[var(--color-text-muted)]">{shop.shopId}</p>
                      </div>
                      <ShopInstanceStatusBadge shop={shop} />
                    </div>

                    <div className="grid grid-cols-2 gap-4 py-4">
                      <div className="flex flex-col gap-1">
                        <span className="text-xs font-medium text-[var(--color-text-muted)]">平台</span>
                        <span className="text-sm text-[var(--color-text-primary)]">{platformLabel(shop.platformCode)}</span>
                      </div>
                      <div className="flex flex-col gap-1">
                        <span className="text-xs font-medium text-[var(--color-text-muted)]">Profile ID</span>
                        <span className="truncate text-sm text-[var(--color-text-primary)]">{shop.profileId || '-'}</span>
                      </div>
                      <div className="flex flex-col gap-1">
                        <span className="text-xs font-medium text-[var(--color-text-muted)]">最近验证</span>
                        <span className="text-sm text-[var(--color-text-primary)]">{lastValidationLabel()}</span>
                      </div>
                      <div className="flex flex-col gap-1">
                        <span className="text-xs font-medium text-[var(--color-text-muted)]">最近打开</span>
                        <span className="text-sm text-[var(--color-text-primary)]">{lastOpenLabel()}</span>
                      </div>
                    </div>

                    <div className="mt-auto border-t border-[var(--color-border-muted)]/50 pt-3">
                      <ShopInstanceActionCell
                        shop={shop}
                        onOpen={() => handlePlaceholderAction('open', shop)}
                        onBind={() => handlePlaceholderAction('bind', shop)}
                        onValidate={() => handlePlaceholderAction('validate', shop)}
                        onDetail={() => setSelectedShop(shop)}
                        compact
                      />
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </Card>

      <ShopInstanceDrawer
        open={Boolean(selectedShop)}
        shop={selectedShop}
        onClose={() => setSelectedShop(null)}
      />
    </div>
  )
}
