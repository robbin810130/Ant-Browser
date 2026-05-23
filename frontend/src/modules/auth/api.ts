import {
  BootstrapDesktopAuthRuntime,
  ClearDesktopAuthSession,
  DesktopAuthStrongCleanup,
  FetchDesktopAuthProfile,
  LoadDesktopAuthSession,
  LoginDesktopUser,
  SaveDesktopAuthSession,
} from '../../wailsjs/go/main/App'
import type { DesktopAuthProfile, DesktopAuthSession } from './types'

export type DesktopAuthStrongCleanupReason =
  | 'login_failed'
  | 'logout'
  | 'rebind_device'
  | 'session_expired'
  | 'switch_account'

type DesktopAuthBindings = {
  BootstrapDesktopAuthRuntime?: () => Promise<void>
  ClearDesktopAuthSession?: () => Promise<void>
  DesktopAuthStrongCleanup?: (reason: DesktopAuthStrongCleanupReason) => Promise<void>
  FetchDesktopAuthProfile?: (accessToken: string) => Promise<any>
  LoadDesktopAuthSession?: () => Promise<any>
  LoginDesktopUser?: (username: string, password: string) => Promise<string>
  SaveDesktopAuthSession?: (accessToken: string, rememberMe: boolean) => Promise<void>
}

function getBinding<K extends keyof DesktopAuthBindings>(name: K): DesktopAuthBindings[K] | undefined {
  const fallback = (window as any)?.go?.main?.App as DesktopAuthBindings | undefined
  const staticBindings: DesktopAuthBindings = {
    BootstrapDesktopAuthRuntime,
    ClearDesktopAuthSession,
    DesktopAuthStrongCleanup,
    FetchDesktopAuthProfile,
    LoadDesktopAuthSession,
    LoginDesktopUser,
    SaveDesktopAuthSession,
  }
  return fallback?.[name] ?? staticBindings[name]
}

function hasDesktopAuthBinding(name: keyof DesktopAuthBindings): boolean {
  return Boolean((window as any)?.go?.main?.App?.[name])
}

function shouldUseDevAuthFallback(): boolean {
  return Boolean((import.meta as any).env?.DEV) && !hasDesktopAuthBinding('LoadDesktopAuthSession')
}

function normalizeDesktopAuthSession(input: any): DesktopAuthSession {
  return {
    accessToken: String(input?.accessToken || ''),
    rememberMe: Boolean(input?.rememberMe),
  }
}

function normalizeDesktopAuthProfile(input: any): DesktopAuthProfile {
  const user = input?.user ?? {}
  const roles = Array.isArray(input?.roles) ? input.roles : []

  return {
    user: {
      id: String(user?.id || ''),
      username: String(user?.username || ''),
      displayName: String(user?.displayName || ''),
    },
    roles: roles.map((role: any) => ({
      code: String(role?.code || ''),
      name: String(role?.name || ''),
    })),
    dataScope: String(input?.dataScope || ''),
  }
}

export async function loadDesktopAuthSession(): Promise<DesktopAuthSession> {
  const fn = getBinding('LoadDesktopAuthSession')
  if (shouldUseDevAuthFallback()) {
    return normalizeDesktopAuthSession({
      accessToken: 'dev-desktop-session',
      rememberMe: false,
    })
  }
  if (!fn || !hasDesktopAuthBinding('LoadDesktopAuthSession')) {
    return normalizeDesktopAuthSession(null)
  }
  return normalizeDesktopAuthSession(await fn())
}

export async function saveDesktopAuthSession(accessToken: string, rememberMe: boolean): Promise<void> {
  const fn = getBinding('SaveDesktopAuthSession')
  if (shouldUseDevAuthFallback()) return
  if (!fn || !hasDesktopAuthBinding('SaveDesktopAuthSession')) {
    throw new Error('当前环境缺少 SaveDesktopAuthSession 绑定')
  }
  await fn(accessToken, rememberMe)
}

export async function loginDesktopUser(username: string, password: string): Promise<string> {
  const fn = getBinding('LoginDesktopUser')
  if (shouldUseDevAuthFallback()) return 'dev-desktop-session'
  if (!fn || !hasDesktopAuthBinding('LoginDesktopUser')) {
    throw new Error('当前环境缺少 LoginDesktopUser 绑定')
  }
  const accessToken = await fn(username.trim(), password.trim())
  return String(accessToken || '').trim()
}

export async function clearDesktopAuthSession(): Promise<void> {
  const fn = getBinding('ClearDesktopAuthSession')
  if (!fn || !hasDesktopAuthBinding('ClearDesktopAuthSession')) return
  await fn()
}

export async function fetchDesktopAuthProfile(accessToken: string): Promise<DesktopAuthProfile> {
  const token = accessToken.trim()
  if (!token) {
    throw new Error('accessToken is required')
  }

  if (shouldUseDevAuthFallback()) {
    return normalizeDesktopAuthProfile({
      user: {
        id: 'dev',
        username: 'dev',
        displayName: '本地开发',
      },
      roles: [{ code: 'admin', name: '管理员' }],
      dataScope: 'all',
    })
  }

  const fn = getBinding('FetchDesktopAuthProfile')
  if (!fn || !hasDesktopAuthBinding('FetchDesktopAuthProfile')) {
    throw new Error('当前环境缺少 FetchDesktopAuthProfile 绑定')
  }
  return normalizeDesktopAuthProfile(await fn(token))
}

export async function bootstrapDesktopAuthRuntime(): Promise<void> {
  const fn = getBinding('BootstrapDesktopAuthRuntime')
  if (!fn || !hasDesktopAuthBinding('BootstrapDesktopAuthRuntime')) return
  await fn()
}

export async function runDesktopAuthStrongCleanup(reason: DesktopAuthStrongCleanupReason): Promise<void> {
  const fn = getBinding('DesktopAuthStrongCleanup')
  if (!fn || !hasDesktopAuthBinding('DesktopAuthStrongCleanup')) return
  await fn(reason)
}
