import { describe, expect, it } from 'vitest'
import {
  authorizationStatusPresentation,
  queueForWorkbenchState,
  recoveryActionForState,
  openFailurePresentation,
  deriveWorkbenchState,
} from './statusMatrix'

describe('workbench authorization status matrix', () => {
  it.each([
    ['not_configured', '未配置', 'credential', 'bind', '去授权'],
    ['binding', '绑定中', 'credential', 'bind', '查看进度'],
    ['awaiting_verification', '待验证', 'manual', 'validate', '继续验证'],
    ['ready', '可打开', 'ready', 'open', '打开后台'],
    ['valid', '可打开', 'ready', 'open', '打开后台'],
    ['relogin_required', '需重新登录', 'credential', 'bind', '更新凭据'],
    ['validation_failed', '验证失败', 'credential', 'validate', '本机验证'],
    ['disabled', '已停用', 'credential', 'bind', '重新启用'],
    ['expired', '授权失效', 'credential', 'bind', '更新凭据'],
  ])('maps %s into one product path', (status, label, queue, action, primaryLabel) => {
    const presentation = authorizationStatusPresentation(status, label)

    expect(presentation.queue).toBe(queue)
    expect(presentation.recommendedAction).toBe(action)
    expect(presentation.primaryLabel).toBe(primaryLabel)
    expect(presentation.description).not.toBe('')
  })

  it('keeps backend login failures on the credential path', () => {
    expect(queueForWorkbenchState({
      sharedLoginStatus: 'ready',
      failureCode: 'ANT_BACKEND_LOGIN_REQUIRED',
    })).toBe('credential')
    expect(recoveryActionForState({
      profileExists: true,
      coreReady: true,
      sharedLoginStatus: 'ready',
      failureCode: 'ANT_BACKEND_LOGIN_REQUIRED',
    }).key).toBe('bind')
    expect(openFailurePresentation('ANT_BACKEND_LOGIN_REQUIRED', '打开后进入登录页')).toMatchObject({
      label: '凭据失效',
      evidence: 'open · 打开失败：打开后进入登录页',
    })
  })

  it('keeps session restore failures on the credential path', () => {
    expect(queueForWorkbenchState({
      sharedLoginStatus: 'ready',
      failureCode: 'ANT_SESSION_RESTORE_FAILED',
    })).toBe('credential')
    expect(recoveryActionForState({
      profileExists: true,
      coreReady: true,
      sharedLoginStatus: 'ready',
      failureCode: 'ANT_SESSION_RESTORE_FAILED',
    }).key).toBe('bind')
    expect(openFailurePresentation('ANT_SESSION_RESTORE_FAILED', '共享会话恢复失败')).toMatchObject({
      label: '凭据失效',
      evidence: 'open · 打开失败：共享会话恢复失败',
    })
  })

  it('separates target mismatch from credential failures', () => {
    expect(queueForWorkbenchState({
      sharedLoginStatus: 'ready',
      failureCode: 'ANT_BACKEND_TARGET_MISMATCH',
    })).toBe('failed')
    expect(recoveryActionForState({
      profileExists: true,
      coreReady: true,
      sharedLoginStatus: 'ready',
      failureCode: 'ANT_BACKEND_TARGET_MISMATCH',
    }).key).toBe('diagnostics')
    expect(openFailurePresentation('ANT_BACKEND_TARGET_MISMATCH', '进入了其他店铺后台')).toMatchObject({
      label: '后台目标不匹配',
      evidence: 'open · 打开失败：进入了其他店铺后台',
    })
  })

  it('routes already opened shop profiles to the close action', () => {
    expect(queueForWorkbenchState({
      sharedLoginStatus: 'ready',
      instanceRunning: true,
    })).toBe('running')
    expect(recoveryActionForState({
      profileExists: true,
      coreReady: true,
      sharedLoginStatus: 'ready',
      instanceRunning: true,
    }).key).toBe('close')
  })

  it('derives one shared workbench state for ready shops', () => {
    expect(deriveWorkbenchState({
      sharedLoginStatus: 'ready',
      profileExists: true,
      coreReady: true,
    })).toMatchObject({
      rawAuthorizationStatus: 'ready',
      normalizedAuthorizationStatus: 'ready',
      queue: 'ready',
      recommendedAction: 'open',
      primaryLabel: '打开后台',
      instanceRunning: false,
      canExecute: true,
    })
  })

  it('gives running shop instances the close action first', () => {
    expect(deriveWorkbenchState({
      sharedLoginStatus: 'ready',
      profileExists: true,
      coreReady: true,
      instanceRunning: true,
    })).toMatchObject({
      queue: 'running',
      recommendedAction: 'close',
      primaryLabel: '关闭后台',
      instanceRunning: true,
      canExecute: true,
    })
  })

  it('keeps unknown authorization status on the credential path', () => {
    expect(deriveWorkbenchState({
      sharedLoginStatus: 'mystery_status',
      profileExists: true,
      coreReady: true,
    })).toMatchObject({
      rawAuthorizationStatus: 'mystery_status',
      normalizedAuthorizationStatus: 'mystery_status',
      queue: 'credential',
      recommendedAction: 'bind',
      primaryLabel: '去授权',
      canExecute: true,
    })
  })

  it('keeps credential work ahead of core setup until shared login is ready', () => {
    expect(deriveWorkbenchState({
      sharedLoginStatus: 'not_configured',
      profileExists: true,
      coreReady: false,
    })).toMatchObject({
      queue: 'credential',
      recommendedAction: 'bind',
      primaryLabel: '去授权',
      canExecute: true,
    })
  })

  it('lets managed core failures retry opening instead of sending users to manual core setup', () => {
    expect(deriveWorkbenchState({
      sharedLoginStatus: 'ready',
      profileExists: true,
      coreReady: false,
      failureCode: 'ANT_CORE_NOT_FOUND',
      failureMessage: '未找到指纹内核',
    })).toMatchObject({
      queue: 'failed',
      recommendedAction: 'open',
      primaryLabel: '打开后台',
      failureCode: 'ANT_CORE_NOT_FOUND',
      failureLabel: '指纹内核不可用',
      evidenceText: 'open · 打开失败：未找到指纹内核',
      canExecute: true,
    })
  })

  it('keeps target mismatch on diagnostics', () => {
    expect(deriveWorkbenchState({
      sharedLoginStatus: 'ready',
      profileExists: true,
      coreReady: true,
      failureCode: 'target_url_not_reached',
      failureMessage: '没有进入目标后台',
    })).toMatchObject({
      queue: 'failed',
      recommendedAction: 'diagnostics',
      primaryLabel: '查看诊断',
      failureLabel: '后台目标不匹配',
      evidenceText: 'open · 打开失败：没有进入目标后台',
    })
  })
})
