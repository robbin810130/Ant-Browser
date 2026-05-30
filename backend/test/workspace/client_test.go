package workspace_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestWorkspaceClientFetchOpenContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/local/shops/shop-001/open-context" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "ok",
			"data": map[string]any{
				"openRequestId": "desktop-open-001",
				"shop": map[string]any{
					"shopId":            "shop-001",
					"shopName":          "测试店铺",
					"platformCode":      "alibaba",
					"sharedLoginStatus": "ready",
				},
				"profile": map[string]any{
					"profileId":  "alibaba:shop-001",
					"profileKey": "alibaba:shop-001",
				},
				"launchContext": map[string]any{
					"targetUrl": "https://work.1688.com/?shopId=shop-001",
					"sessionBundle": map[string]any{
						"lastObservedUrl": "https://work.1688.com/?shopId=shop-001",
						"cookies":         []any{},
						"storages":        []any{},
					},
					"successUrlPatterns": []string{"https://work.1688.com/"},
					"loginUrlPatterns":   []string{"https://login.1688.com/"},
				},
				"runtimeConfig": map[string]any{
					"managedMode": true,
				},
			},
		})
	}))
	defer server.Close()

	client := workspace.NewWorkspaceClient(server.URL, nil)
	got, err := client.FetchOpenShopContext(context.Background(), "shop-001")
	if err != nil {
		t.Fatalf("fetch open context: %v", err)
	}

	if got.OpenRequestID != "desktop-open-001" {
		t.Fatalf("unexpected open request id: %s", got.OpenRequestID)
	}
	if got.LaunchContext.TargetURL != "https://work.1688.com/?shopId=shop-001" {
		t.Fatalf("unexpected target url: %s", got.LaunchContext.TargetURL)
	}
	if len(got.LaunchContext.SuccessURLPatterns) != 1 || got.LaunchContext.SuccessURLPatterns[0] != "https://work.1688.com/" {
		t.Fatalf("unexpected success url patterns: %#v", got.LaunchContext.SuccessURLPatterns)
	}
	if !got.RuntimeConfig.ManagedMode {
		t.Fatal("expected managed mode runtime config")
	}
}

func TestWorkspaceClientReportOpenResult(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotBody   workspace.OpenReportRequest
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "ok",
			"data": map[string]any{
				"reported": true,
			},
		})
	}))
	defer server.Close()

	client := workspace.NewWorkspaceClient(server.URL+"/", nil)
	err := client.ReportOpenShopResult(context.Background(), "desktop-open-002", workspace.OpenReportRequest{
		Status: "failed",
		Runtime: &workspace.OpenReportRuntime{
			PID:       2345,
			DebugPort: 9222,
		},
		FailureCode:    "ANT_INSTANCE_OPEN_FAILED",
		FailureMessage: "native open failed",
	})
	if err != nil {
		t.Fatalf("report open result: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("unexpected method: %s", gotMethod)
	}
	if gotPath != "/local/open-requests/desktop-open-002/report" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if gotBody.Status != "failed" || gotBody.Runtime.PID != 2345 || gotBody.Runtime.DebugPort != 9222 {
		t.Fatalf("unexpected report body: %#v", gotBody)
	}
	if !strings.Contains(gotBody.FailureMessage, "native open failed") {
		t.Fatalf("unexpected failure message: %s", gotBody.FailureMessage)
	}
}
