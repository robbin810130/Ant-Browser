import type { RunEvidenceIndex, RunRecord, ShopRunEvidence } from './types'

const terminalStatuses = new Set(['succeeded', 'failed'])

function parseTime(value: string) {
  const time = Date.parse(value || '')
  return Number.isFinite(time) ? time : 0
}

function newer(current: RunRecord | null, candidate: RunRecord) {
  if (!current) return candidate
  return parseTime(candidate.startedAt) > parseTime(current.startedAt) ? candidate : current
}

function emptyEvidence(): ShopRunEvidence {
  return {
    latestOpen: null,
    latestCredential: null,
    latestValidation: null,
    latestFailure: null,
    activeRun: null,
  }
}

export function buildRunEvidenceIndex(runs: RunRecord[]): RunEvidenceIndex {
  const byShop: RunEvidenceIndex['byShop'] = {}
  runs.forEach((run) => {
    if (!run.shopId) return
    const current = byShop[run.shopId] || emptyEvidence()
    if (run.taskType === 'open') current.latestOpen = newer(current.latestOpen, run)
    if (run.taskType === 'bind') current.latestCredential = newer(current.latestCredential, run)
    if (run.taskType === 'validate') current.latestValidation = newer(current.latestValidation, run)
    if (run.status === 'failed' || run.failureCode || run.failureMessage) {
      current.latestFailure = newer(current.latestFailure, run)
    }
    if (run.status && !terminalStatuses.has(run.status)) {
      current.activeRun = newer(current.activeRun, run)
    }
    byShop[run.shopId] = current
  })
  return { byShop }
}

export function evidenceForShop(index: RunEvidenceIndex, shopId: string): ShopRunEvidence {
  return index.byShop[shopId] || emptyEvidence()
}
