import { Modal, Badge } from '../../../shared/components'
import { ShopInstanceStatusBadge } from './ShopInstanceStatusBadge'
import type { WorkspaceAuthorizedShop } from '../types'

interface ShopInstanceDrawerProps {
  open: boolean
  shop: WorkspaceAuthorizedShop | null
  onClose: () => void
}

function platformLabel(platformCode: string) {
  if (!platformCode) return '-'
  if (platformCode === 'alibaba') return '1688 / Alibaba'
  return platformCode
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-[var(--color-border-muted)] py-3 last:border-0">
      <span className="text-sm text-[var(--color-text-muted)]">{label}</span>
      <span className="max-w-[60%] break-all text-right text-sm text-[var(--color-text-primary)]">{value || '-'}</span>
    </div>
  )
}

export function ShopInstanceDrawer({ open, shop, onClose }: ShopInstanceDrawerProps) {
  if (!shop) return null

  return (
    <Modal open={open} onClose={onClose} title={shop.shopName || shop.shopId} width="760px">
      <div className="space-y-5">
        <div className="flex flex-wrap items-center gap-2">
          <ShopInstanceStatusBadge shop={shop} />
          <Badge variant="default">{platformLabel(shop.platformCode)}</Badge>
          {shop.instanceRunning ? <Badge variant="info">本机实例运行中</Badge> : <Badge variant="default">本机实例未运行</Badge>}
        </div>

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)] p-4">
            <h4 className="mb-2 text-sm font-semibold text-[var(--color-text-primary)]">店铺信息</h4>
            <DetailRow label="店铺名称" value={shop.shopName || '-'} />
            <DetailRow label="Shop ID" value={shop.shopId} />
            <DetailRow label="平台" value={platformLabel(shop.platformCode)} />
            <DetailRow label="共享登录状态" value={shop.sharedLoginStatusLabel || shop.sharedLoginStatus || '-'} />
          </div>

          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)] p-4">
            <h4 className="mb-2 text-sm font-semibold text-[var(--color-text-primary)]">本地实例映射</h4>
            <DetailRow label="Profile ID" value={shop.profileId || '-'} />
            <DetailRow label="Instance ID" value={shop.instanceId || '-'} />
            <DetailRow label="本机运行态" value={shop.instanceRunning ? 'running' : 'stopped'} />
            <DetailRow label="最近验证" value="暂无" />
            <DetailRow label="最近打开" value="暂无" />
          </div>
        </div>
      </div>
    </Modal>
  )
}
