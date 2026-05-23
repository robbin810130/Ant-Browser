export type RunTaskType = 'open' | 'bind' | 'validate' | 'diagnose' | 'retry' | string
export type RunStatus =
  | 'accepted'
  | 'authorizing'
  | 'launching'
  | 'awaiting_verification'
  | 'capturing'
  | 'succeeded'
  | 'failed'
  | string

export interface RunRuntime {
  pid: number
  debugPort: number
  currentUrl: string
  pageTitle: string
  targetUrl: string
}

export interface RunRecord {
  runId: string
  taskId: string
  shopId: string
  taskType: RunTaskType
  status: RunStatus
  statusLabel: string
  startedAt: string
  finishedAt: string
  profileId: string
  runtime: RunRuntime | null
  bindSessionId: string
  manualActionRequired: boolean
  challengeType: string
  failureCode: string
  failureMessage: string
}

export interface RunEvent {
  eventId: string
  stage: string
  message: string
  details: Record<string, unknown>
  createdAt: string
}

export interface RunsPayload {
  items: RunRecord[]
  total: number
}

export interface RunEventsPayload {
  runId: string
  items: RunEvent[]
  total: number
}

export interface ShopRunEvidence {
  latestOpen: RunRecord | null
  latestCredential: RunRecord | null
  latestValidation: RunRecord | null
  latestFailure: RunRecord | null
  activeRun: RunRecord | null
}

export interface RunEvidenceIndex {
  byShop: Record<string, ShopRunEvidence>
}

export interface RunQueryInput {
  shopId?: string
  status?: string
  failureCode?: string
  limit?: number
}
