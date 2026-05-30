import type { BatchCandidate, BatchSummary, ExecutableBatchActionKey, WorkbenchRow } from './types'

const batchableActions = new Set<string>(['open', 'bind', 'validate', 'retry', 'refresh'])

export function buildBatchCandidates(rows: WorkbenchRow[], action: ExecutableBatchActionKey): BatchCandidate[] {
  if (!batchableActions.has(action)) {
    throw new Error(`Unsupported batch action: ${action}`)
  }

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
  const eligibleShopIds = new Set(candidates.filter((item) => item.eligible).map((item) => item.shopId))
  const resultByShopId = new Map<string, boolean>()

  results.forEach((item) => {
    if (eligibleShopIds.has(item.shopId)) {
      resultByShopId.set(item.shopId, item.success)
    }
  })

  const resultValues = Array.from(resultByShopId.values())
  const succeeded = resultValues.filter(Boolean).length
  const failed = resultValues.filter((success) => !success).length
  const skipped = candidates.filter((item) => !item.eligible).length

  return {
    total: candidates.length,
    eligible: eligibleShopIds.size,
    skipped,
    succeeded,
    failed,
  }
}
