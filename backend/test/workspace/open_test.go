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
	result := workspace.ClassifyOpenResultForShop("b2b-222082061706256a1a", workspace.OpenRuntimeSnapshot{
		CurrentURL: "https://work.1688.com/?shopId=b2b-222082061706256a1a",
		PageTitle:  "壹级供应链 - 1688后台管理",
	})
	if result.Code != "" || !result.Success {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenShopFailsWhenLandingOnDifferentShop(t *testing.T) {
	result := workspace.ClassifyOpenResultForShop("b2b-222082061706256a1a", workspace.OpenRuntimeSnapshot{
		CurrentURL: "https://work.1688.com/?shopId=b2b-0000000000000000000",
		PageTitle:  "其他店铺 - 1688后台管理",
	})
	if result.Code != "ANT_BACKEND_TARGET_MISMATCH" {
		t.Fatalf("unexpected code: %s", result.Code)
	}
	if result.Success {
		t.Fatalf("expected failed result: %+v", result)
	}
}

func TestOpenShopLaunchContextRequiresBackendTitle(t *testing.T) {
	result := workspace.ClassifyOpenResultForLaunchContext("b2b-222082061706256a1a", workspace.ShopLaunchContext{
		SuccessURLPatterns: []string{"https://work.1688.com/"},
		LoginURLPatterns:   []string{"https://login.1688.com/"},
	}, workspace.OpenRuntimeSnapshot{
		CurrentURL: "https://work.1688.com/?shopId=b2b-222082061706256a1a",
		PageTitle:  "1688 工作台首页",
	})
	if result.Success {
		t.Fatalf("expected launch context classification to reject generic page: %+v", result)
	}
	if result.Code != "ANT_INSTANCE_OPEN_FAILED" {
		t.Fatalf("unexpected code: %s", result.Code)
	}
}

func TestOpenShopLaunchContextSucceedsWhenBackendMatched(t *testing.T) {
	result := workspace.ClassifyOpenResultForLaunchContext("b2b-222082061706256a1a", workspace.ShopLaunchContext{
		SuccessURLPatterns: []string{"https://work.1688.com/"},
		LoginURLPatterns:   []string{"https://login.1688.com/"},
	}, workspace.OpenRuntimeSnapshot{
		CurrentURL: "https://work.1688.com/?shopId=b2b-222082061706256a1a",
		PageTitle:  "壹级供应链 - 1688后台管理",
	})
	if result.Code != "" || !result.Success {
		t.Fatalf("unexpected result: %+v", result)
	}
}
