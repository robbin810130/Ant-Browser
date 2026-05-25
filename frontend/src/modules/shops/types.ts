export interface ShopProfile {
  shopId: string
  shopName: string
  asmShopId: string
  shopCode: string
  shopAlias: string
  fullShopName: string
  platformCode: string
  platformName: string
  platformSubtype: string
  asmStatus: string
  authorizationStatus: string
  authorizationStatusLabel: string
  ownerName: string
  operatorName: string
  operatorUsername: string
  businessManagerName: string
  businessManagerUsername: string
  department: string
  subCompanyName: string
  shopUrl: string
  shopEmail: string
  shopPhone: string
  brandName: string
  advancedMemberName: string
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
