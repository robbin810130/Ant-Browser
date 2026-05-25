import type { WorkspaceSharedLoginBindSession } from './types'

export type SharedLoginAction = 'bind' | 'validate'

export interface SharedLoginDialogState {
  action: SharedLoginAction
  shopId: string
  shopName: string
  session: WorkspaceSharedLoginBindSession | null
  starting: boolean
  terminalHandled: boolean
  errorMessage: string
}

export function sharedLoginActionLabel(action: SharedLoginAction) {
  if (action === 'bind') return '更新凭据'
  return '本机验证'
}

export function sharedLoginActionSuccessMessage(action: SharedLoginAction, shopName: string) {
  if (action === 'bind') return `${shopName} 共享凭据已更新`
  return `${shopName} 共享会话验证完成`
}

export function isTerminalSharedLoginStatus(status: string) {
  return new Set(['completed', 'failed', 'expired', 'succeeded']).has(status.trim())
}

export function resolveSharedLoginActionError(action: SharedLoginAction, error: any) {
  const fallback = action === 'bind' ? '发起更新凭据失败' : '发起本机验证失败'
  return String(error?.message || '').trim() || fallback
}
