import { ReactNode, useEffect } from 'react'
import { X } from 'lucide-react'
import { Button } from './Button'

interface DrawerProps {
  open: boolean
  title?: ReactNode
  subtitle?: ReactNode
  width?: string
  children: ReactNode
  footer?: ReactNode
  onClose: () => void
}

export function Drawer({
  open,
  title,
  subtitle,
  width = '860px',
  children,
  footer,
  onClose,
}: DrawerProps) {
  useEffect(() => {
    if (!open) return

    const previousOverflow = document.body.style.overflow
    const previousPaddingRight = document.body.style.paddingRight
    const scrollbarWidth = window.innerWidth - document.documentElement.clientWidth

    document.body.style.overflow = 'hidden'
    if (scrollbarWidth > 0) {
      document.body.style.paddingRight = `${scrollbarWidth}px`
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        onClose()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => {
      window.removeEventListener('keydown', handleKeyDown)
      document.body.style.overflow = previousOverflow
      document.body.style.paddingRight = previousPaddingRight
    }
  }, [onClose, open])

  if (!open) return null

  return (
    <div className="client-drawer-root" role="presentation">
      <button className="client-drawer-backdrop" aria-label="关闭抽屉" type="button" onClick={onClose} />
      <section
        className="client-drawer-panel"
        role="dialog"
        aria-modal="true"
        aria-label={typeof title === 'string' ? title : '详情抽屉'}
        style={{ width }}
      >
        <header className="client-drawer-header">
          <div className="min-w-0">
            {subtitle ? <p className="client-drawer-subtitle">{subtitle}</p> : null}
            {title ? <h2 className="client-drawer-title">{title}</h2> : null}
          </div>
          <Button variant="ghost" size="sm" onClick={onClose} title="关闭">
            <X className="h-4 w-4" />
            关闭
          </Button>
        </header>
        <div className="client-drawer-body">{children}</div>
        {footer ? <footer className="client-drawer-footer">{footer}</footer> : null}
      </section>
    </div>
  )
}
