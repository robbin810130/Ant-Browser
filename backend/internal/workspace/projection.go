package workspace

import "strings"

func ProjectShopInstance(shop ShopRecord, runtime LocalRuntimeState) ShopInstanceProjection {
	return ShopInstanceProjection{
		ShopID:                 shop.ShopID,
		ShopName:               shop.ShopName,
		PlatformCode:           shop.PlatformCode,
		ProfileID:              buildProfileID(shop.PlatformCode, shop.ShopID),
		InstanceID:             runtime.InstanceID,
		SharedLoginStatus:      shop.SharedLoginStatus,
		SharedLoginStatusLabel: shop.SharedLoginStatusLabel,
		InstanceRunning:        runtime.Running,
		ProfileExists:          runtime.ProfileExists,
		ReclaimPending:         runtime.ReclaimPending,
		CoreReady:              runtime.CoreReady,
		LastValidatedAt:        strings.TrimSpace(shop.LastValidatedAt),
		LastOpenedAt:           strings.TrimSpace(shop.LastOpenedAt),
		LastOpenFailureCode:    strings.TrimSpace(shop.LastOpenFailureCode),
		LastOpenFailureMessage: strings.TrimSpace(shop.LastOpenFailureMessage),
		LastOpenFailedAt:       strings.TrimSpace(shop.LastOpenFailedAt),
	}
}

func BuildProfileID(platformCode string, shopID string) string {
	platformCode = strings.TrimSpace(platformCode)
	shopID = strings.TrimSpace(shopID)
	if platformCode == "" {
		return shopID
	}
	if shopID == "" {
		return platformCode
	}
	return platformCode + ":" + shopID
}

func buildProfileID(platformCode string, shopID string) string {
	return BuildProfileID(platformCode, shopID)
}
