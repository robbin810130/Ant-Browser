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

func TestOpenShopLaunchContextSucceedsForSellerWorkbenchWithoutShopIDInURL(t *testing.T) {
	result := workspace.ClassifyOpenResultForLaunchContext("b2b-222082061706256a1a", workspace.ShopLaunchContext{
		SuccessURLPatterns: []string{"https://work.1688.com/"},
		LoginURLPatterns:   []string{"https://login.1688.com/"},
	}, workspace.OpenRuntimeSnapshot{
		CurrentURL: "https://work.1688.com/?tracelog=login_target_is_blank_1688",
		PageTitle:  "1688-卖家工作台",
	})
	if result.Code != "" || !result.Success {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestPickOpenTargetForLaunchContextPrefersSuccessfulBackendTab(t *testing.T) {
	target, action, ok := workspace.PickOpenTargetForLaunchContext("b2b-222082061706256a1a", workspace.ShopLaunchContext{
		SuccessURLPatterns: []string{"https://work.1688.com/"},
		LoginURLPatterns:   []string{"https://login.1688.com/"},
	}, []workspace.OpenRuntimeTarget{
		{TargetID: "blank-1", CurrentURL: "about:blank", PageTitle: "新标签页"},
		{TargetID: "backend-1", CurrentURL: "https://work.1688.com/?tracelog=login_target_is_blank_1688", PageTitle: "1688-卖家工作台"},
	})
	if !ok {
		t.Fatal("expected target to be selected")
	}
	if target.TargetID != "backend-1" {
		t.Fatalf("unexpected target: %+v", target)
	}
	if action != workspace.OpenTargetActionActivate {
		t.Fatalf("unexpected action: %s", action)
	}
}

func TestPickOpenTargetForLaunchContextReusesBlankTabForNavigation(t *testing.T) {
	target, action, ok := workspace.PickOpenTargetForLaunchContext("b2b-222082061706256a1a", workspace.ShopLaunchContext{
		SuccessURLPatterns: []string{"https://work.1688.com/"},
		LoginURLPatterns:   []string{"https://login.1688.com/"},
	}, []workspace.OpenRuntimeTarget{
		{TargetID: "blank-1", CurrentURL: "about:blank", PageTitle: "新标签页"},
	})
	if !ok {
		t.Fatal("expected target to be selected")
	}
	if target.TargetID != "blank-1" {
		t.Fatalf("unexpected target: %+v", target)
	}
	if action != workspace.OpenTargetActionNavigate {
		t.Fatalf("unexpected action: %s", action)
	}
}

func TestPickOpenTargetForLaunchContextCreatesTargetWhenNoReusableTabExists(t *testing.T) {
	target, action, ok := workspace.PickOpenTargetForLaunchContext("b2b-222082061706256a1a", workspace.ShopLaunchContext{
		SuccessURLPatterns: []string{"https://work.1688.com/"},
		LoginURLPatterns:   []string{"https://login.1688.com/"},
	}, []workspace.OpenRuntimeTarget{
		{TargetID: "other-1", CurrentURL: "https://www.1688.com/", PageTitle: "1688 首页"},
	})
	if !ok {
		t.Fatal("expected target decision to be available")
	}
	if target.TargetID != "" {
		t.Fatalf("expected no reusable target, got: %+v", target)
	}
	if action != workspace.OpenTargetActionCreate {
		t.Fatalf("unexpected action: %s", action)
	}
}

func TestCollectClosableBlankTargetIDsKeepsActiveBackendTarget(t *testing.T) {
	got := workspace.CollectClosableBlankTargetIDs([]workspace.OpenRuntimeTarget{
		{TargetID: "blank-1", CurrentURL: "about:blank", PageTitle: "新标签页"},
		{TargetID: "backend-1", CurrentURL: "https://work.1688.com/?tracelog=login_target_is_blank_1688", PageTitle: "1688-卖家工作台"},
		{TargetID: "blank-2", CurrentURL: "chrome://newtab/", PageTitle: "新标签页"},
	}, "backend-1")
	if len(got) != 2 || got[0] != "blank-1" || got[1] != "blank-2" {
		t.Fatalf("unexpected blank targets: %v", got)
	}
}

func TestResolveWorkspaceTargetURLFallsBackWhenLaunchContextPointsToBlankPage(t *testing.T) {
	for _, input := range []string{
		"about:blank",
		"chrome://newtab/",
		"chrome://new-tab-page/",
		"chrome://new-tab-page",
	} {
		got := workspace.ResolveWorkspaceTargetURL("b2b-222082061706256a1a", input)
		want := "https://work.1688.com/?shopId=b2b-222082061706256a1a"
		if got != want {
			t.Fatalf("unexpected target url for %s: got=%s want=%s", input, got, want)
		}
	}
}
