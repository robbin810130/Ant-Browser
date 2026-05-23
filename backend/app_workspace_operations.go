package backend

import (
	"ant-chrome/backend/internal/workspace"
	"context"
	"fmt"
)

func (a *App) WorkspaceOperationTasks(query workspace.OperationTaskQuery) (*workspace.OperationTasksPayload, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return a.workspaceService.FetchOperationTasks(context.Background(), query)
}
