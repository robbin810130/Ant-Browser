import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { AlertCircle, CheckCircle2, Database, RefreshCw, ShieldCheck, Store } from 'lucide-react'
import { Badge, Button, Card, StatCard, Table, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import {
  asmStatusKind,
  asmStatusLabel,
  dataCompletenessLabel,
  deriveShopProfileStats,
  fetchShopProfiles,
  sourceLabel,
} from '../api'
import type { ShopProfile } from '../types'

function asmBadge(status: string) {
  const kind = asmStatusKind(status)
  if (kind === 'connected') return <Badge variant="success">{asmStatusLabel(status)}</Badge>
  if (kind === 'error') return <Badge variant="error">{asmStatusLabel(status)}</Badge>
  return <Badge variant="warning">{asmStatusLabel(status)}</Badge>
}

function dataCompletenessBadge(status: string) {
  if (status === 'complete') return <Badge variant="success">{dataCompletenessLabel(status)}</Badge>
  if (status === 'unknown') return <Badge variant="default">{dataCompletenessLabel(status)}</Badge>
  return <Badge variant="warning">{dataCompletenessLabel(status)}</Badge>
}

function authorizationBadge(profile: ShopProfile) {
  const label = profile.authorizationStatusLabel || profile.authorizationStatus || '-'
  if (profile.authorizationStatus === 'ready' || profile.authorizationStatus === 'valid') {
    return <Badge variant="success">{label}</Badge>
  }
  if (profile.authorizationStatus === 'disabled' || profile.authorizationStatus === 'revoked') {
    return <Badge variant="error">{label}</Badge>
  }
  if (profile.authorizationStatus) {
    return <Badge variant="warning">{label}</Badge>
  }
  return <Badge variant="default">未配置</Badge>
}

function platformLabel(platformCode: string) {
  if (!platformCode) return '-'
  if (platformCode === 'alibaba') return '1688 / Alibaba'
  return platformCode
}

function mutedText(value: string) {
  return (
    <span className="block max-w-[180px] truncate text-[var(--color-text-secondary)]" title={value || '-'}>
      {value || '-'}
    </span>
  )
}

export function ShopProfileListPage() {
  const [profiles, setProfiles] = useState<ShopProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const stats = useMemo(() => deriveShopProfileStats(profiles), [profiles])
  const usingMockProfiles = profiles.some((profile) => profile.source === 'dev_mock')

  async function load(silent = false) {
    if (silent) {
      setRefreshing(true)
    } else {
      setLoading(true)
    }

    try {
      setProfiles(await fetchShopProfiles())
    } catch (error) {
      console.error('load shop profiles failed', error)
      toast.error('加载店铺资料失败')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const columns: TableColumn<ShopProfile>[] = [
    {
      key: 'shopName',
      title: '店铺',
      width: 260,
      render: (_, record) => (
        <div className="flex min-w-0 flex-col gap-1">
          <Link
            className="max-w-[220px] truncate text-sm font-medium text-[var(--color-accent)] hover:underline"
            title={record.shopName || record.shopId}
            to={`/shops/${encodeURIComponent(record.shopId)}`}
          >
            {record.shopName || record.shopId}
          </Link>
          <span className="max-w-[220px] truncate text-xs text-[var(--color-text-muted)]" title={record.shopId}>
            {record.shopId}
          </span>
        </div>
      ),
    },
    {
      key: 'platformCode',
      title: '平台',
      width: 140,
      render: (value) => <Badge variant="default">{platformLabel(String(value || ''))}</Badge>,
    },
    {
      key: 'asmStatus',
      title: 'ASM 状态',
      width: 130,
      render: (value) => asmBadge(String(value || '')),
    },
    {
      key: 'authorizationStatus',
      title: '授权状态',
      width: 140,
      render: (_, record) => authorizationBadge(record),
    },
    {
      key: 'ownerName',
      title: '负责人',
      width: 130,
      render: (value) => mutedText(String(value || '')),
    },
    {
      key: 'mainCategory',
      title: '主营类目',
      width: 180,
      render: (value) => mutedText(String(value || '')),
    },
    {
      key: 'dataCompleteness',
      title: '资料完整度',
      width: 130,
      render: (value) => dataCompletenessBadge(String(value || '')),
    },
    {
      key: 'lastSyncedAt',
      title: '最近同步',
      width: 180,
      render: (value) => (
        <span className="block max-w-[160px] truncate text-xs text-[var(--color-text-muted)]" title={String(value || '-')}>
          {String(value || '-')}
        </span>
      ),
    },
    {
      key: 'source',
      title: '来源',
      width: 150,
      render: (value) => <Badge variant="default">{sourceLabel(String(value || ''))}</Badge>,
    },
  ]

  return (
    <div className="space-y-5 p-5 animate-fade-in">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h1 className="truncate text-xl font-semibold text-[var(--color-text-primary)]">店铺资料</h1>
          <p className="mt-1 text-sm text-[var(--color-text-muted)]">
            ASM 店铺业务主数据，聚合授权、接入状态与经营归属信息。
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
        <StatCard title="店铺总数" value={loading ? '-' : stats.total} icon={<Store className="h-5 w-5" />} />
        <StatCard title="ASM 已接入" value={loading ? '-' : stats.asmConnected} icon={<CheckCircle2 className="h-5 w-5" />} />
        <StatCard title="ASM 待接入" value={loading ? '-' : stats.unavailable} icon={<AlertCircle className="h-5 w-5" />} />
        <StatCard title="资料待完善" value={loading ? '-' : stats.incomplete} icon={<Database className="h-5 w-5" />} />
      </div>

      <Card padding="none">
        <div className="flex flex-col gap-2 border-b border-[var(--color-border-muted)] px-4 py-3 text-sm text-[var(--color-text-muted)] sm:flex-row sm:items-center sm:justify-between">
          <span>{usingMockProfiles ? '资料源：开发模拟资料' : '资料源：ASM 店铺资料'}</span>
          <span className="inline-flex items-center gap-1 text-[var(--color-text-secondary)]">
            <ShieldCheck className="h-4 w-4" />
            {usingMockProfiles ? '当前未连接 Wails 客户端后端' : '真实接入状态'}
          </span>
        </div>
        <Table
          columns={columns}
          data={profiles}
          rowKey="shopId"
          loading={loading}
          emptyText="暂无 ASM 店铺资料"
          maxHeight="calc(100vh - 340px)"
          className="min-w-0"
        />
      </Card>
    </div>
  )
}
