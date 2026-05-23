import type { OperationTask, OperationTaskStatus } from './types'
import { useDevWorkspaceFallback } from '../workspace/devData'

const knownStatuses = new Set<OperationTaskStatus>(['waiting', 'running', 'blocked', 'failed', 'completed'])

export function normalizeOperationTask(input: any): OperationTask {
  const status = String(input?.status || 'waiting') as OperationTaskStatus
  return {
    taskId: String(input?.taskId || ''),
    shopId: String(input?.shopId || ''),
    shopName: String(input?.shopName || ''),
    taskType: String(input?.taskType || ''),
    title: String(input?.title || ''),
    status: knownStatuses.has(status) ? status : 'waiting',
    blockedReason: String(input?.blockedReason || ''),
    failureMessage: String(input?.failureMessage || ''),
    updatedAt: String(input?.updatedAt || ''),
  }
}

export async function fetchOperationTasks(): Promise<OperationTask[]> {
  if (useDevWorkspaceFallback()) {
    return [
      normalizeOperationTask({
        taskId: 'op-price-audit',
        shopId: 'shop-ready',
        shopName: '义乌百货样板店',
        taskType: 'price_audit',
        title: '检查近 7 天爆款价格带',
        status: 'running',
        updatedAt: '2026-05-23T09:25:00+08:00',
      }),
      normalizeOperationTask({
        taskId: 'op-title-refresh',
        shopId: 'shop-manual',
        shopName: '深圳数码配件店',
        taskType: 'title_refresh',
        title: '等待人工验证后批量优化商品标题',
        status: 'blocked',
        blockedReason: '店铺登录态需要人工短信验证',
        updatedAt: '2026-05-23T09:15:00+08:00',
      }),
      normalizeOperationTask({
        taskId: 'op-credential-rebind',
        shopId: 'shop-credential',
        shopName: '广州家居源头厂',
        taskType: 'credential_rebind',
        title: '重新绑定 ASM 授权凭据',
        status: 'waiting',
        updatedAt: '2026-05-23T09:00:00+08:00',
      }),
    ]
  }

  return []
}

export function deriveOperationTaskCounts(tasks: OperationTask[]) {
  return {
    total: tasks.length,
    waiting: tasks.filter((task) => task.status === 'waiting').length,
    running: tasks.filter((task) => task.status === 'running').length,
    blocked: tasks.filter((task) => task.status === 'blocked').length,
    failed: tasks.filter((task) => task.status === 'failed').length,
    completed: tasks.filter((task) => task.status === 'completed').length,
  }
}
