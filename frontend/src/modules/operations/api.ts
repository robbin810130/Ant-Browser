import type { OperationTask, OperationTaskStatus } from './types'

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

