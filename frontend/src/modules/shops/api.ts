import { WorkspaceShopProfile, WorkspaceShopProfiles } from '../../wailsjs/go/main/App'
import { devShopProfiles, useDevWorkspaceFallback } from '../workspace/devData'
import type { ShopProfile, ShopProfileStats } from './types'

export type AsmStatusKind = 'connected' | 'error' | 'unavailable'

export function asmStatusKind(status: string): AsmStatusKind {
  const normalizedStatus = status.trim()
  if (normalizedStatus === 'connected') return 'connected'
  if (normalizedStatus === 'error') return 'error'
  return 'unavailable'
}

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
  if (useDevWorkspaceFallback()) return devShopProfiles
  const payload = await WorkspaceShopProfiles()
  return Array.isArray(payload) ? payload.map(normalizeShopProfile) : []
}

export async function fetchShopProfile(shopId: string): Promise<ShopProfile> {
  const normalizedShopId = shopId.trim()
  if (!normalizedShopId) {
    throw new Error('shop id is required')
  }

  if (useDevWorkspaceFallback()) {
    const profile = devShopProfiles.find((item) => item.shopId === normalizedShopId)
    if (!profile) throw new Error('shop profile not found')
    return profile
  }

  const payload = await WorkspaceShopProfile(normalizedShopId)
  return normalizeShopProfile(payload)
}

export function deriveShopProfileStats(profiles: ShopProfile[]): ShopProfileStats {
  return {
    total: profiles.length,
    asmConnected: profiles.filter((profile) => asmStatusKind(profile.asmStatus) === 'connected').length,
    unavailable: profiles.filter((profile) => asmStatusKind(profile.asmStatus) === 'unavailable').length,
    incomplete: profiles.filter((profile) => profile.dataCompleteness !== 'complete').length,
  }
}
