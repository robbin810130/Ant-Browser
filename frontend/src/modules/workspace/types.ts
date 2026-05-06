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
}

export interface WorkspaceDashboardStats {
  totalAccounts: number
  readyShopCount: number
  manualAttentionCount: number
  runningInstanceCount: number
}
