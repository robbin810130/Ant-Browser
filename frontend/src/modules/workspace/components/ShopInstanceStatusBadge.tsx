import { Badge } from '../../../shared/components'
import type { WorkspaceAuthorizedShop } from '../types'

function resolveStatus(shop: WorkspaceAuthorizedShop) {
  if (shop.instanceRunning && shop.sharedLoginStatus === 'ready') {
    return { variant: 'success' as const, label: '已就绪 / 运行中' }
  }
  if (shop.sharedLoginStatus === 'ready') {
    return { variant: 'info' as const, label: '已就绪' }
  }
  if (shop.instanceRunning) {
    return { variant: 'warning' as const, label: '运行中待处理' }
  }
  return {
    variant: 'warning' as const,
    label: shop.sharedLoginStatusLabel || shop.sharedLoginStatus || '待人工处理',
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
