import { useEffect, useRef, useState } from 'react'
import { Bell, Search, User, Settings, Check, Trash2, Info, AlertCircle, CheckCircle, ChevronDown } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import clsx from 'clsx'
import { runDesktopAuthStrongCleanup, type DesktopAuthStrongCleanupReason } from '../../modules/auth/api'
import { toast } from '../components'
import { useAuthStore } from '../../store/authStore'
import { useNotificationStore, type Notification } from '../../store/notificationStore'

function NotificationDropdown({
  notifications,
  onMarkAsRead,
  onMarkAllAsRead,
  onClear
}: {
  notifications: Notification[]
  onMarkAsRead: (id: string) => void
  onMarkAllAsRead: () => void
  onClear: () => void
}) {
  const unreadCount = notifications.filter(n => !n.read).length

  const getIcon = (type: Notification['type']) => {
    switch (type) {
      case 'success': return <CheckCircle className="w-4 h-4 text-[var(--color-success)]" />
      case 'warning': return <AlertCircle className="w-4 h-4 text-[var(--color-warning)]" />
      case 'error': return <AlertCircle className="w-4 h-4 text-[var(--color-error)]" />
      default: return <Info className="w-4 h-4 text-[var(--color-accent)]" />
    }
  }

  return (
    <div className="absolute right-0 top-full mt-2 w-80 bg-[var(--color-bg-surface)] border border-[var(--color-border-default)] rounded-xl shadow-xl overflow-hidden z-50 animate-fade-in">
      <div className="px-4 py-3 border-b border-[var(--color-border-muted)] flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold text-[var(--color-text-primary)]">通知</span>
          {unreadCount > 0 && (
            <span className="px-1.5 py-0.5 text-xs font-medium bg-[var(--color-accent)] text-white rounded-full">
              {unreadCount}
            </span>
          )}
        </div>
        <div className="flex items-center gap-1">
          {unreadCount > 0 && (
            <button
              onClick={onMarkAllAsRead}
              className="p-1.5 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-accent)] hover:bg-[var(--color-bg-muted)] rounded transition-colors"
              title="全部标为已读"
            >
              <Check className="w-3.5 h-3.5" />
            </button>
          )}
          <button
            onClick={onClear}
            className="p-1.5 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-error)] hover:bg-[var(--color-bg-muted)] rounded transition-colors"
            title="清空通知"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <div className="max-h-80 overflow-y-auto">
        {notifications.length === 0 ? (
          <div className="py-8 text-center text-[var(--color-text-muted)]">
            <Bell className="w-8 h-8 mx-auto mb-2 opacity-50" />
            <p className="text-sm">暂无通知</p>
          </div>
        ) : (
          notifications.map((notification) => (
            <div
              key={notification.id}
              onClick={() => onMarkAsRead(notification.id)}
              className={clsx(
                'px-4 py-3 border-b border-[var(--color-border-muted)] last:border-0 cursor-pointer transition-colors hover:bg-[var(--color-bg-muted)]',
                !notification.read && 'bg-[var(--color-accent)]/5'
              )}
            >
              <div className="flex gap-3">
                <div className="shrink-0 mt-0.5">
                  {getIcon(notification.type)}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-start justify-between gap-2">
                    <p className={clsx(
                      'text-sm truncate',
                      notification.read ? 'text-[var(--color-text-secondary)]' : 'text-[var(--color-text-primary)] font-medium'
                    )}>
                      {notification.title}
                    </p>
                    {!notification.read && (
                      <span className="w-2 h-2 rounded-full bg-[var(--color-accent)] shrink-0 mt-1.5" />
                    )}
                  </div>
                  <p className="text-xs text-[var(--color-text-muted)] mt-0.5 line-clamp-2">
                    {notification.message}
                  </p>
                  <p className="text-[10px] text-[var(--color-text-muted)] mt-1">
                    {notification.time}
                  </p>
                </div>
              </div>
            </div>
          ))
        )}
      </div>

      {notifications.length > 0 && (
        <div className="px-4 py-2 border-t border-[var(--color-border-muted)] bg-[var(--color-bg-muted)]/50">
          <button className="w-full text-xs text-center text-[var(--color-accent)] hover:underline">
            查看全部通知
          </button>
        </div>
      )}
    </div>
  )
}

