package workspace_test

import (
	"testing"

	"ant-chrome/backend/internal/workspace"
)

func TestOpenShopFailsWhenLandingOnLoginPage(t *testing.T) {
	result := workspace.ClassifyOpenResult(workspace.OpenRuntimeSnapshot{
		CurrentURL: "https://login.1688.com/",
		PageTitle:  "阿里巴巴登录",
	})
	if result.Code != "ANT_BACKEND_LOGIN_REQUIRED" {
		t.Fatalf("unexpected code: %s", result.Code)
	}
	if result.Success {
		t.Fatalf("expected failed result: %+v", result)
	}
}

func TestOpenShopSucceedsWhenTargetBackendMatched(t *testing.T) {
	result := workspace.ClassifyOpenResult(workspace.OpenRuntimeSnapshot{
		CurrentURL: "https://work.1688.com/?shopId=b2b-222082061706256a1a",
		PageTitle:  "壹级供应链 - 1688后台管理",
	})
	if result.Code != "" || !result.Success {
		t.Fatalf("unexpected result: %+v", result)
	}
}
