export type OperationTaskStatus = 'waiting' | 'running' | 'blocked' | 'failed' | 'completed'

export interface OperationTask {
  taskId: string
  shopId: string
  shopName: string
  taskType: string
  title: string
  status: OperationTaskStatus
  blockedReason: string
  failureMessage: string
  updatedAt: string
  runId: string
  failureCode: string
}
