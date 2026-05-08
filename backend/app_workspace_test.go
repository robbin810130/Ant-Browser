package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/managedinstance"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ant-chrome/backend/internal/workspace"
)

func TestWorkspaceOpenShopDelegatesToManagedInstanceService(t *testing.T) {
	var gotReq managedinstance.OpenRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/local/shops":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"items": []map[string]any{{
						"shopId":            "b2b-222082061706256a1a",
						"shopName":          "壹级供应链",
						"platformCode":      "1688",
						"sharedLoginStatus": "ready",
					}},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/local/shops/b2b-222082061706256a1a/open-context":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"openRequestId": "desktop-open-003",
					"shop": map[string]any{
						"shopId":            "b2b-222082061706256a1a",
						"shopName":          "壹级供应链",
						"platformCode":      "1688",
						"sharedLoginStatus": "ready",
					},
					"profile": map[string]any{
						"profileId": "1688:b2b-222082061706256a1a",
					},
					"launchContext": map[string]any{
						"targetUrl": "https://work.1688.com/?tracelog=login_target_is_blank_1688",
						"sessionBundle": map[string]any{
							"platformCode": "1688",
						},
						"successUrlPatterns": []string{"https://work.1688.com/"},
						"loginUrlPatterns":   []string{"https://login.1688.com/"},
					},
					"runtimeConfig": map[string]any{
						"managedMode": true,
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/local/open-requests/desktop-open-003/report":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data":    map[string]any{},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	previousOpenManagedShop := workspaceOpenManagedShop
	workspaceOpenManagedShop = func(_ *managedinstance.Service, req managedinstance.OpenRequest) (*managedinstance.OpenResult, error) {
		gotReq = req
		return &managedinstance.OpenResult{
			ProfileID:  req.ProfileID,
			Success:    true,
			CurrentURL: "https://work.1688.com/?tracelog=login_target_is_blank_1688",
			PageTitle:  "1688-卖家工作台",
		}, nil
	}
	defer func() {
		workspaceOpenManagedShop = previousOpenManagedShop
	}()

	browserMgr := browser.NewManager(config.DefaultConfig(), t.TempDir())
	app := &App{
		workspaceService:       workspace.NewService(workspace.NewWorkspaceClient(server.URL, nil), nil),
		managedInstanceService: &managedinstance.Service{},
		browserMgr:             browserMgr,
	}

	result, err := app.WorkspaceOpenShop("b2b-222082061706256a1a")
	if err != nil {
		t.Fatalf("workspace open: %v", err)
	}
	if !result.Success {
		t.Fatalf("unexpected open result: %+v", result)
	}
	if gotReq.ShopID != "b2b-222082061706256a1a" {
		t.Fatalf("unexpected shop id: %s", gotReq.ShopID)
	}
	if gotReq.ProfileID != "1688:b2b-222082061706256a1a" {
		t.Fatalf("unexpected profile id: %s", gotReq.ProfileID)
	}
	if !gotReq.ManagedMode {
		t.Fatal("expected managed mode request")
	}
	if !gotReq.PreferVisible {
		t.Fatal("expected visible preference")
	}
	if !strings.Contains(gotReq.TargetURL, "work.1688.com") {
		t.Fatalf("unexpected target url: %s", gotReq.TargetURL)
	}
}

func TestResolveWorkspaceAgentBaseURLFallsBackToDefaultWhenUnset(t *testing.T) {
	t.Setenv("ANT_BROWSER_WORKSPACE_AGENT_BASE_URL", "")
	t.Setenv("AGENT_BASE_URL", "")

	got := resolveWorkspaceAgentBaseURL()

	if got != "http://127.0.0.1:47831" {
		t.Fatalf("unexpected fallback base url: %s", got)
	}
}

func TestResolveWorkspaceAgentBaseURLPrefersExplicitOverride(t *testing.T) {
	t.Setenv("AGENT_BASE_URL", "http://127.0.0.1:47831")
	t.Setenv("ANT_BROWSER_WORKSPACE_AGENT_BASE_URL", " http://127.0.0.1:49000/ ")

	got := resolveWorkspaceAgentBaseURL()

	if got != "http://127.0.0.1:49000" {
		t.Fatalf("unexpected configured base url: %s", got)
	}
}

func TestReportWorkspaceOpenResultReturnsErrorWhenReportFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	app := &App{
		workspaceService: workspace.NewService(workspace.NewWorkspaceClient(server.URL, nil), nil),
	}

	err := app.reportWorkspaceOpenResult(context.Background(), "desktop-open-001", &workspace.OpenShopResult{
		ShopID:     "shop-001",
		ProfileID:  "alibaba:shop-001",
		Success:    true,
		CurrentURL: "https://work.1688.com/?shopId=shop-001",
		PageTitle:  "店铺 - 1688后台管理",
	}, &workspace.OpenReportRuntime{
		PID:       1234,
		DebugPort: 9222,
	})
	if err == nil {
		t.Fatal("expected report error to be returned")
	}
}

func TestReportWorkspaceOpenResultSendsFailurePayload(t *testing.T) {
	var got workspace.OpenReportRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "ok",
			"data":    map[string]any{},
		})
	}))
	defer server.Close()

	app := &App{
		workspaceService: workspace.NewService(workspace.NewWorkspaceClient(server.URL, nil), nil),
	}

	err := app.reportWorkspaceOpenResult(context.Background(), "desktop-open-002", &workspace.OpenShopResult{
		ShopID:    "shop-001",
		ProfileID: "alibaba:shop-001",
		Code:      "ANT_INSTANCE_OPEN_FAILED",
		Message:   "native open failed",
	}, &workspace.OpenReportRuntime{
		PID:       5678,
		DebugPort: 9333,
	})
	if err != nil {
		t.Fatalf("report open result: %v", err)
	}
	if got.Status != "failed" {
		t.Fatalf("unexpected status: %s", got.Status)
	}
	if got.FailureCode != "ANT_INSTANCE_OPEN_FAILED" {
		t.Fatalf("unexpected failure code: %s", got.FailureCode)
	}
	if got.Runtime == nil || got.Runtime.PID != 5678 {
		t.Fatalf("unexpected runtime payload: %#v", got.Runtime)
	}
}

func TestWaitForTargetReadyRetriesUntilTargetAppears(t *testing.T) {
	var listCalls atomic.Int32
	server := startDevToolsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/list":
			call := listCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if call < 3 {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":"target-1","type":"page","title":"1688-卖家工作台","url":"https://work.1688.com/"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app := &App{
		browserMgr: &browser.Manager{
			Profiles: map[string]*BrowserProfile{
				"profile-1": {
					ProfileId:  "profile-1",
					Running:    true,
					DebugReady: true,
					DebugPort:  server.port,
				},
			},
		},
	}

	if err := app.waitForTargetReady("profile-1", "target-1", 500*time.Millisecond); err != nil {
		t.Fatalf("waitForTargetReady 返回错误: %v", err)
	}
	if listCalls.Load() < 3 {
		t.Fatalf("expected polling to retry, got=%d", listCalls.Load())
	}
}
