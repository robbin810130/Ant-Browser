import type { DesktopAuthProfile, DesktopAuthSession } from './types'

export type DesktopAuthStrongCleanupReason =
  | 'login_failed'
  | 'logout'
  | 'rebind_device'
  | 'session_expired'
  | 'switch_account'

type AppBindings = {
  LoadDesktopAuthSession?: () => Promise<any>
  SaveDesktopAuthSession?: (accessToken: string, rememberMe: boolean) => Promise<void>
  ClearDesktopAuthSession?: () => Promise<void>
  LoginDesktopUser?: (username: string, password: string) => Promise<string>
  FetchDesktopAuthProfile?: (accessToken: string) => Promise<any>
  BootstrapDesktopAuthRuntime?: () => Promise<void>
  DesktopAuthStrongCleanup?: (reason: string) => Promise<void>
}

async function getBindings(): Promise<AppBindings> {
  const windowBindings = (window as any).go?.main?.App ?? {}

  try {
    const moduleBindings = (await import('../../wailsjs/go/main/App')) as any
    return {
      ...moduleBindings,
      ...windowBindings,
    }
  } catch {
    return windowBindings
  }
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
  const bindings = await getBindings()
  if (!bindings.LoadDesktopAuthSession) {
    return normalizeDesktopAuthSession(null)
  }
  return normalizeDesktopAuthSession(await bindings.LoadDesktopAuthSession())
}

export async function saveDesktopAuthSession(accessToken: string, rememberMe: boolean): Promise<void> {
  const bindings = await getBindings()
  if (!bindings.SaveDesktopAuthSession) {
    throw new Error('当前环境不支持 SaveDesktopAuthSession')
  }
  await bindings.SaveDesktopAuthSession(accessToken, rememberMe)
}

export async function loginDesktopUser(username: string, password: string): Promise<string> {
  const bindings = await getBindings()
  if (!bindings.LoginDesktopUser) {
    throw new Error('当前环境不支持 LoginDesktopUser')
  }

  const accessToken = await bindings.LoginDesktopUser(username.trim(), password.trim())
  return String(accessToken || '').trim()
}

export async function clearDesktopAuthSession(): Promise<void> {
  const bindings = await getBindings()
  if (!bindings.ClearDesktopAuthSession) {
    throw new Error('当前环境不支持 ClearDesktopAuthSession')
  }
  await bindings.ClearDesktopAuthSession()
}

export async function fetchDesktopAuthProfile(accessToken: string): Promise<DesktopAuthProfile> {
  const token = accessToken.trim()
  if (!token) {
    throw new Error('accessToken is required')
  }

  const bindings = await getBindings()
  if (!bindings.FetchDesktopAuthProfile) {
    throw new Error('当前环境不支持 FetchDesktopAuthProfile')
  }
  return normalizeDesktopAuthProfile(await bindings.FetchDesktopAuthProfile(token))
}

export async function bootstrapDesktopAuthRuntime(): Promise<void> {
  const bindings = await getBindings()
  if (!bindings.BootstrapDesktopAuthRuntime) {
    throw new Error('当前环境不支持 BootstrapDesktopAuthRuntime')
  }
  await bindings.BootstrapDesktopAuthRuntime()
}

export async function runDesktopAuthStrongCleanup(reason: DesktopAuthStrongCleanupReason): Promise<void> {
  const bindings = await getBindings()
  if (!bindings.DesktopAuthStrongCleanup) {
    throw new Error('当前环境不支持 DesktopAuthStrongCleanup')
  }
  await bindings.DesktopAuthStrongCleanup(reason)
}
