import type { RecoveryAction, WorkbenchActionKey, WorkbenchQueueKey, WorkbenchState } from './types'

export const credentialFailureCodes = new Set([
  'ANT_BACKEND_LOGIN_REQUIRED',
  'ANT_SESSION_RESTORE_FAILED',
])

const coreFailureCodes = new Set([
  'ANT_CORE_UNAVAILABLE',
  'ANT_CORE_NOT_FOUND',
  'ANT_FINGERPRINT_CORE_REQUIRED',
])

const targetMismatchFailureCodes = new Set([
  'ANT_BACKEND_TARGET_MISMATCH',
  'target_url_not_reached',
])

const actions: Record<WorkbenchActionKey, RecoveryAction> = {
  open: { key: 'open', label: '打开后台', description: '店铺已可执行，可直接打开后台', retryable: true, batchSkippable: false },
  close: { key: 'close', label: '关闭后台', description: '店铺后台正在运行，可关闭本机浏览器实例', retryable: true, batchSkippable: false },
  bind: { key: 'bind', label: '更新凭据', description: '共享登录未就绪，需要更新凭据', retryable: true, batchSkippable: false },
  validate: { key: 'validate', label: '本机验证', description: '需要在本机完成验证', retryable: true, batchSkippable: false },
  retry: { key: 'retry', label: '重试', description: '最近失败动作可重试', retryable: true, batchSkippable: false },
  core_management: { key: 'core_management', label: '配置指纹内核', description: '缺少可用指纹内核，需先修复内核配置', retryable: false, batchSkippable: true },
  refresh: { key: 'refresh', label: '刷新同步', description: '刷新授权店铺与本地 profile 映射', retryable: true, batchSkippable: false },
  diagnostics: { key: 'diagnostics', label: '查看诊断', description: '查看运行证据并导出诊断信息', retryable: false, batchSkippable: true },
  none: { key: 'none', label: '不可执行', description: '当前状态不允许执行动作', retryable: false, batchSkippable: true },
}

export function normalizeAuthorizationStatus(status = '') {
  if (status === 'valid') return 'ready'
  if (status === 'manual_required') return 'awaiting_verification'
  if (status === 'expired' || status === 'revoked') return 'relogin_required'
  return status || 'not_configured'
}

export function authorizationStatusPresentation(status = '', label = ''): {
  status: string
  label: string
  queue: WorkbenchQueueKey
  recommendedAction: WorkbenchActionKey
  primaryLabel: string
  description: string
} {
  const normalized = normalizeAuthorizationStatus(status)
  const displayLabel = label || status || '未配置'
  if (normalized === 'ready') {
    return {
      status: normalized,
      label: displayLabel,
      queue: 'ready',
      recommendedAction: 'open',
      primaryLabel: '打开后台',
      description: '授权状态可用，可直接打开店铺后台。',
    }
  }
  if (normalized === 'awaiting_verification') {
    return {
      status: normalized,
      label: displayLabel,
      queue: 'manual',
      recommendedAction: 'validate',
      primaryLabel: '继续验证',
      description: '当前存在待人工验证流程，进入工作台继续处理。',
    }
  }
  if (normalized === 'binding') {
    return {
      status: normalized,
      label: displayLabel,
      queue: 'credential',
      recommendedAction: 'bind',
      primaryLabel: '查看进度',
      description: '当前已有绑定流程，进入工作台查看进度。',
    }
  }
  if (normalized === 'disabled') {
    return {
      status: normalized,
      label: displayLabel,
      queue: 'credential',
      recommendedAction: 'bind',
      primaryLabel: '重新启用',
      description: '当前授权已停用，进入工作台重新启用或更新凭据。',
    }
  }
  if (normalized === 'validation_failed') {
    return {
      status: normalized,
      label: displayLabel,
      queue: 'credential',
      recommendedAction: 'validate',
      primaryLabel: '本机验证',
      description: '验证失败，需要重新本机验证后再打开后台。',
    }
  }
  if (normalized === 'relogin_required') {
    return {
      status: normalized,
      label: displayLabel,
      queue: 'credential',
      recommendedAction: 'bind',
      primaryLabel: '更新凭据',
      description: '共享登录不可用，需要更新凭据后再打开后台。',
    }
  }
  return {
    status: normalized,
    label: displayLabel,
    queue: 'credential',
    recommendedAction: 'bind',
    primaryLabel: '去授权',
    description: '当前未配置本地授权，进入工作台完成授权接入。',
  }
}

