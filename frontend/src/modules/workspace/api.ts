import { WorkspaceAuthorizedShops, WorkspaceSummary } from '../../wailsjs/go/main/App'
import type {
  WorkspaceAuthorizedShop,
  WorkspaceDashboardStats,
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
