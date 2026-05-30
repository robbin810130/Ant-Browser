import { describe, expect, it } from 'vitest'
import {
  authorizationStatusPresentation,
  queueForWorkbenchState,
  recoveryActionForState,
  openFailurePresentation,
} from './statusMatrix'

describe('workbench authorization status matrix', () => {
  it.each([
    ['not_configured', '未配置', 'credential', 'bind', '去授权'],
    ['binding', '绑定中', 'credential', 'bind', '查看进度'],
    ['awaiting_verification', '待验证', 'manual', 'validate', '继续验证'],
    ['ready', '可打开', 'ready', 'open', '打开后台'],
    ['valid', '可打开', 'ready', 'open', '打开后台'],
    ['relogin_required', '需重新登录', 'credential', 'bind', '更新凭据'],
    ['validation_failed', '验证失败', 'credential', 'bind', '更新凭据'],
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
})
