import type { OperationTask, OperationTaskStatus } from './types'
import { useDevWorkspaceFallback } from '../workspace/devData'
import { WorkspaceOperationTasks } from '../../wailsjs/go/main/App'
import type { workspace } from '../../wailsjs/go/models'

const knownStatuses = new Set<OperationTaskStatus>(['waiting', 'running', 'blocked', 'failed', 'completed'])

export type OperationTaskQuery = Partial<Pick<workspace.OperationTaskQuery, 'Limit' | 'Status' | 'ShopID' | 'TaskType'>> & {
  limit?: number
  status?: string
  shopId?: string
  taskType?: string
}

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
    runId: String(input?.runId || ''),
    failureCode: String(input?.failureCode || ''),
  }
}

export function operationTaskStatusLabel(status: OperationTaskStatus): string {
  if (status === 'running') return '运行中'
  if (status === 'blocked') return '阻塞'
  if (status === 'failed') return '失败'
  if (status === 'completed') return '已完成'
  return '等待中'
}

function buildOperationTaskQuery(query: OperationTaskQuery = {}): workspace.OperationTaskQuery {
  return {
    Limit: Number(query.Limit ?? query.limit ?? 100),
    Status: String(query.Status ?? query.status ?? ''),
    ShopID: String(query.ShopID ?? query.shopId ?? ''),
    TaskType: String(query.TaskType ?? query.taskType ?? ''),
  } as workspace.OperationTaskQuery
}

export async function fetchOperationTasks(query: OperationTaskQuery = {}): Promise<OperationTask[]> {
  if (useDevWorkspaceFallback()) {
    let items = [
      normalizeOperationTask({
        taskId: 'op-price-audit',
        shopId: 'shop-ready',
        shopName: '义乌百货样板店',
        taskType: 'price_audit',
        title: '检查近 7 天爆款价格带',
        status: 'running',
        updatedAt: '2026-05-23T09:25:00+08:00',
        runId: 'run-open-ready',
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
        runId: 'run-validate-manual',
      }),
      normalizeOperationTask({
        taskId: 'op-credential-rebind',
        shopId: 'shop-credential',
        shopName: '广州家居源头厂',
        taskType: 'credential_rebind',
        title: '重新绑定 ASM 授权凭据',
        status: 'waiting',
        updatedAt: '2026-05-23T09:00:00+08:00',
        failureCode: 'AUTH_EXPIRED',
      }),
    ]
    const shopId = String(query.ShopID ?? query.shopId ?? '').trim()
    const status = String(query.Status ?? query.status ?? '').trim()
    const taskType = String(query.TaskType ?? query.taskType ?? '').trim()
    const limit = Number(query.Limit ?? query.limit ?? 100)
    if (shopId) items = items.filter((item) => item.shopId === shopId)
    if (status) items = items.filter((item) => item.status === status)
    if (taskType) items = items.filter((item) => item.taskType === taskType)
    return items.slice(0, Number.isFinite(limit) && limit > 0 ? limit : 100)
  }

  const payload = await WorkspaceOperationTasks(buildOperationTaskQuery(query))
  return Array.isArray(payload?.items) ? payload.items.map(normalizeOperationTask) : []
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
