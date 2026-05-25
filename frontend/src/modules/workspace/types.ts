export interface WorkspaceSummary {
  status: string
  agentStatus: string
  sessionReady: boolean
  serverReachable: boolean
  antRuntimeReachable: boolean
  activeRunCount: number
  deviceId: string
  deviceStatus: string
}

export interface WorkspaceAuthorizedShop {
  shopId: string
  shopName: string
  platformCode: string
  profileId: string
  instanceId: string
  sharedLoginStatus: string
  sharedLoginStatusLabel: string
  instanceRunning: boolean
  profileExists: boolean
  reclaimPending: boolean
  coreReady: boolean
}

export interface WorkspaceDashboardStats {
  asmShopProfileCount: number
  totalAccounts: number
  readyShopCount: number
  manualAttentionCount: number
  runningInstanceCount: number
}

export interface WorkspaceOpenShopResult {
  shopId: string
  profileId: string
  instanceId: string
  currentUrl: string
  pageTitle: string
  success: boolean
  code: string
  message: string
}

export interface WorkspaceSharedLoginBindSession {
  bindSessionId: string
  traceId: string
  shopId: string
  shopName: string
  sessionType: string
  status: string
  statusLabel: string
  message: string
  manualActionRequired: boolean
  lastObservedUrl: string
  startedAt: string
  expiresAt: string
  completedAt: string
  updatedAt: string
  challengeType: string
}

export interface WorkspaceSharedLoginDetail {
  shopId: string
  shopName: string
  platformCode: string
  sharedLoginStatus: string
  sharedLoginStatusLabel: string
}

export interface WorkspaceSharedLoginActionResult {
  bindSession: WorkspaceSharedLoginBindSession
  detail: WorkspaceSharedLoginDetail
}
