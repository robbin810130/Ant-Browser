import type { RecoveryAction, WorkbenchActionKey } from './types'

const actions: Record<WorkbenchActionKey, RecoveryAction> = {
  open: { key: 'open', label: '打开后台', description: '店铺已可执行，可直接打开后台', retryable: true, batchSkippable: false },
  bind: { key: 'bind', label: '更新凭据', description: '共享登录未就绪，需要更新凭据', retryable: true, batchSkippable: false },
  validate: { key: 'validate', label: '本机验证', description: '需要在本机完成验证', retryable: true, batchSkippable: false },
  retry: { key: 'retry', label: '重试', description: '最近失败动作可重试', retryable: true, batchSkippable: false },
  core_management: { key: 'core_management', label: '配置指纹内核', description: '缺少可用指纹内核，需先修复内核配置', retryable: false, batchSkippable: true },
  refresh: { key: 'refresh', label: '刷新同步', description: '刷新授权店铺与本地 profile 映射', retryable: true, batchSkippable: false },
  diagnostics: { key: 'diagnostics', label: '查看诊断', description: '查看运行证据并导出诊断信息', retryable: false, batchSkippable: true },
  none: { key: 'none', label: '不可执行', description: '当前状态不允许执行动作', retryable: false, batchSkippable: true },
}

const coreFailureCodes = new Set([
  'ANT_CORE_UNAVAILABLE',
  'ANT_CORE_NOT_FOUND',
  'ANT_FINGERPRINT_CORE_REQUIRED',
])

export const credentialFailureCodes = new Set([
  'ANT_BACKEND_LOGIN_REQUIRED',
  'ANT_SESSION_RESTORE_FAILED',
])

export function recoveryActionFor(input: {
  reclaimPending?: boolean
  profileExists?: boolean
  coreReady?: boolean
  sharedLoginStatus?: string
  failureCode?: string
}): RecoveryAction {
  if (input.reclaimPending) return actions.none
  if (!input.profileExists) return actions.refresh
  if (!input.coreReady || coreFailureCodes.has(input.failureCode || '')) {
    return actions.core_management
  }
  if (credentialFailureCodes.has(input.failureCode || '')) return actions.bind
  if (input.sharedLoginStatus === 'awaiting_verification') return actions.validate
  if (input.sharedLoginStatus === 'validation_failed') return actions.bind
  if (input.sharedLoginStatus !== 'ready') return actions.bind
  if (input.failureCode) return actions.retry
  return actions.open
}