function getErrorMessage(error: unknown): string {
  const message = typeof error === 'object' && error && 'message' in error ? (error as { message?: unknown }).message : ''
  return typeof message === 'string' && message.trim() ? message.trim() : '操作失败，请重试'
}

export function Topbar() {
  const navigate = useNavigate()
  const [showNotifications, setShowNotifications] = useState(false)
  const [showAccountMenu, setShowAccountMenu] = useState(false)
  const { notifications, markAsRead, markAllAsRead, clearNotifications } = useNotificationStore()
  const profile = useAuthStore((state) => state.profile)
  const signingOut = useAuthStore((state) => state.signingOut)
  const setAnonymous = useAuthStore((state) => state.setAnonymous)
  const setSigningOut = useAuthStore((state) => state.setSigningOut)
  const notificationRef = useRef<HTMLDivElement>(null)
  const accountMenuRef = useRef<HTMLDivElement>(null)

  const unreadCount = notifications.filter(n => !n.read).length
  const accountName = profile?.user.displayName || profile?.user.username || '账号'
  const username = profile?.user.username || 'unknown'
  const rolesLabel = profile?.roles.map((item) => item.name).filter(Boolean).join(' / ') || '未分配角色'
  const dataScopeLabel = profile?.dataScope || 'unknown'

  async function runDestructiveFlow(
    reason: DesktopAuthStrongCleanupReason,
    confirmationText: string,
    navigateState?: Record<string, string>,
  ) {
    if (signingOut) return
    if (!window.confirm(confirmationText)) return

    setShowAccountMenu(false)
    setSigningOut()

    try {
      await runDesktopAuthStrongCleanup(reason)
      setAnonymous()
      navigate('/login', { replace: true, state: navigateState })
    } catch (error) {
      setSigningOut(false)
      toast.error(getErrorMessage(error))
    }
  }

  async function handleLogout() {
    await runDestructiveFlow('logout', '注销将关闭当前所有 managed 店铺实例，并清理当前登录会话。确定继续吗？')
  }

  async function handleSwitchAccount() {
    await runDestructiveFlow(
      'switch_account',
      '切换账号将关闭当前所有 managed 店铺实例，并回到登录页。确定继续吗？',
      { reason: 'switch_account' },
    )
  }

  async function handleRebindDevice() {
    await runDestructiveFlow(
      'rebind_device',
      '重绑设备将关闭当前所有 managed 店铺实例，并清理当前设备绑定。确定继续吗？',
      { reason: 'rebind_device' },
    )
  }

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      const target = event.target as Node

      if (notificationRef.current && !notificationRef.current.contains(target)) {
        setShowNotifications(false)
      }

      if (accountMenuRef.current && !accountMenuRef.current.contains(target)) {
        setShowAccountMenu(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  useEffect(() => {
    const isDev = Boolean((window as Window & { __ANT_APP_BOOTED__?: boolean }).__ANT_APP_BOOTED__)
    if (!isDev) return

    const onKeyDown = async (event: KeyboardEvent) => {
      if (!(event.metaKey || event.ctrlKey) || !event.altKey || signingOut) return

      let reason: DesktopAuthStrongCleanupReason | null = null
      let navigateState: Record<string, string> | undefined
      switch (event.key) {
        case '1':
          reason = 'logout'
          break
        case '2':
          reason = 'switch_account'
          navigateState = { reason: 'switch_account' }
          break
        case '3':
          reason = 'rebind_device'
          navigateState = { reason: 'rebind_device' }
          break
        default:
          return
      }

      event.preventDefault()
      setShowAccountMenu(false)
      setSigningOut()

      try {
        await runDesktopAuthStrongCleanup(reason)
        setAnonymous()
        navigate('/login', { replace: true, state: navigateState })
      } catch (error) {
        setSigningOut(false)
        toast.error(getErrorMessage(error))
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [navigate, setAnonymous, setSigningOut, signingOut])

  return (
    <header className="h-14 bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)] px-4 flex items-center justify-between gap-4">
      <div className="w-64">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--color-text-muted)]" />
          <input
            type="text"
            placeholder="搜索..."
            className="w-full h-8 pl-9 pr-3 bg-[var(--color-bg-muted)] border border-transparent rounded-md text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] focus:outline-none focus:bg-[var(--color-bg-surface)] focus:border-[var(--color-border-strong)] transition-all duration-150"
          />
        </div>
      </div>

      <div className="flex-1" />

      <div className="flex items-center gap-1">
        <div className="relative" ref={notificationRef}>
          <button
            onClick={() => {
              setShowNotifications(!showNotifications)
              setShowAccountMenu(false)
            }}
            className={clsx(
              'relative w-8 h-8 flex items-center justify-center rounded-md transition-colors duration-150',
              showNotifications
                ? 'text-[var(--color-accent)] bg-[var(--color-accent-muted)]'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)]'
            )}
            title="通知"
          >
            <Bell className="w-4 h-4" />
            {unreadCount > 0 && (
              <span className="absolute -top-0.5 -right-0.5 w-4 h-4 text-[10px] font-medium bg-[var(--color-error)] text-white rounded-full flex items-center justify-center">
                {unreadCount > 9 ? '9+' : unreadCount}
              </span>
            )}
          </button>

          {showNotifications && (
            <NotificationDropdown
              notifications={notifications}
              onMarkAsRead={markAsRead}
              onMarkAllAsRead={markAllAsRead}
              onClear={() => {
                clearNotifications()
                setShowNotifications(false)
              }}
            />
          )}
        </div>

        <Link
          to="/settings"
          className="w-8 h-8 flex items-center justify-center text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] rounded-md transition-colors duration-150"
          title="设置"
        >
          <Settings className="w-4 h-4" />
        </Link>

        <div className="w-px h-5 bg-[var(--color-border-default)] mx-1.5" />

        <div className="relative" ref={accountMenuRef}>
          <button
            type="button"
            onClick={() => {
              if (signingOut) return
              setShowAccountMenu((value) => !value)
              setShowNotifications(false)
            }}
            className={clsx(
              'flex items-center gap-2 pl-1 pr-2.5 py-1 rounded-md transition-colors duration-150',
              signingOut ? 'cursor-not-allowed opacity-60' : 'hover:bg-[var(--color-accent-muted)]'
            )}
            aria-expanded={showAccountMenu}
            aria-haspopup="menu"
            disabled={signingOut}
          >
            <div className="w-7 h-7 bg-[var(--color-accent)] rounded-md flex items-center justify-center">
              <User className="w-3.5 h-3.5 text-[var(--color-text-inverse)]" />
            </div>
            <span className="max-w-32 truncate text-sm font-medium text-[var(--color-text-secondary)]">
              {accountName}
            </span>
            <ChevronDown className={clsx('w-3.5 h-3.5 text-[var(--color-text-muted)] transition-transform', showAccountMenu && 'rotate-180')} />
          </button>

          {showAccountMenu && (
            <div className="absolute right-0 top-full z-50 mt-2 w-72 overflow-hidden rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] shadow-xl animate-fade-in">
              <div className="border-b border-[var(--color-border-muted)] px-4 py-3">
                <p className="text-sm font-semibold text-[var(--color-text-primary)]">{accountName}</p>
                <p className="text-xs text-[var(--color-text-muted)]">{username}</p>
                <p className="mt-2 text-xs text-[var(--color-text-secondary)]">角色：{rolesLabel}</p>
                <p className="mt-1 text-xs text-[var(--color-text-secondary)]">数据范围：{dataScopeLabel}</p>
              </div>
              <div className="p-2">
                <button
                  type="button"
                  onClick={handleSwitchAccount}
                  className="w-full rounded-lg px-3 py-2 text-left text-sm text-[var(--color-text-primary)] hover:bg-[var(--color-bg-muted)] disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={signingOut}
                >
                  切换账号
                </button>
                <button
                  type="button"
                  onClick={handleRebindDevice}
                  className="w-full rounded-lg px-3 py-2 text-left text-sm text-[var(--color-text-primary)] hover:bg-[var(--color-bg-muted)] disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={signingOut}
                >
                  重绑设备
                </button>
                <button
                  type="button"
                  onClick={handleLogout}
                  className="w-full rounded-lg px-3 py-2 text-left text-sm text-[var(--color-error)] hover:bg-[var(--color-bg-muted)] disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={signingOut}
                >
                  注销
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </header>
  )
}
