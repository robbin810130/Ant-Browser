import { Suspense, lazy, useEffect, useState } from 'react'
import type { ComponentType } from 'react'
import { BrowserRouter as Router, Routes, Route, Navigate, Outlet } from 'react-router-dom'
import { ThemeProvider } from './shared/theme'
import { Layout } from './shared/layout'
import { ToastContainer, Modal, Button, Loading } from './shared/components'
import { RequireAuth } from './shared/auth/RequireAuth'
import { AlertCircle } from 'lucide-react'
import {
  bootstrapDesktopAuthRuntime,
  fetchDesktopAuthProfile,
  loadDesktopAuthSession,
  runDesktopAuthStrongCleanup,
} from './modules/auth/api'
import type { DesktopAuthStrongCleanupReason } from './modules/auth/api'
import { EnvironmentGatePage } from './modules/runtime/pages/EnvironmentGatePage'
import { UpdatePromptModal } from './modules/runtime/components/UpdatePromptModal'
import { useNotificationStore } from './store/notificationStore'
import { useAuthStore } from './store/authStore'
import { useBackupStore } from './store/backupStore'
import { useRuntimeStore } from './store/runtimeStore'
import { ForceQuit as ForceQuitApp, QuitAppOnly as QuitAppOnlyApp } from './wailsjs/go/main/App'
import { Environment, Quit, WindowHide, WindowMinimise } from './wailsjs/runtime/runtime'

function lazyNamed<TModule extends Record<string, ComponentType<any>>>(
  loader: () => Promise<TModule>,
  exportName: keyof TModule,
) {
  return lazy(async () => {
    const module = await loader()
    return {
      default: module[exportName] as ComponentType<any>,
    }
  })
}

const WorkspaceDashboardPage = lazyNamed(() => import('./modules/workspace/pages/WorkspaceDashboardPage'), 'WorkspaceDashboardPage')
const SettingsPage = lazyNamed(() => import('./modules/settings/SettingsPage'), 'SettingsPage')
const ProfilePage = lazyNamed(() => import('./modules/profile/ProfilePage'), 'ProfilePage')
const AdminKeygenPage = lazyNamed(() => import('./modules/profile/AdminKeygenPage'), 'AdminKeygenPage')
const ChartsPage = lazyNamed(() => import('./modules/charts/ChartsPage'), 'ChartsPage')
const BrowserListPage = lazyNamed(() => import('./modules/browser/pages/BrowserListPage'), 'BrowserListPage')
const BrowserDetailPage = lazyNamed(() => import('./modules/browser/pages/BrowserDetailPage'), 'BrowserDetailPage')
const BrowserEditPage = lazyNamed(() => import('./modules/browser/pages/BrowserEditPage'), 'BrowserEditPage')
const BrowserCopyPage = lazyNamed(() => import('./modules/browser/pages/BrowserCopyPage'), 'BrowserCopyPage')
const BrowserLogsPage = lazyNamed(() => import('./modules/browser/pages/BrowserLogsPage'), 'BrowserLogsPage')
const ProxyPoolPage = lazyNamed(() => import('./modules/browser/pages/ProxyPoolPage'), 'ProxyPoolPage')
const CoreManagementPage = lazyNamed(() => import('./modules/browser/pages/CoreManagementPage'), 'CoreManagementPage')
const BookmarkSettingsPage = lazyNamed(() => import('./modules/browser/pages/BookmarkSettingsPage'), 'BookmarkSettingsPage')
const LaunchApiDocsPage = lazyNamed(() => import('./modules/browser/pages/LaunchApiDocsPage'), 'LaunchApiDocsPage')
const TagManagementPage = lazyNamed(() => import('./modules/browser/pages/TagManagementPage'), 'TagManagementPage')
const AutomationPage = lazyNamed(() => import('./modules/browser/pages/AutomationPage'), 'AutomationPage')
const UsageTutorialPage = lazyNamed(() => import('./modules/browser/pages/UsageTutorialPage'), 'UsageTutorialPage')
const QuickLaunchModal = lazyNamed(() => import('./modules/browser/components/QuickLaunchModal'), 'QuickLaunchModal')
const LoginPage = lazyNamed(() => import('./modules/auth/pages/LoginPage'), 'LoginPage')

const ProtectedAppShell = () => (
  <Layout>
    <Outlet />
  </Layout>
)

