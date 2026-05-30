import type { RunRecord, ShopRunEvidence } from '../runEvidence'
import type { WorkspaceAuthorizedShop } from '../workspace/types'

export type WorkbenchQueueKey = 'ready' | 'manual' | 'credential' | 'failed' | 'running' | 'reclaim'
export type WorkbenchActionKey = 'open' | 'bind' | 'validate' | 'retry' | 'core_management' | 'refresh' | 'diagnostics' | 'none'
export type ExecutableBatchActionKey = 'open' | 'bind' | 'validate' | 'retry' | 'refresh'

export interface WorkbenchRow {
  shop: WorkspaceAuthorizedShop
  evidence: ShopRunEvidence
  queue: WorkbenchQueueKey
  recommendedAction: WorkbenchActionKey
  failureCode: string
  failureMessage: string
}

export interface RecoveryAction {
  key: WorkbenchActionKey
  label: string
  description: string
  retryable: boolean
  batchSkippable: boolean
}

export interface BatchCandidate {
  shopId: string
  action: ExecutableBatchActionKey
  eligible: boolean
  skipReason: string
}

export interface BatchSummary {
  total: number
  eligible: number
  skipped: number
  failed: number
  succeeded: number
}

export type { RunRecord, ShopRunEvidence, WorkspaceAuthorizedShop }
