import type { WorkbenchActionKey, WorkbenchQueueKey } from './types'

export const workbenchQueueLabels: Record<WorkbenchQueueKey, string> = {
  ready: '可打开',
  manual: '待验证',
  credential: '凭据处理',
  failed: '失败',
  running: '运行中',
  reclaim: '待回收',
}

export function workbenchQueueVariant(queue: WorkbenchQueueKey) {
  if (queue === 'ready') return 'success' as const
  if (queue === 'failed' || queue === 'reclaim') return 'error' as const
  if (queue === 'running') return 'info' as const
  return 'warning' as const
}

export function workbenchActionLabel(action: WorkbenchActionKey) {
  if (action === 'open') return '打开后台'
  if (action === 'close') return '关闭后台'
  if (action === 'bind') return '更新凭据'
  if (action === 'validate') return '本机验证'
  if (action === 'retry') return '重试'
  if (action === 'refresh') return '刷新同步'
  if (action === 'core_management') return '配置内核'
  if (action === 'diagnostics') return '查看诊断'
  return '不可执行'
}
