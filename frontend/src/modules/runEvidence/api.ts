import { WorkspaceRunEvents, WorkspaceRunEvidence, WorkspaceRuns } from '../../wailsjs/go/main/App'
import type { workspace } from '../../wailsjs/go/models'
import { devWorkspaceRuns, useDevWorkspaceFallback } from '../workspace/devData'
import type {
  RunEvent,
  RunEventsPayload,
  RunEvidenceIndex,
  RunQueryInput,
  RunRecord,
  RunsPayload,
  ShopRunEvidence,
} from './types'

type PascalRunQueryInput = Partial<Pick<workspace.RunQuery, 'Limit' | 'Status' | 'ShopID' | 'FailureCode'>>
type AnyRunQueryInput = RunQueryInput & PascalRunQueryInput

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : String(value ?? '')
}

function numberValue(value: unknown): number {
  const next = Number(value ?? 0)
  return Number.isFinite(next) ? next : 0
}

function normalizeRuntime(input: any): RunRecord['runtime'] {
  if (!input) return null
  return {
    pid: numberValue(input.pid),
    debugPort: numberValue(input.debugPort),
    currentUrl: stringValue(input.currentUrl),
    pageTitle: stringValue(input.pageTitle),
    targetUrl: stringValue(input.targetUrl),
  }
}

export function normalizeRun(input: any): RunRecord {
  return {
    runId: stringValue(input?.runId),
    taskId: stringValue(input?.taskId),
    shopId: stringValue(input?.shopId),
    taskType: stringValue(input?.taskType),
    status: stringValue(input?.status),
    statusLabel: stringValue(input?.statusLabel),
    startedAt: stringValue(input?.startedAt),
    finishedAt: stringValue(input?.finishedAt),
    profileId: stringValue(input?.profileId),
    runtime: normalizeRuntime(input?.runtime),
    bindSessionId: stringValue(input?.bindSessionId),
    manualActionRequired: Boolean(input?.manualActionRequired),
    challengeType: stringValue(input?.challengeType),
    failureCode: stringValue(input?.failureCode),
    failureMessage: stringValue(input?.failureMessage),
  }
}

export function normalizeRunEvent(input: any): RunEvent {
  return {
    eventId: stringValue(input?.eventId),
    stage: stringValue(input?.stage),
    message: stringValue(input?.message),
    details: input?.details && typeof input.details === 'object' ? { ...input.details } : {},
    createdAt: stringValue(input?.createdAt),
  }
}

function normalizeShopRunEvidence(input: any): ShopRunEvidence {
  return {
    latestOpen: input?.latestOpen ? normalizeRun(input.latestOpen) : null,
    latestCredential: input?.latestCredential ? normalizeRun(input.latestCredential) : null,
    latestValidation: input?.latestValidation ? normalizeRun(input.latestValidation) : null,
    latestFailure: input?.latestFailure ? normalizeRun(input.latestFailure) : null,
    activeRun: input?.activeRun ? normalizeRun(input.activeRun) : null,
  }
}

export function normalizeRunEvidenceIndex(input: any): RunEvidenceIndex {
  const byShop: RunEvidenceIndex['byShop'] = {}
  if (input?.byShop && typeof input.byShop === 'object') {
    Object.entries(input.byShop).forEach(([shopId, evidence]) => {
      byShop[String(shopId)] = normalizeShopRunEvidence(evidence)
    })
  }
  return { byShop }
}

export function buildWorkspaceRunQuery(query: AnyRunQueryInput = {}): workspace.RunQuery {
  return {
    Limit: numberValue(query.Limit ?? query.limit ?? 50),
    Status: stringValue(query.Status ?? query.status),
    ShopID: stringValue(query.ShopID ?? query.shopId),
    FailureCode: stringValue(query.FailureCode ?? query.failureCode),
  } as workspace.RunQuery
}

export async function fetchWorkspaceRuns(query: AnyRunQueryInput = {}): Promise<RunsPayload> {
  if (useDevWorkspaceFallback()) {
    let items = devWorkspaceRuns
    const shopId = stringValue(query.ShopID ?? query.shopId).trim()
    const status = stringValue(query.Status ?? query.status).trim()
    const failureCode = stringValue(query.FailureCode ?? query.failureCode).trim()
    if (shopId) items = items.filter((item) => item.shopId === shopId)
    if (status) items = items.filter((item) => item.status === status)
    if (failureCode) items = items.filter((item) => item.failureCode === failureCode)
    return { items: items.slice(0, numberValue(query.Limit ?? query.limit ?? 50)), total: items.length }
  }

  const payload = await WorkspaceRuns(buildWorkspaceRunQuery(query))
  const items = Array.isArray(payload?.items) ? payload.items.map(normalizeRun) : []
  return { items, total: numberValue(payload?.total ?? items.length) }
}

export async function fetchWorkspaceRunEvents(runId: string, limit = 50): Promise<RunEventsPayload> {
  const normalizedRunId = runId.trim()
  if (!normalizedRunId) {
    return { runId: '', items: [], total: 0 }
  }

  if (useDevWorkspaceFallback()) {
    const run = devWorkspaceRuns.find((item) => item.runId === normalizedRunId)
    const items = run
      ? [
          {
            eventId: `${normalizedRunId}-accepted`,
            stage: 'accepted',
            message: '任务已进入执行队列',
            details: { shopId: run.shopId, taskType: run.taskType },
            createdAt: run.startedAt,
          },
          {
            eventId: `${normalizedRunId}-latest`,
            stage: run.status,
            message: run.failureMessage || run.statusLabel || '任务状态已更新',
            details: { failureCode: run.failureCode },
            createdAt: run.finishedAt || run.startedAt,
          },
        ]
      : []
    return { runId: normalizedRunId, items, total: items.length }
  }

  const payload = await WorkspaceRunEvents(normalizedRunId, limit)
  const items = Array.isArray(payload?.items) ? payload.items.map(normalizeRunEvent) : []
  return {
    runId: stringValue(payload?.runId || normalizedRunId),
    items,
    total: numberValue(payload?.total ?? items.length),
  }
}

export async function fetchWorkspaceRunEvidence(query: AnyRunQueryInput = {}): Promise<RunEvidenceIndex> {
  if (useDevWorkspaceFallback()) {
    const runs = await fetchWorkspaceRuns(query)
    const byShop: RunEvidenceIndex['byShop'] = {}
    runs.items.forEach((run) => {
      const current = byShop[run.shopId] || {
        latestOpen: null,
        latestCredential: null,
        latestValidation: null,
        latestFailure: null,
        activeRun: null,
      }
      if (run.taskType === 'open') current.latestOpen = run
      if (run.taskType === 'bind') current.latestCredential = run
      if (run.taskType === 'validate') current.latestValidation = run
      if (run.status === 'failed') current.latestFailure = run
      if (!run.finishedAt && run.status !== 'failed' && run.status !== 'succeeded') current.activeRun = run
      byShop[run.shopId] = current
    })
    return { byShop }
  }

  const payload = await WorkspaceRunEvidence(buildWorkspaceRunQuery(query))
  return normalizeRunEvidenceIndex(payload)
}