function useWailsNotifications() {
  const addNotification = useNotificationStore((s) => s.addNotification)

  useEffect(() => {
    const runtime = (window as any).runtime
    if (!runtime?.EventsOn) return

    const offCrashed = runtime.EventsOn(
      'browser:instance:crashed',
      (data: { profileId: string; profileName: string; error: string }) => {
        addNotification({
          type: 'error',
          title: '实例异常退出',
          message: `「${data.profileName || data.profileId}」意外崩溃：${data.error}`,
        })
      }
    )

    const offBridgeFailed = runtime.EventsOn(
      'proxy:bridge:failed',
      (data: { profileId: string; profileName: string; error: string }) => {
        addNotification({
          type: 'error',
          title: '代理连接失败',
          message: `「${data.profileName || data.profileId}」代理桥接启动失败：${data.error}`,
        })
      }
    )

    const offBridgeDied = runtime.EventsOn(
      'proxy:bridge:died',
      (data: { key: string; error: string }) => {
        addNotification({
          type: 'warning',
          title: '连接池节点失效',
          message: `代理节点 ${data.key} 连接中断，相关实例可能无法访问网络`,
        })
      }
    )

    return () => {
      offCrashed?.()
      offBridgeFailed?.()
      offBridgeDied?.()
    }
  }, [addNotification])
}

function CloseConfirmModal() {
  const [open, setOpen] = useState(false)
  const [platform, setPlatform] = useState('windows')
  const [quittingAction, setQuittingAction] = useState<'app-only' | 'app-and-browser' | null>(null)
  const importInProgress = useBackupStore((s) => s.importInProgress)
  const importProgress = useBackupStore((s) => s.importProgress)
  const importMessage = useBackupStore((s) => s.importMessage)
  const supportsTray = platform === 'windows'
  const quitting = quittingAction !== null

  useEffect(() => {
    const runtime = (window as any).runtime
    if (!runtime?.EventsOn) return

    const off = runtime.EventsOn('app:request-close', () => {
      setQuittingAction(null)
      setOpen(true)
    })
    return () => {
      if (typeof off === 'function') off()
    }
  }, [])

  useEffect(() => {
    let cancelled = false

    Environment()
      .then((info) => {
        if (!cancelled && info?.platform) {
          setPlatform(info.platform)
        }
      })
      .catch(() => {})

    return () => {
      cancelled = true
    }
  }, [])

  const closeModal = () => {
    if (quitting) return
    setOpen(false)
  }

  const handleMinimize = () => {
    if (quitting) return
    setOpen(false)
    if (supportsTray) {
      WindowHide()
      return
    }
    WindowMinimise()
  }

  const handleQuitAppOnly = async () => {
    setQuittingAction('app-only')
    try {
      await QuitAppOnlyApp()
    } catch (error) {
      console.error('QuitAppOnly failed', error)
      setQuittingAction(null)
    }
  }

  const handleQuitAppAndBrowsers = async () => {
    setQuittingAction('app-and-browser')
    try {
      await Promise.race([
        ForceQuitApp(),
        new Promise((resolve) => setTimeout(resolve, 1200)),
      ])
    } catch (error) {
      console.error('ForceQuit failed, falling back to runtime.Quit()', error)
    }
    Quit()
  }

  return (
    <Modal
      open={open}
      onClose={closeModal}
      title={importInProgress ? '关闭应用确认' : undefined}
      width={importInProgress ? '360px' : '420px'}
      closable={!quitting}
    >
      <div className="flex flex-col items-center pt-2 pb-6 px-4">
        <div className={`w-12 h-12 rounded-full flex items-center justify-center mb-4 ${
          importInProgress ? 'bg-amber-50 text-amber-500' : 'bg-red-50 text-red-500'
        }`}>
          <AlertCircle className="w-6 h-6" />
        </div>
        {importInProgress && (
          <h3 className="text-lg font-medium text-[var(--color-text-primary)] mb-2">
            正在加载中，是否关闭？
          </h3>
        )}
        {importInProgress ? (
          <p className="text-sm text-[var(--color-text-secondary)] text-center mb-6">
            当前正在加载配置
            {importProgress > 0 ? `（${importProgress}%）` : ''}。
            <br />
            {importMessage || '强制关闭会中断本次加载，是否仍要关闭应用？'}
          </p>
        ) : (
          <p className="mb-6 text-sm text-center text-[var(--color-text-secondary)]">
            可仅退出应用，或连同浏览器一起关闭。
          </p>
        )}

        <div className={`w-full ${importInProgress ? 'flex gap-3' : 'flex flex-col gap-2'}`}>
          {importInProgress ? (
            <>
              <Button variant="secondary" className="flex-1" onClick={closeModal} disabled={quitting}>
                继续加载
              </Button>
              <Button
                variant="danger"
                className="flex-1"
                onClick={handleQuitAppAndBrowsers}
                loading={quittingAction === 'app-and-browser'}
              >
                仍要关闭
              </Button>
            </>
          ) : (
            <>
              <Button
                variant="secondary"
                className="w-full !bg-[#f3f4f6] !border-[#e5e7eb] !text-[var(--color-text-primary)] hover:!bg-[#e5e7eb]"
                onClick={supportsTray ? handleMinimize : closeModal}
                disabled={quitting}
              >
                {supportsTray ? '最小化到托盘' : '取消'}
              </Button>
              <Button
                className="w-full"
                onClick={handleQuitAppOnly}
                loading={quittingAction === 'app-only'}
                disabled={quitting}
              >
                仅退出应用
              </Button>
              <Button
                variant="danger"
                className="w-full"
                onClick={handleQuitAppAndBrowsers}
                loading={quittingAction === 'app-and-browser'}
                disabled={quitting}
              >
                退出应用与浏览器
              </Button>
            </>
          )}
        </div>
      </div>
    </Modal>
  )
}

