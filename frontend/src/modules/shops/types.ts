export interface ShopProfile {
  shopId: string
  shopName: string
  platformCode: string
  asmStatus: string
  authorizationStatus: string
  ownerName: string
  mainCategory: string
  dataCompleteness: string
  lastSyncedAt: string
  source: string
}

export interface ShopProfileStats {
  total: number
  asmConnected: number
  unavailable: number
  incomplete: number
}
