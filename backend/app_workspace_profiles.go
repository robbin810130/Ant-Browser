package backend

import (
	"ant-chrome/backend/internal/workspace"
	"context"
	"fmt"
)

func (a *App) WorkspaceShopProfiles() ([]workspace.ShopProfileRecord, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return a.workspaceService.FetchShopProfiles(context.Background())
}
