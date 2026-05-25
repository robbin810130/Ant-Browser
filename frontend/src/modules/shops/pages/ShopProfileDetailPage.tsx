import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ArrowLeft, ExternalLink } from 'lucide-react'
import { Alert, Badge, Button, Card, Loading, toast } from '../../../shared/components'
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

export function ShopProfileDetailPage() {
  const { shopId = '' } = useParams()
  const [profile, setProfile] = useState<ShopProfile | null>(null)
  const [loading, setLoading] = useState(true)
  const [errorMessage, setErrorMessage] = useState('')

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
        setErrorMessage('')
        const next = await fetchShopProfile(normalizedShopId)
        if (!cancelled) setProfile(next)
      } catch (error) {
        console.error('load shop profile failed', error)
        const message = error instanceof Error ? error.message : '加载店铺详情失败'
        toast.error(message)
        if (!cancelled) {
          setProfile(null)
          setErrorMessage(message)
        }
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
        {errorMessage ? (
          <Alert
            type="warning"
            title="店铺资料未连接真实客户端链路"
            message={errorMessage}
          />
        ) : (
          <p className="text-sm text-[var(--color-text-muted)]">店铺资料不存在</p>
        )}
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
          <DetailRow label="ASM Shop ID" value={profile.asmShopId} />
          <DetailRow label="Shop ID" value={profile.shopId} />
          <DetailRow label="店铺编码" value={profile.shopCode} />
          <DetailRow label="店铺别名" value={profile.shopAlias} />
          <DetailRow label="完整店铺名" value={profile.fullShopName} />
          <DetailRow label="平台" value={profile.platformName || platformLabel(profile.platformCode)} />
          <DetailRow label="平台子类型" value={profile.platformSubtype} />
          <DetailRow label="主营类目" value={profile.mainCategory} />
        </Card>
        <Card title="ASM 状态" subtitle="来自 ASM 店铺资料与本地授权状态">
          <DetailRow label="ASM 状态" value={asmBadge(profile.asmStatus)} />
          <DetailRow label="授权状态" value={authorizationBadge(profile)} />
          <DetailRow label="数据完整度" value={completenessBadge(profile.dataCompleteness)} />
          <DetailRow label="最近同步" value={profile.lastSyncedAt} />
          <DetailRow label="数据来源" value={sourceLabel(profile.source)} />
          <DetailRow label="高级会员" value={profile.advancedMemberName} />
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <Card title="经营归属" subtitle="ASM 运营负责人信息">
          <DetailRow label="负责人" value={profile.ownerName} />
          <DetailRow label="运营" value={profile.operatorName} />
          <DetailRow label="运营账号" value={profile.operatorUsername} />
          <DetailRow label="业务经理" value={profile.businessManagerName} />
          <DetailRow label="业务经理账号" value={profile.businessManagerUsername} />
          <DetailRow label="部门" value={profile.department} />
          <DetailRow label="分公司" value={profile.subCompanyName} />
        </Card>
        <Card title="联系与品牌" subtitle="ASM 店铺扩展资料">
          <DetailRow label="店铺地址" value={profile.shopUrl} />
          <DetailRow label="邮箱" value={profile.shopEmail} />
          <DetailRow label="电话" value={profile.shopPhone} />
          <DetailRow label="品牌" value={profile.brandName} />
        </Card>
      </div>
    </div>
  )
}
