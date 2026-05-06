import { Badge } from '../../../shared/components'
import type { WorkspaceAuthorizedShop } from '../types'

function resolveStatus(shop: WorkspaceAuthorizedShop) {
  const businessReason = shop.sharedLoginStatusLabel || shop.sharedLoginStatus || ''

  if (shop.instanceRunning && shop.sharedLoginStatus === 'ready') {
    return { variant: 'success' as const, label: '已就绪 / 运行中' }
  }
  if (shop.sharedLoginStatus === 'ready') {
    return { variant: 'info' as const, label: '已就绪' }
  }

  const label = businessReason || '待人工处理'
  return {
    variant: 'warning' as const,
    label: shop.instanceRunning ? `${label} / 本机运行中` : label,
  }
}

export function ShopInstanceStatusBadge({ shop }: { shop: WorkspaceAuthorizedShop }) {
  const status = resolveStatus(shop)
  return (
    <Badge variant={status.variant} dot>
      {status.label}
    </Badge>
  )
}