export function openFailurePresentation(failureCode = '', failureMessage = '') {
  const message = failureMessage || failureCode || '未知失败'
  if (credentialFailureCodes.has(failureCode)) {
    return {
      label: '凭据失效',
      evidence: `open · 打开失败：${message}`,
    }
  }
  if (targetMismatchFailureCodes.has(failureCode)) {
    return {
      label: '后台目标不匹配',
      evidence: `open · 打开失败：${message}`,
    }
  }
  if (coreFailureCodes.has(failureCode)) {
    return {
      label: '指纹内核不可用',
      evidence: `open · 打开失败：${message}`,
    }
  }
  return {
    label: failureCode ? '打开失败' : '',
    evidence: failureCode || failureMessage ? `open · 打开失败：${message}` : '',
  }
}

export function queueForWorkbenchState(input: {
  reclaimPending?: boolean
  instanceRunning?: boolean
  activeRun?: boolean
  sharedLoginStatus?: string
  failureCode?: string
}): WorkbenchQueueKey {
  if (input.reclaimPending) return 'reclaim'
  if (input.activeRun || input.instanceRunning) return 'running'
  if (credentialFailureCodes.has(input.failureCode || '')) return 'credential'
  if (input.failureCode) return 'failed'
  return authorizationStatusPresentation(input.sharedLoginStatus).queue
}

export function recoveryActionForState(input: {
  reclaimPending?: boolean
  instanceRunning?: boolean
  profileExists?: boolean
  coreReady?: boolean
  sharedLoginStatus?: string
  failureCode?: string
}): RecoveryAction {
  if (input.reclaimPending) return actions.none
  if (input.instanceRunning) return actions.close
  if (!input.profileExists) return actions.refresh
  if (!input.coreReady || coreFailureCodes.has(input.failureCode || '')) return actions.core_management
  if (credentialFailureCodes.has(input.failureCode || '')) return actions.bind
  if (targetMismatchFailureCodes.has(input.failureCode || '')) return actions.diagnostics
  const presentation = authorizationStatusPresentation(input.sharedLoginStatus)
  if (presentation.recommendedAction !== 'open') return actions[presentation.recommendedAction]
  if (input.failureCode) return actions.retry
  return actions.open
}

export function deriveWorkbenchState(input: {
  reclaimPending?: boolean
  instanceRunning?: boolean
  activeRun?: boolean
  profileExists?: boolean
  coreReady?: boolean
  sharedLoginStatus?: string
  failureCode?: string
  failureMessage?: string
}): WorkbenchState {
  const rawAuthorizationStatus = input.sharedLoginStatus || 'not_configured'
  const normalizedAuthorizationStatus = normalizeAuthorizationStatus(rawAuthorizationStatus)
  const failure = openFailurePresentation(input.failureCode || '', input.failureMessage || '')
  const action = recoveryActionForState(input)
  const queue = queueForWorkbenchState(input)
  const presentation = authorizationStatusPresentation(rawAuthorizationStatus)
  const evidenceText = failure.evidence
  const failureLabel = failure.label
  const usesAuthorizationPresentation =
    queue === presentation.queue && action.key === presentation.recommendedAction

  return {
    rawAuthorizationStatus,
    normalizedAuthorizationStatus,
    queue,
    recommendedAction: action.key,
    primaryLabel: usesAuthorizationPresentation ? presentation.primaryLabel : (action.label || presentation.primaryLabel),
    description: evidenceText || action.description || presentation.description,
    failureCode: input.failureCode || '',
    failureLabel,
    evidenceText,
    diagnosticCode: action.key === 'diagnostics' ? (input.failureCode || '') : '',
    instanceRunning: Boolean(input.instanceRunning || input.activeRun),
    canExecute: action.key !== 'none',
  }
}
