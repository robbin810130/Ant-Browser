package workspace_test

import (
	"context"
	"testing"

	"ant-chrome/backend/internal/workspace"
)

func TestProjectAuthorizedShopToManagedProfile(t *testing.T) {
	projected := workspace.ProjectShopInstance(workspace.ShopRecord{
		ShopID:            "b2b-222082061706256a1a",
		ShopName:          "壹级供应链",
		PlatformCode:      "alibaba",
		SharedLoginStatus: "ready",
	}, workspace.LocalRuntimeState{
		ProfileExists: true,
		InstanceID:    "inst-001",
		Running:       true,
	})

	if projected.ProfileID != "alibaba:b2b-222082061706256a1a" {
		t.Fatalf("unexpected profile id: %s", projected.ProfileID)
	}
	if projected.ShopName != "壹级供应链" {
		t.Fatalf("unexpected shop name: %s", projected.ShopName)
	}
	if projected.SharedLoginStatus != "ready" {
		t.Fatalf("unexpected shared login status: %s", projected.SharedLoginStatus)
	}
}

func TestWorkspaceClientHealthRequiresReachableServer(t *testing.T) {
	client := workspace.NewWorkspaceClient("http://127.0.0.1:1", nil)
	_, err := client.FetchWorkspaceSummary(context.Background())
	if err == nil {
		t.Fatal("expected server unreachable error")
	}
}
