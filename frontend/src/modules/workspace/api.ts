import { WorkspaceAuthorizedShops, WorkspaceOpenShop, WorkspaceSummary } from '../../wailsjs/go/main/App'
import type {
  WorkspaceAuthorizedShop,
  WorkspaceDashboardStats,
  WorkspaceOpenShopResult,
  WorkspaceSummary as WorkspaceSummaryModel,
} from './types'

function normalizeSummary(input: any): WorkspaceSummaryModel {
  return {
    status: String(input?.status || ''),
    agentStatus: String(input?.agentStatus || ''),
    sessionReady: Boolean(input?.sessionReady),
    serverReachable: Boolean(input?.serverReachable),
    antRuntimeReachable: Boolean(input?.antRuntimeReachable),
    activeRunCount: Number(input?.activeRunCount || 0),
    deviceId: String(input?.deviceId || ''),
    deviceStatus: String(input?.deviceStatus || ''),
  }
}

function normalizeShop(input: any): WorkspaceAuthorizedShop {
  return {
    shopId: String(input?.shopId || ''),
    shopName: String(input?.shopName || ''),
    platformCode: String(input?.platformCode || ''),
    profileId: String(input?.profileId || ''),
    instanceId: String(input?.instanceId || ''),
    sharedLoginStatus: String(input?.sharedLoginStatus || ''),
    sharedLoginStatusLabel: String(input?.sharedLoginStatusLabel || ''),
    instanceRunning: Boolean(input?.instanceRunning),
    profileExists: Boolean(input?.profileExists),
    reclaimPending: Boolean(input?.reclaimPending),
    coreReady: Boolean(input?.coreReady),
  }
}

export async function fetchWorkspaceSummary(): Promise<WorkspaceSummaryModel> {
  const payload = await WorkspaceSummary()
  return normalizeSummary(payload)
}

export async function fetchWorkspaceAuthorizedShops(): Promise<WorkspaceAuthorizedShop[]> {
  const payload = await WorkspaceAuthorizedShops()
  return Array.isArray(payload) ? payload.map(normalizeShop) : []
}

export function deriveWorkspaceDashboardStats(shops: WorkspaceAuthorizedShop[]): WorkspaceDashboardStats {
  const readyShopCount = shops.filter((shop) => shop.sharedLoginStatus === 'ready').length
  const manualAttentionCount = shops.filter((shop) => shop.sharedLoginStatus !== 'ready').length
  const runningInstanceCount = shops.filter((shop) => shop.instanceRunning).length

  return {
    totalAccounts: shops.length,
    readyShopCount,
    manualAttentionCount,
    runningInstanceCount,
  }
}

function normalizeOpenResult(input: any): WorkspaceOpenShopResult {
  return {
    shopId: String(input?.shopId || ''),
    profileId: String(input?.profileId || ''),
    instanceId: String(input?.instanceId || ''),
    currentUrl: String(input?.currentUrl || ''),
    pageTitle: String(input?.pageTitle || ''),
    success: Boolean(input?.success),
    code: String(input?.code || ''),
    message: String(input?.message || ''),
  }
}

function resolveOpenErrorMessage(code: string, message: string) {
  if (message) return message
  switch (code) {
    case 'ANT_FINGERPRINT_CORE_REQUIRED':
      return '当前环境未配置指纹内核，无法打开 managed 店铺'
    case 'ANT_CORE_NOT_FOUND':
      return '当前店铺绑定的指纹内核不存在'
    case 'ANT_CORE_UNAVAILABLE':
      return '指纹内核当前不可用'
    default:
      return '未能打开目标店铺后台'
  }
}

export async function openWorkspaceShop(shopId: string): Promise<WorkspaceOpenShopResult> {
  const payload = await WorkspaceOpenShop(shopId)
  const result = normalizeOpenResult(payload)
  return result.success
    ? result
    : {
        ...result,
        message: resolveOpenErrorMessage(result.code, result.message),
      }
}
