package backend

import (
	"ant-chrome/backend/internal/workspace"
	"context"
	"fmt"
	"strings"
)

func (a *App) WorkspaceShopProfiles() ([]workspace.ShopProfileRecord, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	a.ensureWorkspaceAgentReachableForRequest("shop profiles")
	return a.workspaceService.FetchShopProfiles(context.Background())
}

func (a *App) WorkspaceShopProfile(shopID string) (*workspace.ShopProfileRecord, error) {
	shopID = strings.TrimSpace(shopID)
	if shopID == "" {
		return nil, fmt.Errorf("shop id is required")
	}

	profiles, err := a.WorkspaceShopProfiles()
	if err != nil {
		return nil, err
	}
	for i := range profiles {
		if strings.TrimSpace(profiles[i].ShopID) == shopID {
			return &profiles[i], nil
		}
	}
	return nil, fmt.Errorf("shop profile not found: %s", shopID)
}