function DevDesktopAuthCleanupPanel() {
  const isDev = Boolean((window as Window & { __ANT_APP_BOOTED__?: boolean }).__ANT_APP_BOOTED__)
  const signingOut = useAuthStore((state) => state.signingOut)
  const setAnonymous = useAuthStore((state) => state.setAnonymous)
  const setSigningOut = useAuthStore((state) => state.setSigningOut)

  if (!isDev) {
    return null
  }

  async function run(reason: DesktopAuthStrongCleanupReason) {
    if (signingOut) return

    setSigningOut()
    try {
      await runDesktopAuthStrongCleanup(reason)
    } finally {
      setAnonymous()
      window.location.replace(`/login${reason === 'logout' ? '' : `?reason=${encodeURIComponent(reason)}`}`)
    }
  }

  return (
    <div className="fixed bottom-4 left-4 z-[100] rounded-2xl border border-amber-200 bg-white/95 p-3 shadow-[0_20px_48px_rgba(15,23,42,0.14)] backdrop-blur">
      <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-amber-700">Dev Cleanup</div>
      <div className="flex gap-2">
        <Button size="sm" variant="danger" onClick={() => void run('logout')} disabled={signingOut}>
          注销
        </Button>
        <Button size="sm" variant="secondary" onClick={() => void run('switch_account')} disabled={signingOut}>
          切换账号
        </Button>
        <Button size="sm" variant="secondary" onClick={() => void run('rebind_device')} disabled={signingOut}>
          重绑设备
        </Button>
      </div>
    </div>
  )
}

