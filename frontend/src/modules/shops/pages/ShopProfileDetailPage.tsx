import { ReactNode, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ExternalLink } from 'lucide-react'
import { Alert, Badge, Button, Card, Drawer, Loading, toast } from '../../../shared/components'
import { fetchShopProfile, sourceLabel } from '../api'
import {
  asmBadge,
  authorizationBadge,
  buildShopProfileDetailGroups,
  dataCompletenessBadge,
  platformLabel,
} from '../profilePresentation'
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

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={title}
      subtitle="ASM 店铺资料 / 客户端详情"
      footer={profile ? (
        <div className="flex flex-col gap-2 sm:flex-row sm:justify-end">
          <Link className="w-full sm:w-auto" to={`/workbench?shopId=${encodeURIComponent(profile.shopId)}`}>
            <Button size="sm" className="w-full sm:w-auto">
              <ExternalLink className="h-4 w-4" />
              去工作台
            </Button>
          </Link>
        </div>
      ) : null}
    >
      {loading ? (
        <div className="py-12">
          <Loading text="加载店铺资料..." />
        </div>
      ) : errorMessage ? (
        <Alert type="warning" title="店铺资料未连接真实客户端链路" message={errorMessage} />
      ) : profile ? (
        <div className="space-y-5">
          <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] p-4">
            <div className="min-w-0">
              <p className="break-all text-sm text-[var(--color-text-muted)]">
                {platformLabel(profile.platformCode)} · {profile.shopId}
              </p>
              <div className="mt-3 flex flex-wrap gap-2">
                {asmBadge(profile.asmStatus)}
                {authorizationBadge(profile)}
                {dataCompletenessBadge(profile.dataCompleteness)}
                <Badge variant="default">{sourceLabel(profile.source)}</Badge>
              </div>
            </div>
          </div>

          {buildShopProfileDetailGroups(profile).map((group) => (
            <Card key={group.title} title={group.title} subtitle={group.subtitle}>
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
