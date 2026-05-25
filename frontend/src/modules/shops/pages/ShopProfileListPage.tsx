import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { AlertCircle, CheckCircle2, Database, RefreshCw, ShieldCheck, Store } from 'lucide-react'
import { Alert, Button, Card, DataTable, StatCard, toast } from '../../../shared/components'
import {
  deriveShopProfileStats,
  fetchShopProfiles,
} from '../api'
import { buildShopProfileColumns } from '../profilePresentation'
import type { ShopProfile } from '../types'
import { ShopProfileDetailDrawer } from './ShopProfileDetailPage'

export function ShopProfileListPage() {
  const navigate = useNavigate()
  const { shopId = '' } = useParams()
  const [profiles, setProfiles] = useState<ShopProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')

  const stats = useMemo(() => deriveShopProfileStats(profiles), [profiles])
  const columns = useMemo(() => buildShopProfileColumns(), [])
  const usingMockProfiles = profiles.some((profile) => profile.source === 'dev_mock')
  const asmSnapshotMissing = !loading && !errorMessage && profiles.length === 0
  const selectedShopId = decodeURIComponent(shopId || '').trim()

  async function load(silent = false) {
    if (silent) {
      setRefreshing(true)
    } else {
      setLoading(true)
    }

    try {
      setErrorMessage('')
      setProfiles(await fetchShopProfiles())
    } catch (error) {
      console.error('load shop profiles failed', error)
      const message = error instanceof Error ? error.message : '加载店铺资料失败'
      setProfiles([])
      setErrorMessage(message)
      toast.error(message)
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const openProfile = (profile: ShopProfile) => {
    navigate(`/shops/${encodeURIComponent(profile.shopId)}`)
  }

  const closeDrawer = () => {
    navigate('/shops')
  }

  return (
    <div className="space-y-5 p-5 animate-fade-in">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h1 className="truncate text-xl font-semibold text-[var(--color-text-primary)]">店铺资料</h1>
          <p className="mt-1 text-sm text-[var(--color-text-muted)]">
            客户端查看 ASM 店铺主数据、经营归属与本地授权状态；同步与治理指标留在管理端。
          </p>
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => void load(true)}
          loading={refreshing}
          className="w-full shrink-0 sm:w-auto"
        >
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard title="店铺资料" value={loading ? '-' : stats.total} icon={<Store className="h-5 w-5" />} />
        <StatCard title="ASM 已接入" value={loading ? '-' : stats.asmConnected} icon={<CheckCircle2 className="h-5 w-5" />} />
        <StatCard title="ASM 待接入" value={loading ? '-' : stats.unavailable} icon={<AlertCircle className="h-5 w-5" />} />
        <StatCard title="资料待完善" value={loading ? '-' : stats.incomplete} icon={<Database className="h-5 w-5" />} />
      </div>

      <Card padding="none">
        <div className="flex flex-col gap-2 border-b border-[var(--color-border-muted)] px-4 py-3 text-sm text-[var(--color-text-muted)] sm:flex-row sm:items-center sm:justify-between">
          <span>
            {errorMessage
              ? '资料源：未连接'
              : asmSnapshotMissing
                ? '资料源：ASM 店铺资料未同步'
                : usingMockProfiles
                  ? '资料源：开发模拟资料'
                  : '资料源：ASM 店铺资料'}
          </span>
          <span className="inline-flex items-center gap-1 text-[var(--color-text-secondary)]">
            <ShieldCheck className="h-4 w-4" />
            {errorMessage
              ? '请在 Ant Browser 客户端本体中验证'
              : asmSnapshotMissing
                ? '等待 ASM 同步'
                : usingMockProfiles
                  ? '当前使用显式开发模拟模式'
                  : '真实接入状态'}
          </span>
        </div>
        {errorMessage ? (
          <div className="border-b border-[var(--color-border-muted)] p-4">
            <Alert
              type="warning"
              title="店铺资料未连接真实客户端链路"
              message={errorMessage}
            />
          </div>
        ) : asmSnapshotMissing ? (
          <div className="border-b border-[var(--color-border-muted)] p-4">
            <Alert
              type="warning"
              title="ASM 店铺资料尚未同步"
              message="当前客户端已连接真实 workspace，但服务端没有可用的 ASM 店铺快照。请先在管理端执行 ASM 店铺同步，完成后再刷新本页。"
            />
          </div>
        ) : null}
        <DataTable
          columns={columns}
          data={profiles}
          rowKey="shopId"
          loading={loading}
          emptyText={errorMessage ? '店铺资料未连接' : asmSnapshotMissing ? 'ASM 店铺资料尚未同步' : '暂无 ASM 店铺资料'}
          maxHeight="calc(100vh - 380px)"
          storageKey="client-shop-profile-table-columns"
          onRowClick={openProfile}
        />
      </Card>

      <ShopProfileDetailDrawer
        open={Boolean(selectedShopId)}
        shopId={selectedShopId}
        onClose={closeDrawer}
      />
    </div>
  )
}
