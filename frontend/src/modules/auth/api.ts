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
  return normalizeDesktopAuthSession(await LoadDesktopAuthSession())
}

export async function saveDesktopAuthSession(accessToken: string, rememberMe: boolean): Promise<void> {
  await SaveDesktopAuthSession(accessToken, rememberMe)
}

export async function loginDesktopUser(username: string, password: string): Promise<string> {
  const accessToken = await LoginDesktopUser(username.trim(), password.trim())
  return String(accessToken || '').trim()
}

export async function clearDesktopAuthSession(): Promise<void> {
  await ClearDesktopAuthSession()
}

export async function fetchDesktopAuthProfile(accessToken: string): Promise<DesktopAuthProfile> {
  const token = accessToken.trim()
  if (!token) {
    throw new Error('accessToken is required')
  }

  return normalizeDesktopAuthProfile(await FetchDesktopAuthProfile(token))
}

export async function bootstrapDesktopAuthRuntime(): Promise<void> {
  await BootstrapDesktopAuthRuntime()
}

export async function runDesktopAuthStrongCleanup(reason: DesktopAuthStrongCleanupReason): Promise<void> {
  await DesktopAuthStrongCleanup(reason)
}
