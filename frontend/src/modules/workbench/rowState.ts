import type { ShopRunEvidence } from '../runEvidence'
import type { WorkspaceAuthorizedShop } from '../workspace/types'
import { normalizeAuthorizationStatus } from './statusMatrix'

function parseTime(value = '') {
  const time = Date.parse(value)
  return Number.isFinite(time) ? time : 0
}

function runTime(run: { finishedAt?: string; startedAt?: string } | null) {
  return parseTime(run?.finishedAt || run?.startedAt || '')
}

function isSucceeded(run: { status?: string } | null) {
  return run?.status === 'succeeded' || run?.status === 'completed'
}

function latestCredentialSuccessTime(evidence: ShopRunEvidence) {
  return Math.max(
    isSucceeded(evidence.latestCredential) ? runTime(evidence.latestCredential) : 0,
    isSucceeded(evidence.latestValidation) ? runTime(evidence.latestValidation) : 0,
  )
}

function hasReadySharedLogin(shop: WorkspaceAuthorizedShop) {
  return normalizeAuthorizationStatus(shop.sharedLoginStatus) === 'ready'
}

export function shouldSuppressStaleFailure(shop: WorkspaceAuthorizedShop, evidence: ShopRunEvidence) {
  const failure = evidence.latestFailure
  if (!failure || evidence.activeRun || !hasReadySharedLogin(shop)) return false

  if (failure.taskType === 'bind' || failure.taskType === 'validate') {
    return true
  }

  const credentialSuccessAt = latestCredentialSuccessTime(evidence)
  return credentialSuccessAt > 0 && runTime(failure) <= credentialSuccessAt
}

export function evidenceForWorkbenchRow(shop: WorkspaceAuthorizedShop, evidence: ShopRunEvidence): ShopRunEvidence {
  const withoutStaleLocalFailure = shouldSuppressStaleFailure(shop, evidence)
    ? {
        ...evidence,
        latestFailure: null,
      }
    : evidence

  const withShopOpenFailure = withoutStaleLocalFailure.latestFailure || !shop.lastOpenFailureCode
    ? withoutStaleLocalFailure
    : {
        ...withoutStaleLocalFailure,
        latestFailure: {
          runId: `desktop-open:${shop.shopId}:${shop.lastOpenFailedAt || shop.lastOpenFailureCode}`,
          taskId: '',
          shopId: shop.shopId,
          taskType: 'open',
          status: 'failed',
          statusLabel: '打开失败',
          startedAt: shop.lastOpenFailedAt,
          finishedAt: shop.lastOpenFailedAt,
          profileId: shop.profileId,
          runtime: null,
          bindSessionId: '',
          manualActionRequired: false,
          challengeType: '',
          failureCode: shop.lastOpenFailureCode,
          failureMessage: shop.lastOpenFailureMessage,
        },
      }

  if (!shouldSuppressStaleFailure(shop, withShopOpenFailure)) {
    return withShopOpenFailure
  }

  return {
    ...withShopOpenFailure,
    latestFailure: null,
  }
}
