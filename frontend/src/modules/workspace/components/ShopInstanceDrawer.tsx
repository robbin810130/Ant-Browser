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

function localRuntimeSummary(shop: WorkspaceAuthorizedShop) {
  if (shop.reclaimPending) return 'pending_reclaim'
  if (!shop.profileExists) return 'profile_missing'
  return shop.instanceRunning ? 'running' : 'stopped'
}

function localRuntimeHint(shop: WorkspaceAuthorizedShop) {
  if (shop.reclaimPending) return '服务端授权已失效，本地实例等待回收'
  if (!shop.profileExists) return '尚未建立本地 managed profile 映射'
  if (!shop.coreReady) return '本地 profile 已存在，但缺少可用指纹内核'
  if (shop.instanceRunning) return '本地 managed 实例已运行，可直接复用'
  return '本地 profile 已就绪，可按需冷启动'
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
          {shop.profileExists ? <Badge variant="default">Profile 已映射</Badge> : <Badge variant="warning">Profile 待创建</Badge>}
          {shop.coreReady ? <Badge variant="success">指纹内核就绪</Badge> : <Badge variant="warning">指纹内核缺失</Badge>}
          {shop.reclaimPending ? <Badge variant="warning">待回收</Badge> : null}
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
            <DetailRow label="本机运行态" value={localRuntimeSummary(shop)} />
            <DetailRow label="本地映射" value={shop.profileExists ? 'present' : 'missing'} />
            <DetailRow label="指纹内核" value={shop.coreReady ? 'ready' : 'missing'} />
            <DetailRow label="回收状态" value={shop.reclaimPending ? 'pending' : 'active'} />
            <DetailRow label="运行态说明" value={localRuntimeHint(shop)} />
            <DetailRow label="最近验证" value="暂无" />
            <DetailRow label="最近打开" value="暂无" />
          </div>
        </div>
      </div>
    </Modal>
  )
}
