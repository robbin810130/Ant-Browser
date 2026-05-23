import { WorkspaceShopProfile, WorkspaceShopProfiles } from '../../wailsjs/go/main/App'
import type { ShopProfile, ShopProfileStats } from './types'

export function normalizeShopProfile(input: any): ShopProfile {
  return {
    shopId: String(input?.shopId || ''),
    shopName: String(input?.shopName || ''),
    platformCode: String(input?.platformCode || ''),
    asmStatus: String(input?.asmStatus || 'unavailable'),
    authorizationStatus: String(input?.authorizationStatus || ''),
    ownerName: String(input?.ownerName || ''),
    mainCategory: String(input?.mainCategory || ''),
    dataCompleteness: String(input?.dataCompleteness || 'unknown'),
    lastSyncedAt: String(input?.lastSyncedAt || ''),
    source: String(input?.source || ''),
  }
}

export async function fetchShopProfiles(): Promise<ShopProfile[]> {
  const payload = await WorkspaceShopProfiles()
  return Array.isArray(payload) ? payload.map(normalizeShopProfile) : []
}

export async function fetchShopProfile(shopId: string): Promise<ShopProfile> {
  const payload = await WorkspaceShopProfile(shopId.trim())
  return normalizeShopProfile(payload)
}

export function deriveShopProfileStats(profiles: ShopProfile[]): ShopProfileStats {
  return {
    total: profiles.length,
    asmConnected: profiles.filter((profile) => profile.asmStatus === 'connected').length,
    unavailable: profiles.filter((profile) => profile.asmStatus === 'unavailable').length,
    incomplete: profiles.filter((profile) => profile.dataCompleteness !== 'complete').length,
  }
}
