import type { BatchCandidate, BatchSummary, WorkbenchActionKey, WorkbenchRow } from './types'

export function buildBatchCandidates(rows: WorkbenchRow[], action: WorkbenchActionKey): BatchCandidate[] {
  return rows.map((row) => {
    if (row.shop.reclaimPending) {
      return { shopId: row.shop.shopId, action, eligible: false, skipReason: '授权已失效，待回收' }
    }
    if (!row.shop.profileExists) {
      return { shopId: row.shop.shopId, action, eligible: false, skipReason: '本地 profile 未映射' }
    }
    if (!row.shop.coreReady) {
      return { shopId: row.shop.shopId, action, eligible: false, skipReason: '指纹内核不可用' }
    }
    if (action === 'open' && row.shop.sharedLoginStatus !== 'ready') {
      return { shopId: row.shop.shopId, action, eligible: false, skipReason: '共享会话未就绪' }
    }
    return { shopId: row.shop.shopId, action, eligible: true, skipReason: '' }
  })
}

export function summarizeBatch(candidates: BatchCandidate[], results: Array<{ shopId: string; success: boolean }>): BatchSummary {
  const succeeded = results.filter((item) => item.success).length
  const failed = results.filter((item) => !item.success).length
  const skipped = candidates.filter((item) => !item.eligible).length

  return {
    total: candidates.length,
    eligible: candidates.length - skipped,
    skipped,
    succeeded,
    failed,
  }
}
