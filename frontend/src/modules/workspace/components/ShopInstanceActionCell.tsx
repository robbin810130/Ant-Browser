import { ExternalLink, KeyRound, ShieldCheck, Sheet } from 'lucide-react'
import { Button } from '../../../shared/components'
import type { WorkspaceAuthorizedShop } from '../types'

interface ShopInstanceActionCellProps {
  shop: WorkspaceAuthorizedShop
  onOpen: () => void
  onBind: () => void
  onValidate: () => void
  onDetail: () => void
  compact?: boolean
}

export function ShopInstanceActionCell({
  shop,
  onOpen,
  onBind,
  onValidate,
  onDetail,
  compact = false,
}: ShopInstanceActionCellProps) {
  const size = compact ? 'sm' : 'sm'
  const wrapClass = compact ? 'flex-wrap' : 'justify-end'

  return (
    <div className={`flex gap-1.5 ${wrapClass}`}>
      <Button size={size} onClick={(event) => { event.stopPropagation(); onOpen() }}>
        <ExternalLink className="h-3.5 w-3.5" />
        一键打开后台
      </Button>
      <Button size={size} variant="secondary" onClick={(event) => { event.stopPropagation(); onBind() }}>
        <KeyRound className="h-3.5 w-3.5" />
        更新凭据
      </Button>
      <Button size={size} variant="secondary" onClick={(event) => { event.stopPropagation(); onValidate() }}>
        <ShieldCheck className="h-3.5 w-3.5" />
        本机验证
      </Button>
      <Button size={size} variant="ghost" onClick={(event) => { event.stopPropagation(); onDetail() }} title={`查看 ${shop.shopName || shop.shopId} 详情`}>
        <Sheet className="h-3.5 w-3.5" />
        详情
      </Button>
    </div>
  )
}
