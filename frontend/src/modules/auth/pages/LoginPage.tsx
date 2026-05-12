import { FormEvent, useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { Button, Card, Input, toast } from '../../../shared/components'
import {
  bootstrapDesktopAuthRuntime,
  fetchDesktopAuthProfile,
  loginDesktopUser,
  runDesktopAuthStrongCleanup,
  saveDesktopAuthSession,
} from '../api'
import { useAuthStore } from '../../../store/authStore'

type LoginLocationState = {
  from?: {
    pathname?: string
    search?: string
    hash?: string
  }
}

function resolveRedirectTarget(state: LoginLocationState | null | undefined): string {
  const pathname = state?.from?.pathname?.trim()
  if (!pathname || pathname === '/login') {
    return '/'
  }
  return `${pathname}${state?.from?.search || ''}${state?.from?.hash || ''}`
}

function getErrorMessage(error: unknown): string {
  const message = typeof error === 'object' && error && 'message' in error ? (error as { message?: unknown }).message : ''
  return typeof message === 'string' && message.trim() ? message.trim() : '登录失败，请检查账号密码后重试'
}

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const status = useAuthStore((state) => state.status)
  const bootstrapReady = useAuthStore((state) => state.bootstrapReady)
  const setAuthenticating = useAuthStore((state) => state.setAuthenticating)
  const setAuthenticated = useAuthStore((state) => state.setAuthenticated)
  const setAnonymous = useAuthStore((state) => state.setAnonymous)

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [rememberMe, setRememberMe] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')

  const redirectTarget = useMemo(
    () => resolveRedirectTarget(location.state as LoginLocationState | null | undefined),
    [location.state],
  )

  useEffect(() => {
    if (status === 'authenticated' && bootstrapReady) {
      navigate(redirectTarget, { replace: true })
    }
  }, [bootstrapReady, navigate, redirectTarget, status])

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()

    const normalizedUsername = username.trim()
    const normalizedPassword = password.trim()
    if (!normalizedUsername || !normalizedPassword) {
      const nextError = '请输入用户名和密码'
      setErrorMessage(nextError)
      toast.error(nextError)
      return
    }

    setSubmitting(true)
    setErrorMessage('')
    setAuthenticating()

    let persistedSessionSaved = false
    try {
      const accessToken = await loginDesktopUser(normalizedUsername, normalizedPassword)
      if (!accessToken) {
        throw new Error('登录成功但未返回 accessToken')
      }

      await saveDesktopAuthSession(accessToken, rememberMe)
      persistedSessionSaved = true
      const profile = await fetchDesktopAuthProfile(accessToken)
      await bootstrapDesktopAuthRuntime()

      setAuthenticated({
        accessToken,
        rememberMe,
        profile,
        bootstrapReady: true,
      })
      toast.success(`欢迎回来，${profile.user.displayName || profile.user.username}`)
      navigate(redirectTarget, { replace: true })
    } catch (error) {
      if (persistedSessionSaved) {
        try {
          await runDesktopAuthStrongCleanup('login_failed')
        } catch (cleanupError) {
          console.error('Desktop auth cleanup failed after login error', cleanupError)
        }
      }
      const nextError = getErrorMessage(error)
      setErrorMessage(nextError)
      setAnonymous()
      toast.error(nextError)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(245,158,11,0.16),_transparent_34%),linear-gradient(180deg,_#fff9f0_0%,_#f6f7fb_48%,_#eef2f7_100%)] px-4 py-10">
      <div className="mx-auto flex min-h-[calc(100vh-5rem)] max-w-5xl items-center justify-center">
        <div className="grid w-full gap-6 lg:grid-cols-[1.15fr_0.85fr]">
          <div className="rounded-[28px] border border-white/70 bg-white/65 p-8 shadow-[0_24px_80px_rgba(15,23,42,0.08)] backdrop-blur">
            <div className="inline-flex items-center rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-medium text-amber-700">
              1688 店铺工作台
            </div>
            <div className="mt-6 space-y-4">
              <h1 className="text-4xl font-semibold tracking-tight text-slate-900">桌面端登录</h1>
              <p className="max-w-xl text-sm leading-6 text-slate-600">
                登录后会通过 Wails 后端走 workspace server origin 完成鉴权，并显式启动 runtime bootstrap，避免“前端已登录、桌面 runtime 还没就绪”的假成功。
              </p>
            </div>
            <div className="mt-8 grid gap-4 sm:grid-cols-3">
              <div className="rounded-2xl border border-slate-200 bg-white/80 p-4">
                <div className="text-xs uppercase tracking-[0.18em] text-slate-400">Step 1</div>
                <div className="mt-2 text-sm font-medium text-slate-900">服务端登录</div>
                <p className="mt-1 text-xs leading-5 text-slate-500">账号密码只经由 Wails backend 转发到 workspace 服务端。</p>
              </div>
              <div className="rounded-2xl border border-slate-200 bg-white/80 p-4">
                <div className="text-xs uppercase tracking-[0.18em] text-slate-400">Step 2</div>
                <div className="mt-2 text-sm font-medium text-slate-900">会话恢复</div>
                <p className="mt-1 text-xs leading-5 text-slate-500">本地仅保存 accessToken 与 remember me 标志。</p>
              </div>
              <div className="rounded-2xl border border-slate-200 bg-white/80 p-4">
                <div className="text-xs uppercase tracking-[0.18em] text-slate-400">Step 3</div>
                <div className="mt-2 text-sm font-medium text-slate-900">Runtime 启动</div>
                <p className="mt-1 text-xs leading-5 text-slate-500">登录成功后立即 bootstrap agent，并显式读取授权店铺校验。</p>
              </div>
            </div>
          </div>

          <Card className="self-center shadow-[0_24px_64px_rgba(15,23,42,0.1)]" padding="lg">
            <form className="space-y-5" onSubmit={handleSubmit}>
              <div className="space-y-1">
                <h2 className="text-2xl font-semibold text-slate-900">登录账号</h2>
                <p className="text-sm text-slate-500">使用 workspace 账号进入桌面端。</p>
              </div>

              <div className="space-y-4">
                <div className="space-y-1.5">
                  <label htmlFor="desktop-login-username" className="text-sm font-medium text-slate-700">
                    用户名
                  </label>
                  <Input
                    id="desktop-login-username"
                    value={username}
                    onChange={(event) => setUsername(event.target.value)}
                    placeholder="请输入用户名"
                    autoComplete="username"
                    error={Boolean(errorMessage) && !username.trim()}
                    disabled={submitting}
                  />
                </div>

                <div className="space-y-1.5">
                  <label htmlFor="desktop-login-password" className="text-sm font-medium text-slate-700">
                    密码
                  </label>
                  <Input
                    id="desktop-login-password"
                    type="password"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    placeholder="请输入密码"
                    autoComplete="current-password"
                    error={Boolean(errorMessage) && !password.trim()}
                    disabled={submitting}
                  />
                </div>
              </div>

              <label className="flex items-center gap-2 text-sm text-slate-600">
                <input
                  type="checkbox"
                  checked={rememberMe}
                  onChange={(event) => setRememberMe(event.target.checked)}
                  disabled={submitting}
                  className="h-4 w-4 rounded border-slate-300 text-[var(--color-accent)] focus:ring-[var(--color-accent)]"
                />
                记住本次桌面端登录
              </label>

              {errorMessage ? (
                <div className="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                  {errorMessage}
                </div>
              ) : null}

              <Button type="submit" className="w-full" size="lg" loading={submitting}>
                登录并启动工作区
              </Button>
            </form>
          </Card>
        </div>
      </div>
    </div>
  )
}
