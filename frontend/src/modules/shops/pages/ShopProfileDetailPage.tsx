import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ArrowLeft, ExternalLink } from 'lucide-react'
import { Button, Card, Loading, toast } from '../../../shared/components'
import { fetchShopProfile } from '../api'
import type { ShopProfile } from '../types'

function DetailRow({ label, value }: { label: string; value: string }) {
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

export function ShopProfileDetailPage() {
  const { shopId = '' } = useParams()
  const [profile, setProfile] = useState<ShopProfile | null>(null)
  const [loading, setLoading] = useState(true)

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
          <DetailRow label="ASM 状态" value={profile.asmStatus} />
          <DetailRow label="授权状态" value={profile.authorizationStatus} />
          <DetailRow label="数据完整度" value={profile.dataCompleteness} />
          <DetailRow label="最近同步" value={profile.lastSyncedAt} />
          <DetailRow label="数据来源" value={profile.source} />
        </Card>
      </div>
    </div>
  )
}
