import { ReactNode, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ExternalLink } from 'lucide-react'
import clsx from 'clsx'
import { Alert, Badge, Button, Card, Drawer, Loading, toast } from '../../../shared/components'
import { fetchShopProfile, sourceLabel } from '../api'
import {
  asmBadge,
  authorizationBadge,
  buildShopProfileDetailGroups,
  formatProfileTime,
  platformLabel,
  shopProfileAction,
} from '../profilePresentation'
import type { ShopProfile } from '../types'

function isEmptyValue(value: ReactNode) {
  return value === null || value === undefined || value === ''
}

function DetailRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="grid gap-1 border-b border-[var(--color-border-muted)] py-3 last:border-0 sm:grid-cols-[140px_minmax(0,1fr)] sm:gap-4">
      <span className="text-sm text-[var(--color-text-muted)]">{label}</span>
      <span className="min-w-0 break-all text-sm text-[var(--color-text-primary)] sm:text-right">
        {isEmptyValue(value) ? '-' : value}
      </span>
    </div>
  )
}

function OverviewItem({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="min-w-0 rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-surface)] px-3 py-2">
      <p className="text-xs text-[var(--color-text-muted)]">{label}</p>
      <div className="mt-1 min-w-0 truncate text-sm font-medium text-[var(--color-text-primary)]" title={typeof value === 'string' ? value : undefined}>
        {isEmptyValue(value) ? '-' : value}
      </div>
    </div>
  )
}

interface ShopProfileDetailDrawerProps {
  shopId: string
  open: boolean
  onClose: () => void
}

export function ShopProfileDetailDrawer({ shopId, open, onClose }: ShopProfileDetailDrawerProps) {
  const [profile, setProfile] = useState<ShopProfile | null>(null)
  const [loading, setLoading] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    let cancelled = false
    const normalizedShopId = shopId.trim()

    if (!open || !normalizedShopId) {
      setProfile(null)
      setErrorMessage('')
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
  }, [open, shopId])

  const title = profile?.shopName || profile?.shopId || shopId
  const action = profile ? shopProfileAction(profile) : null

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={title}
      subtitle="ASM 店铺资料 / 客户端详情"
    >
      {loading ? (
        <div className="py-12">
          <Loading text="加载店铺资料..." />
        </div>
      ) : errorMessage ? (
        <Alert type="warning" title="店铺资料未连接真实客户端链路" message={errorMessage} />
      ) : profile ? (
        <div className="space-y-5">
          <section className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)] p-4">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <div className="min-w-0">
                <p className="break-all text-xs text-[var(--color-text-muted)]">
                  {profile.fullShopName || profile.shopId}
                </p>
                <div className="mt-3 flex flex-wrap gap-2">
                  {asmBadge(profile.asmStatus)}
                  {authorizationBadge(profile)}
                  <Badge variant="default">{sourceLabel(profile.source)}</Badge>
                </div>
              </div>
              <Link className="shrink-0" to={`/workbench?shopId=${encodeURIComponent(profile.shopId)}`} title={action?.description}>
                <Button size="sm" className="w-full sm:w-auto">
                  <ExternalLink className="h-4 w-4" />
                  {action?.label || '进入工作台'}
                </Button>
              </Link>
            </div>
            <div className="mt-4 grid gap-3 sm:grid-cols-2">
              <OverviewItem label="店铺编码" value={profile.shopCode} />
              <OverviewItem label="平台" value={profile.platformName || platformLabel(profile.platformCode)} />
              <OverviewItem label="主营类目" value={profile.mainCategory} />
              <OverviewItem label="最近同步" value={formatProfileTime(profile.lastSyncedAt)} />
            </div>
          </section>

          {buildShopProfileDetailGroups(profile).map((group) => (
            <Card
              key={group.title}
              title={group.title}
              subtitle={group.subtitle}
              className={clsx(group.tone === 'muted' && 'bg-[var(--color-bg-subtle)]')}
            >
              {group.fields.map((field) => (
                <DetailRow key={field.label} label={field.label} value={field.value} />
              ))}
            </Card>
          ))}
        </div>
      ) : (
        <p className="text-sm text-[var(--color-text-muted)]">店铺资料不存在</p>
      )}
    </Drawer>
  )
}

export function ShopProfileDetailPage() {
  const { shopId = '' } = useParams()
  const navigate = useNavigate()
  return (
    <ShopProfileDetailDrawer
      shopId={shopId}
      open={Boolean(shopId)}
      onClose={() => navigate('/shops')}
    />
  )
}