function App() {
  useWailsNotifications()
  const [quickLaunchOpen, setQuickLaunchOpen] = useState(false)
  const [authRecoveryComplete, setAuthRecoveryComplete] = useState(false)
  const setAuthenticating = useAuthStore((state) => state.setAuthenticating)
  const setAuthenticated = useAuthStore((state) => state.setAuthenticated)
  const setAnonymous = useAuthStore((state) => state.setAnonymous)
  const runtimeBootstrapped = useRuntimeStore((state) => state.bootstrapped)
  const environmentReady = useRuntimeStore((state) => state.environmentReady)
  const bootstrapRuntime = useRuntimeStore((state) => state.bootstrap)
  const routeFallback = (
    <div className="flex min-h-[240px] items-center justify-center py-10">
      <Loading text="页面加载中..." />
    </div>
  )

  useEffect(() => {
    void bootstrapRuntime()
  }, [bootstrapRuntime])

  useEffect(() => {
    if (!runtimeBootstrapped || !environmentReady) {
      return
    }

    let cancelled = false

    const recoverDesktopSession = async () => {
      setAuthenticating()

      try {
        const session = await loadDesktopAuthSession()
        const accessToken = session.accessToken.trim()

        if (!accessToken) {
          if (!cancelled) {
            setAnonymous()
          }
          return
        }

        const profile = await fetchDesktopAuthProfile(accessToken)
        await bootstrapDesktopAuthRuntime()

        if (!cancelled) {
          setAuthenticated({
            accessToken,
            rememberMe: session.rememberMe,
            profile,
            bootstrapReady: true,
          })
        }
      } catch (error) {
        try {
          await runDesktopAuthStrongCleanup('session_expired')
        } catch (cleanupError) {
          console.error('Desktop auth cleanup failed', cleanupError)
        }

        if (!cancelled) {
          setAnonymous()
        }
        console.error('Desktop auth session recovery failed', error)
      } finally {
        if (!cancelled) {
          setAuthRecoveryComplete(true)
        }
      }
    }

    void recoverDesktopSession()

    return () => {
      cancelled = true
    }
  }, [environmentReady, runtimeBootstrapped, setAnonymous, setAuthenticated, setAuthenticating])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.isComposing) return
      if (!(event.ctrlKey || event.metaKey)) return
      if (event.key.toLowerCase() !== 'k') return
      event.preventDefault()
      setQuickLaunchOpen((prev) => !prev)
    }

    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [])

  useEffect(() => {
    const isDev = Boolean((window as Window & { __ANT_APP_BOOTED__?: boolean }).__ANT_APP_BOOTED__)
    if (!isDev) return

    const params = new URLSearchParams(window.location.search)
    const reason = params.get('desktopAuthCleanup')
    const allowedReasons = new Set<DesktopAuthStrongCleanupReason>(['logout', 'switch_account', 'rebind_device'])
    if (!reason || !allowedReasons.has(reason as DesktopAuthStrongCleanupReason)) return

    const cleanupReason = reason as DesktopAuthStrongCleanupReason

    const run = async () => {
      try {
        await runDesktopAuthStrongCleanup(cleanupReason)
      } finally {
        setAnonymous()
        const nextURL = new URL(window.location.href)
        nextURL.searchParams.delete('desktopAuthCleanup')
        window.history.replaceState({}, '', `${nextURL.pathname}${nextURL.search}${nextURL.hash}`)
        window.location.replace(`/login${cleanupReason === 'logout' ? '' : `?reason=${encodeURIComponent(cleanupReason)}`}`)
      }
    }

    void run()
  }, [setAnonymous])

  if (!runtimeBootstrapped || !environmentReady) {
    return (
      <ThemeProvider>
        <EnvironmentGatePage />
        <UpdatePromptModal />
        <ToastContainer />
      </ThemeProvider>
    )
  }

  if (!authRecoveryComplete) {
    return (
      <ThemeProvider>
        <div className="flex min-h-screen items-center justify-center px-6">
          <Loading text="正在恢复登录状态..." />
        </div>
        <UpdatePromptModal />
        <ToastContainer />
      </ThemeProvider>
    )
  }

  return (
    <ThemeProvider>
      <Router>
        <Suspense fallback={routeFallback}>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route element={<RequireAuth />}>
              <Route element={<ProtectedAppShell />}>
                <Route path="/" element={<WorkspaceDashboardPage />} />
                <Route path="/charts" element={<ChartsPage />} />
                <Route path="/settings" element={<SettingsPage />} />
                <Route path="/profile" element={<ProfilePage />} />
                <Route path="/admin/keygen" element={<AdminKeygenPage />} />
                <Route path="/browser/list" element={<BrowserListPage />} />
                <Route path="/browser/detail/:id" element={<BrowserDetailPage />} />
                <Route path="/browser/edit/:id" element={<BrowserEditPage />} />
                <Route path="/browser/copy/:id" element={<BrowserCopyPage />} />
                <Route path="/browser/monitor" element={<Navigate to="/browser/list" replace />} />
                <Route path="/browser/logs" element={<BrowserLogsPage />} />
                <Route path="/browser/proxy-pool" element={<ProxyPoolPage />} />
                <Route path="/browser/cores" element={<CoreManagementPage />} />
                <Route path="/browser/bookmarks" element={<BookmarkSettingsPage />} />
                <Route path="/browser/automation" element={<AutomationPage />} />
                <Route path="/browser/launch-api" element={<LaunchApiDocsPage />} />
                <Route path="/browser/tags" element={<TagManagementPage />} />
                <Route path="/system/tutorial" element={<UsageTutorialPage />} />
              </Route>
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Suspense>
        <UpdatePromptModal />
        <ToastContainer />
        <CloseConfirmModal />
        <DevDesktopAuthCleanupPanel />
        <Suspense fallback={null}>
          {quickLaunchOpen ? (
            <QuickLaunchModal open={quickLaunchOpen} onClose={() => setQuickLaunchOpen(false)} />
          ) : null}
        </Suspense>
      </Router>
    </ThemeProvider>
  )
}

export default App
