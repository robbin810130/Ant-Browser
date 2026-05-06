package backend

import (
	"ant-chrome/backend/internal/workspace"
	"context"
	"fmt"
	"os"
	"strings"
)

const workspaceBaseURLEnv = "ANT_WORKSPACE_BASE_URL"

func (a *App) WorkspaceSummary() (*workspace.WorkspaceSummary, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return a.workspaceService.FetchSummary(context.Background())
}

func (a *App) WorkspaceAuthorizedShops() ([]workspace.ShopInstanceProjection, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return a.workspaceService.FetchAuthorizedShops(context.Background())
}

func (a *App) initWorkspaceService() {
	baseURL := strings.TrimSpace(os.Getenv(workspaceBaseURLEnv))
	client := workspace.NewWorkspaceClient(baseURL, nil)
	a.workspaceService = workspace.NewService(client, a.browserMgr)
}
