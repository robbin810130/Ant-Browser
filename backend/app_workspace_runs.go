package backend

import (
	"ant-chrome/backend/internal/workspace"
	"context"
	"fmt"
)

func (a *App) WorkspaceRuns(query workspace.RunQuery) (*workspace.RunsPayload, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return a.workspaceService.FetchRuns(context.Background(), query)
}

func (a *App) WorkspaceRunEvents(runID string, limit int) (*workspace.RunEventsPayload, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return a.workspaceService.FetchRunEvents(context.Background(), runID, limit)
}

func (a *App) WorkspaceRunEvidence(query workspace.RunQuery) (*workspace.RunEvidenceIndex, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return a.workspaceService.FetchRunEvidence(context.Background(), query)
}
