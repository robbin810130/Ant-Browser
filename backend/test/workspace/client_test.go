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
		LastValidatedAt:   "2026-06-09T09:20:00.000Z",
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
	if projected.LastValidatedAt != "2026-06-09T09:20:00.000Z" {
		t.Fatalf("unexpected last validated at: %s", projected.LastValidatedAt)
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

func TestWorkspaceClientFetchShopProfilesFallsBackToAuthorizedShops(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/local/shops" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "ok",
			"data": map[string]any{
				"items": []map[string]any{{
					"shopId":                 "shop-001",
					"shopName":               "壹级供应链",
					"platformCode":           "alibaba",
					"sharedLoginStatus":      "ready",
					"sharedLoginStatusLabel": "ready",
				}},
				"syncedAt": "2026-05-23T00:00:00Z",
			},
		})
	}))
	defer server.Close()

	client := workspace.NewWorkspaceClient(server.URL, nil)
	profiles, err := client.FetchShopProfiles(context.Background())
	if err != nil {
		t.Fatalf("fetch shop profiles: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	got := profiles[0]
	if got.ShopID != "shop-001" || got.ShopName != "壹级供应链" || got.PlatformCode != "alibaba" {
		t.Fatalf("unexpected profile: %#v", got)
	}
	if got.Source != "authorized_shop_projection" {
		t.Fatalf("expected explicit fallback source, got %s", got.Source)
	}
	if got.ASMStatus != "unavailable" {
		t.Fatalf("expected unavailable ASM status, got %s", got.ASMStatus)
	}
}

func TestWorkspaceClientFetchShopProfilesPrefersASMProfiles(t *testing.T) {
	var hitAuthorizedShops bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/local/shop-profiles":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"items": []map[string]any{{
						"shopId":                   " shop-asm-001 ",
						"shopName":                 " 真实 ASM 店铺 ",
						"asmShopId":                "1001",
						"shopCode":                 "b2b-code-001",
						"shopAlias":                "真实 ASM 店铺",
						"fullShopName":             "阿里巴巴-真实 ASM 店铺",
						"platformCode":             "1688",
						"platformName":             "1688",
						"platformSubtype":          "supplier",
						"asmStatus":                "connected",
						"authorizationStatus":      "valid",
						"authorizationStatusLabel": "已授权",
						"ownerName":                "运营一组",
						"operatorName":             "ASM 运营",
						"operatorUsername":         "asm_operator",
						"businessManagerName":      "ASM 业务",
						"businessManagerUsername":  "asm_manager",
						"department":               "销售部",
						"subCompanyName":           "华南分公司",
						"shopUrl":                  "https://example.1688.com",
						"shopEmail":                "shop@example.com",
						"shopPhone":                "13800000000",
						"legalRepName":             " 张法人 ",
						"businessLicense":          "营业执照-001",
						"unifiedSocialCode":        "91440000123456789X",
						"registeredAddress":        "广东省深圳市",
						"categoryIds":              []any{101, " 202 "},
						"categoryNames":            []string{" 日用百货 "},
						"brandName":                "真实品牌",
						"brandIds":                 []any{303, " brand-1 "},
						"advancedMember":           1,
						"advancedMemberName":       "高级会员",
						"trustPassExpireAt":        "2027-05-23T00:00:00+08:00",
						"jstShopCount":             1,
						"jstShopSummary":           " 聚水潭店铺 ",
						"mabangShopCount":          1,
						"mabangShopSummary":        "马帮店铺",
						"erpShopCount":             1,
						"erpShopSummary":           "ERP店铺",
						"abnormalCount":            1,
						"abnormalSummary":          "资质待复核",
						"tableSource":              "shop_merged",
						"isPush":                   1,
						"mainCategory":             "日用百货",
						"dataCompleteness":         "complete",
						"sourceCreatedAt":          "2026-05-20T10:00:00+08:00",
						"sourceUpdatedAt":          "2026-05-23T10:00:00+08:00",
						"lastSyncedAt":             "2026-05-23T10:00:00+08:00",
						"source":                   "asm",
					}},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/local/shops":
			hitAuthorizedShops = true
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := workspace.NewWorkspaceClient(server.URL, nil)
	profiles, err := client.FetchShopProfiles(context.Background())
	if err != nil {
		t.Fatalf("fetch shop profiles: %v", err)
	}
	if hitAuthorizedShops {
		t.Fatal("expected ASM shop profiles to avoid authorized shop fallback")
	}
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	got := profiles[0]
	if got.ShopID != "shop-asm-001" || got.ShopName != "真实 ASM 店铺" {
		t.Fatalf("unexpected profile normalization: %#v", got)
	}
	if got.Source != "asm" || got.ASMStatus != "connected" || got.AuthorizationStatus != "valid" || got.AuthorizationStatusLabel != "已授权" {
		t.Fatalf("expected real ASM profile fields, got %#v", got)
	}
	if got.ASMShopID != "1001" || got.FullShopName != "阿里巴巴-真实 ASM 店铺" || got.OperatorName != "ASM 运营" || got.BusinessManagerName != "ASM 业务" {
		t.Fatalf("expected expanded ASM profile fields, got %#v", got)
	}
	if got.LegalRepName != "张法人" || got.BusinessLicense != "营业执照-001" || got.UnifiedSocialCode != "91440000123456789X" {
		t.Fatalf("expected ASM qualification fields, got %#v", got)
	}
	if len(got.CategoryIDs) != 2 || got.CategoryIDs[0] != "101" || got.CategoryIDs[1] != "202" || len(got.CategoryNames) != 1 || got.CategoryNames[0] != "日用百货" {
		t.Fatalf("expected normalized ASM category fields, got %#v", got)
	}
	if len(got.BrandIDs) != 2 || got.BrandIDs[0] != "303" || got.BrandIDs[1] != "brand-1" {
		t.Fatalf("expected normalized ASM brand fields, got %#v", got)
	}
	if got.JSTShopCount != 1 || got.JSTShopSummary != "聚水潭店铺" || got.AbnormalSummary != "资质待复核" || got.TableSource != "shop_merged" {
		t.Fatalf("expected ASM integration summary fields, got %#v", got)
	}
}

func TestWorkspaceClientFetchRunsAndEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/local/runs":
			query := r.URL.Query()
			if query.Get("limit") != "20" {
				t.Fatalf("unexpected limit filter: %s", r.URL.RawQuery)
			}
			if query.Get("status") != "failed" {
				t.Fatalf("unexpected status filter: %s", r.URL.RawQuery)
			}
			if query.Get("shopId") != "shop/001" {
				t.Fatalf("unexpected shop filter: %s", r.URL.RawQuery)
			}
			if query.Get("failureCode") != "ANT CORE" {
				t.Fatalf("unexpected failure code filter: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"items": []map[string]any{{
						"runId":       "run-001",
						"taskId":      "run-001",
						"shopId":      "shop-001",
						"taskType":    "open",
						"status":      "succeeded",
						"statusLabel": "succeeded",
						"startedAt":   "2026-05-23T00:00:00Z",
						"finishedAt":  "2026-05-23T00:00:03Z",
						"profileId":   "alibaba:shop-001",
					}},
					"total": 1,
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/local/runs/run-001/events":
			if r.URL.Query().Get("limit") != "50" {
				t.Fatalf("unexpected event limit: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"runId": "run-001",
					"items": []map[string]any{{
						"eventId":   "evt-001",
						"stage":     "succeeded",
						"message":   "shop open succeeded",
						"createdAt": "2026-05-23T00:00:03Z",
					}},
					"total": 1,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := workspace.NewWorkspaceClient(server.URL, nil)
	runs, err := client.FetchRuns(context.Background(), workspace.RunQuery{
		ShopID:      " shop/001 ",
		Status:      " failed ",
		FailureCode: " ANT CORE ",
		Limit:       20,
	})
	if err != nil {
		t.Fatalf("fetch runs: %v", err)
	}
	if len(runs.Items) != 1 || runs.Items[0].RunID != "run-001" {
		t.Fatalf("unexpected runs: %#v", runs)
	}
	events, err := client.FetchRunEvents(context.Background(), "run-001", 50)
	if err != nil {
		t.Fatalf("fetch run events: %v", err)
	}
	if len(events.Items) != 1 || events.Items[0].Stage != "succeeded" {
		t.Fatalf("unexpected events: %#v", events)
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
