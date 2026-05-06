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
	}
}

func buildProfileID(platformCode string, shopID string) string {
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
