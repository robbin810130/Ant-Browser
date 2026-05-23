# ASM 店铺运营闭环 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 Ant Browser 桌面端升级为 ASM 店铺运营控制台，具备店铺资料中心、店铺工作台、运营任务中心和 runs/events 运行证据层。

**Architecture:** 后端保持 Wails API 作为唯一桌面边界，新增 Workspace shop profile、run evidence、operation task 的薄适配层；前端按 `shops`、`workbench`、`operations`、`runEvidence` 拆分业务模块。执行动作继续复用现有 open/bind/validate 链路，新增证据派生、失败修复映射和安全批量编排。

**Tech Stack:** Go/Wails backend、React 18、TypeScript、Zustand、Tailwind CSS、lucide-react、本地 Workspace agent HTTP contract。

---

## Scope Check

这个 spec 涵盖三个界面模块和一个共享证据层，但它们不是独立产品线：店铺资料中心、店铺工作台、运营任务中心都依赖同一组 shop profile、authorized shop projection、run evidence。计划按可独立提交的任务拆分，每个任务都能保持系统可构建。

本计划不实现完整选品、铺货、AI 日报、外部持久化任务调度。运营任务中心在本阶段只落骨架和状态边界。

## File Structure

### Backend

- Modify: `backend/internal/workspace/types.go`
  - 新增 shop profile、operation task、run evidence DTO。
- Modify: `backend/internal/workspace/client.go`
  - 新增 local agent run APIs 和 shop profile fallback adapter。
- Create: `backend/internal/workspace/evidence.go`
  - 运行证据派生：按 shop/task type 找最近 run、活跃 run、最近失败。
- Create: `backend/internal/workspace/evidence_test.go`
  - 证据派生测试。
- Modify: `backend/test/workspace/client_test.go`
  - Workspace client shop profiles / runs / events contract 测试。
- Create: `backend/app_workspace_profiles.go`
  - Wails-facing shop profile APIs。
- Create: `backend/app_workspace_runs.go`
  - Wails-facing run evidence APIs。
- Modify: `backend/app_workspace_test.go`
  - App API contract 测试。

### Frontend

- Create: `frontend/src/modules/runEvidence/types.ts`
  - RunRecord、RunEvent、RunEvidenceIndex、派生类型。
- Create: `frontend/src/modules/runEvidence/api.ts`
  - Wails API normalize/fetch。
- Create: `frontend/src/modules/runEvidence/selectors.ts`
  - 最近 open/validate/bind、失败、活跃 run 派生。
- Create: `frontend/src/modules/runEvidence/components/RunTimeline.tsx`
  - 事件时间线组件。
- Create: `frontend/src/modules/shops/types.ts`
  - ShopProfile、ShopProfileDetail、ASM 状态类型。
- Create: `frontend/src/modules/shops/api.ts`
  - Shop profile normalize/fetch。
- Create: `frontend/src/modules/shops/pages/ShopProfileListPage.tsx`
  - 店铺资料列表。
- Create: `frontend/src/modules/shops/pages/ShopProfileDetailPage.tsx`
  - 店铺资料详情。
- Create: `frontend/src/modules/workbench/types.ts`
  - Workbench row、queue、action、batch result 类型。
- Create: `frontend/src/modules/workbench/recovery.ts`
  - 失败到修复动作映射。
- Create: `frontend/src/modules/workbench/batch.ts`
  - 批量 eligibility、跳过原因、并发限制编排。
- Create: `frontend/src/modules/workbench/WorkbenchPage.tsx`
  - 替代当前 BrowserList 的新工作台页面。
- Create: `frontend/src/modules/workbench/components/WorkbenchQueues.tsx`
- Create: `frontend/src/modules/workbench/components/WorkbenchTable.tsx`
- Create: `frontend/src/modules/workbench/components/ShopWorkbenchDrawer.tsx`
- Create: `frontend/src/modules/operations/types.ts`
  - OperationTask skeleton 类型。
- Create: `frontend/src/modules/operations/api.ts`
  - 初版任务骨架数据 adapter。
- Create: `frontend/src/modules/operations/pages/OperationTaskCenterPage.tsx`
  - 全局运营任务中心骨架。
- Modify: `frontend/src/App.tsx`
  - 新增 `/shops`、`/shops/:shopId`、`/workbench`、`/operations` 路由；保留 `/browser/list` redirect。
- Modify: `frontend/src/config/project.config.ts`
  - 导航改为业务优先。

### Generated Bindings

- Modify generated: `frontend/src/wailsjs/go/main/App.js`
- Modify generated: `frontend/src/wailsjs/go/main/App.d.ts`
- Modify generated: `frontend/src/wailsjs/go/models.ts`

这些文件通过 `rtk wails generate` 生成，不手写。

---

## Task 1: Backend Workspace DTOs And Client Adapters

**Files:**
- Modify: `backend/internal/workspace/types.go`
- Modify: `backend/internal/workspace/client.go`
- Modify: `backend/test/workspace/client_test.go`

- [ ] **Step 1: Write failing tests for shop profile fallback and run APIs**

Add these tests to `backend/test/workspace/client_test.go`:

```go
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

func TestWorkspaceClientFetchRunsAndEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/local/runs":
			if r.URL.Query().Get("shopId") != "shop-001" {
				t.Fatalf("unexpected shop filter: %s", r.URL.RawQuery)
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
	runs, err := client.FetchRuns(context.Background(), workspace.RunQuery{ShopID: "shop-001", Limit: 20})
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
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
rtk go test ./backend/test/workspace -count=1
```

Expected: FAIL with missing `FetchShopProfiles`, `FetchRuns`, `FetchRunEvents`, `RunQuery` types.

- [ ] **Step 3: Add backend DTOs**

Append these types to `backend/internal/workspace/types.go`:

```go
type ShopProfileRecord struct {
	ShopID              string `json:"shopId"`
	ShopName            string `json:"shopName"`
	PlatformCode        string `json:"platformCode"`
	ASMStatus           string `json:"asmStatus"`
	AuthorizationStatus string `json:"authorizationStatus"`
	OwnerName           string `json:"ownerName"`
	MainCategory        string `json:"mainCategory"`
	DataCompleteness    string `json:"dataCompleteness"`
	LastSyncedAt        string `json:"lastSyncedAt"`
	Source              string `json:"source"`
}

type RunQuery struct {
	Limit       int
	Status      string
	ShopID      string
	FailureCode string
}

type RunRuntime struct {
	PID        int    `json:"pid"`
	DebugPort  int    `json:"debugPort"`
	CurrentURL string `json:"currentUrl"`
	PageTitle  string `json:"pageTitle"`
	TargetURL  string `json:"targetUrl"`
}

type RunRecord struct {
	RunID                string      `json:"runId"`
	TaskID               string      `json:"taskId"`
	ShopID               string      `json:"shopId"`
	TaskType             string      `json:"taskType"`
	Status               string      `json:"status"`
	StatusLabel          string      `json:"statusLabel"`
	StartedAt            string      `json:"startedAt"`
	FinishedAt           string      `json:"finishedAt"`
	ProfileID            string      `json:"profileId"`
	Runtime              *RunRuntime `json:"runtime,omitempty"`
	BindSessionID        string      `json:"bindSessionId"`
	ManualActionRequired bool        `json:"manualActionRequired"`
	ChallengeType        string      `json:"challengeType"`
	FailureCode          string      `json:"failureCode"`
	FailureMessage       string      `json:"failureMessage"`
}

type RunsPayload struct {
	Items []RunRecord `json:"items"`
	Total int         `json:"total"`
}

type RunEvent struct {
	EventID   string         `json:"eventId"`
	Stage     string         `json:"stage"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	CreatedAt string         `json:"createdAt"`
}

type RunEventsPayload struct {
	RunID string     `json:"runId"`
	Items []RunEvent `json:"items"`
	Total int        `json:"total"`
}
```

- [ ] **Step 4: Add client methods**

Add to `backend/internal/workspace/client.go`:

```go
func (c *WorkspaceClient) FetchShopProfiles(ctx context.Context) ([]ShopProfileRecord, error) {
	shops, err := c.FetchAuthorizedShops(ctx)
	if err != nil {
		return nil, err
	}
	profiles := make([]ShopProfileRecord, 0, len(shops))
	for _, shop := range shops {
		profiles = append(profiles, ShopProfileRecord{
			ShopID:              strings.TrimSpace(shop.ShopID),
			ShopName:            strings.TrimSpace(shop.ShopName),
			PlatformCode:        strings.TrimSpace(shop.PlatformCode),
			ASMStatus:           "unavailable",
			AuthorizationStatus: strings.TrimSpace(shop.SharedLoginStatus),
			DataCompleteness:    "unknown",
			Source:              "authorized_shop_projection",
		})
	}
	return profiles, nil
}

func (c *WorkspaceClient) FetchRuns(ctx context.Context, query RunQuery) (*RunsPayload, error) {
	params := make([]string, 0, 4)
	if query.Limit > 0 {
		params = append(params, "limit="+url.QueryEscape(fmt.Sprintf("%d", query.Limit)))
	}
	if strings.TrimSpace(query.Status) != "" {
		params = append(params, "status="+url.QueryEscape(strings.TrimSpace(query.Status)))
	}
	if strings.TrimSpace(query.ShopID) != "" {
		params = append(params, "shopId="+url.QueryEscape(strings.TrimSpace(query.ShopID)))
	}
	if strings.TrimSpace(query.FailureCode) != "" {
		params = append(params, "failureCode="+url.QueryEscape(strings.TrimSpace(query.FailureCode)))
	}
	path := "/local/runs"
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}
	var payload RunsPayload
	if err := c.getJSON(ctx, path, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *WorkspaceClient) FetchRunEvents(ctx context.Context, runID string, limit int) (*RunEventsPayload, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	path := fmt.Sprintf("/local/runs/%s/events", urlPathEscape(runID))
	if limit > 0 {
		path += "?limit=" + url.QueryEscape(fmt.Sprintf("%d", limit))
	}
	var payload RunEventsPayload
	if err := c.getJSON(ctx, path, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}
```

Also add `net/url` to imports.

- [ ] **Step 5: Run tests**

Run:

```bash
rtk go test ./backend/test/workspace -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add backend/internal/workspace/types.go backend/internal/workspace/client.go backend/test/workspace/client_test.go
rtk git commit -m "Add workspace shop profile and run adapters"
```

---

## Task 2: Backend Run Evidence Derivation

**Files:**
- Create: `backend/internal/workspace/evidence.go`
- Create: `backend/internal/workspace/evidence_test.go`

- [ ] **Step 1: Write failing evidence tests**

Create `backend/internal/workspace/evidence_test.go`:

```go
package workspace

import "testing"

func TestBuildRunEvidenceIndexSelectsLatestByShopAndTask(t *testing.T) {
	index := BuildRunEvidenceIndex([]RunRecord{
		{RunID: "old-open", ShopID: "shop-001", TaskType: "open", Status: "failed", StartedAt: "2026-05-23T00:00:00Z", FailureCode: "ANT_OPEN_FAILED"},
		{RunID: "new-open", ShopID: "shop-001", TaskType: "open", Status: "succeeded", StartedAt: "2026-05-23T00:01:00Z"},
		{RunID: "validate-1", ShopID: "shop-001", TaskType: "validate", Status: "failed", StartedAt: "2026-05-23T00:02:00Z", FailureCode: "VALIDATION_FAILED"},
	})

	shop := index.ByShop["shop-001"]
	if shop.LatestOpen == nil || shop.LatestOpen.RunID != "new-open" {
		t.Fatalf("unexpected latest open: %#v", shop.LatestOpen)
	}
	if shop.LatestValidation == nil || shop.LatestValidation.RunID != "validate-1" {
		t.Fatalf("unexpected latest validation: %#v", shop.LatestValidation)
	}
	if shop.LatestFailure == nil || shop.LatestFailure.RunID != "validate-1" {
		t.Fatalf("unexpected latest failure: %#v", shop.LatestFailure)
	}
}

func TestBuildRunEvidenceIndexTracksActiveRuns(t *testing.T) {
	index := BuildRunEvidenceIndex([]RunRecord{
		{RunID: "run-active", ShopID: "shop-001", TaskType: "open", Status: "launching", StartedAt: "2026-05-23T00:01:00Z"},
		{RunID: "run-done", ShopID: "shop-001", TaskType: "open", Status: "succeeded", StartedAt: "2026-05-23T00:00:00Z"},
	})

	shop := index.ByShop["shop-001"]
	if shop.ActiveRun == nil || shop.ActiveRun.RunID != "run-active" {
		t.Fatalf("unexpected active run: %#v", shop.ActiveRun)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
rtk go test ./backend/internal/workspace -count=1
```

Expected: FAIL with missing `BuildRunEvidenceIndex`.

- [ ] **Step 3: Add evidence derivation**

Create `backend/internal/workspace/evidence.go`:

```go
package workspace

import "time"

type ShopRunEvidence struct {
	LatestOpen       *RunRecord `json:"latestOpen,omitempty"`
	LatestCredential *RunRecord `json:"latestCredential,omitempty"`
	LatestValidation *RunRecord `json:"latestValidation,omitempty"`
	LatestFailure    *RunRecord `json:"latestFailure,omitempty"`
	ActiveRun        *RunRecord `json:"activeRun,omitempty"`
}

type RunEvidenceIndex struct {
	ByShop map[string]ShopRunEvidence `json:"byShop"`
}

func BuildRunEvidenceIndex(runs []RunRecord) RunEvidenceIndex {
	index := RunEvidenceIndex{ByShop: map[string]ShopRunEvidence{}}
	for i := range runs {
		run := runs[i]
		if run.ShopID == "" {
			continue
		}
		current := index.ByShop[run.ShopID]
		switch run.TaskType {
		case "open":
			current.LatestOpen = newerRun(current.LatestOpen, &run)
		case "bind":
			current.LatestCredential = newerRun(current.LatestCredential, &run)
		case "validate":
			current.LatestValidation = newerRun(current.LatestValidation, &run)
		}
		if isFailureRun(run) {
			current.LatestFailure = newerRun(current.LatestFailure, &run)
		}
		if isActiveRun(run) {
			current.ActiveRun = newerRun(current.ActiveRun, &run)
		}
		index.ByShop[run.ShopID] = current
	}
	return index
}

func isFailureRun(run RunRecord) bool {
	return run.Status == "failed" || run.FailureCode != "" || run.FailureMessage != ""
}

func isActiveRun(run RunRecord) bool {
	switch run.Status {
	case "succeeded", "failed":
		return false
	default:
		return run.Status != ""
	}
}

func newerRun(current *RunRecord, candidate *RunRecord) *RunRecord {
	if candidate == nil {
		return current
	}
	if current == nil {
		cloned := *candidate
		return &cloned
	}
	if parseRunTime(candidate.StartedAt).After(parseRunTime(current.StartedAt)) {
		cloned := *candidate
		return &cloned
	}
	return current
}

func parseRunTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
```

- [ ] **Step 4: Run tests**

```bash
rtk go test ./backend/internal/workspace -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add backend/internal/workspace/evidence.go backend/internal/workspace/evidence_test.go
rtk git commit -m "Derive workspace run evidence"
```

---

## Task 3: Wails APIs For Shop Profiles And Runs

**Files:**
- Create: `backend/app_workspace_profiles.go`
- Create: `backend/app_workspace_runs.go`
- Modify: `backend/app_workspace_test.go`

- [ ] **Step 1: Write failing App API tests**

Append to `backend/app_workspace_test.go`:

```go
func TestWorkspaceShopProfilesReturnsFallbackProfiles(t *testing.T) {
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
					"shopId":            "shop-001",
					"shopName":          "壹级供应链",
					"platformCode":      "alibaba",
					"sharedLoginStatus": "ready",
				}},
			},
		})
	}))
	defer server.Close()

	app := &App{
		workspaceService: workspace.NewService(workspace.NewWorkspaceClient(server.URL, nil), nil, nil),
	}
	profiles, err := app.WorkspaceShopProfiles()
	if err != nil {
		t.Fatalf("WorkspaceShopProfiles: %v", err)
	}
	if len(profiles) != 1 || profiles[0].ShopID != "shop-001" {
		t.Fatalf("unexpected profiles: %#v", profiles)
	}
}

func TestWorkspaceRunsAndEventsReturnLocalAgentEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/local/runs":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"items": []map[string]any{{
						"runId":    "run-001",
						"taskId":   "run-001",
						"shopId":   "shop-001",
						"taskType": "open",
						"status":   "succeeded",
					}},
					"total": 1,
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/local/runs/run-001/events":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"runId": "run-001",
					"items": []map[string]any{{
						"eventId": "evt-001",
						"stage":   "succeeded",
						"message": "shop open succeeded",
					}},
					"total": 1,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app := &App{
		workspaceService: workspace.NewService(workspace.NewWorkspaceClient(server.URL, nil), nil, nil),
	}
	runs, err := app.WorkspaceRuns(workspace.RunQuery{ShopID: "shop-001", Limit: 10})
	if err != nil {
		t.Fatalf("WorkspaceRuns: %v", err)
	}
	if len(runs.Items) != 1 || runs.Items[0].RunID != "run-001" {
		t.Fatalf("unexpected runs: %#v", runs)
	}
	events, err := app.WorkspaceRunEvents("run-001", 20)
	if err != nil {
		t.Fatalf("WorkspaceRunEvents: %v", err)
	}
	if len(events.Items) != 1 || events.Items[0].EventID != "evt-001" {
		t.Fatalf("unexpected events: %#v", events)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

```bash
rtk go test ./backend -run 'TestWorkspaceShopProfiles|TestWorkspaceRunsAndEvents' -count=1
```

Expected: FAIL with missing App methods.

- [ ] **Step 3: Add App APIs**

Create `backend/app_workspace_profiles.go`:

```go
package backend

import (
	"ant-chrome/backend/internal/workspace"
	"context"
	"fmt"
	"strings"
)

func (a *App) WorkspaceShopProfiles() ([]workspace.ShopProfileRecord, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return a.workspaceService.FetchShopProfiles(context.Background())
}

func (a *App) WorkspaceShopProfile(shopID string) (*workspace.ShopProfileRecord, error) {
	shopID = strings.TrimSpace(shopID)
	if shopID == "" {
		return nil, fmt.Errorf("shop id is required")
	}
	profiles, err := a.WorkspaceShopProfiles()
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		if strings.TrimSpace(profile.ShopID) == shopID {
			cloned := profile
			return &cloned, nil
		}
	}
	return nil, fmt.Errorf("shop profile not found: %s", shopID)
}
```

Create `backend/app_workspace_runs.go`:

```go
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
```

Add these service passthroughs to `backend/internal/workspace/client.go`:

```go
func (s *WorkspaceService) FetchShopProfiles(ctx context.Context) ([]ShopProfileRecord, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return s.client.FetchShopProfiles(ctx)
}

func (s *WorkspaceService) FetchRuns(ctx context.Context, query RunQuery) (*RunsPayload, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return s.client.FetchRuns(ctx, query)
}

func (s *WorkspaceService) FetchRunEvents(ctx context.Context, runID string, limit int) (*RunEventsPayload, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return s.client.FetchRunEvents(ctx, runID, limit)
}
```

- [ ] **Step 4: Run backend tests**

```bash
rtk go test ./backend ./backend/internal/workspace ./backend/test/workspace -count=1
```

Expected: PASS.

- [ ] **Step 5: Regenerate Wails bindings**

```bash
rtk wails generate
```

Expected: `frontend/src/wailsjs/go/main/App.d.ts`, `App.js`, and `models.ts` include `WorkspaceShopProfiles`, `WorkspaceShopProfile`, `WorkspaceRuns`, `WorkspaceRunEvents`.

- [ ] **Step 6: Commit**

```bash
rtk git add backend/app_workspace_profiles.go backend/app_workspace_runs.go backend/internal/workspace/client.go backend/app_workspace_test.go frontend/src/wailsjs/go/main/App.d.ts frontend/src/wailsjs/go/main/App.js frontend/src/wailsjs/go/models.ts
rtk git commit -m "Expose workspace shop profile and run APIs"
```

---

## Task 4: Frontend Run Evidence Module

**Files:**
- Create: `frontend/src/modules/runEvidence/types.ts`
- Create: `frontend/src/modules/runEvidence/api.ts`
- Create: `frontend/src/modules/runEvidence/selectors.ts`
- Create: `frontend/src/modules/runEvidence/components/RunTimeline.tsx`
- Create: `frontend/src/modules/runEvidence/index.ts`

- [ ] **Step 1: Add run evidence types**

Create `frontend/src/modules/runEvidence/types.ts`:

```ts
export type RunTaskType = 'open' | 'bind' | 'validate' | 'diagnose' | 'retry' | string
export type RunStatus = 'accepted' | 'authorizing' | 'launching' | 'awaiting_verification' | 'capturing' | 'succeeded' | 'failed' | string

export interface RunRuntime {
  pid: number
  debugPort: number
  currentUrl: string
  pageTitle: string
  targetUrl: string
}

export interface RunRecord {
  runId: string
  taskId: string
  shopId: string
  taskType: RunTaskType
  status: RunStatus
  statusLabel: string
  startedAt: string
  finishedAt: string
  profileId: string
  runtime: RunRuntime | null
  bindSessionId: string
  manualActionRequired: boolean
  challengeType: string
  failureCode: string
  failureMessage: string
}

export interface RunEvent {
  eventId: string
  stage: string
  message: string
  details: Record<string, unknown>
  createdAt: string
}

export interface RunsPayload {
  items: RunRecord[]
  total: number
}

export interface RunEventsPayload {
  runId: string
  items: RunEvent[]
  total: number
}

export interface ShopRunEvidence {
  latestOpen: RunRecord | null
  latestCredential: RunRecord | null
  latestValidation: RunRecord | null
  latestFailure: RunRecord | null
  activeRun: RunRecord | null
}

export interface RunEvidenceIndex {
  byShop: Record<string, ShopRunEvidence>
}
```

- [ ] **Step 2: Add API normalizers**

Create `frontend/src/modules/runEvidence/api.ts`:

```ts
import { WorkspaceRunEvents, WorkspaceRuns } from '../../wailsjs/go/main/App'
import type { RunEvent, RunEventsPayload, RunRecord, RunsPayload } from './types'

function normalizeRuntime(input: any): RunRecord['runtime'] {
  if (!input) return null
  return {
    pid: Number(input?.pid || 0),
    debugPort: Number(input?.debugPort || 0),
    currentUrl: String(input?.currentUrl || ''),
    pageTitle: String(input?.pageTitle || ''),
    targetUrl: String(input?.targetUrl || ''),
  }
}

export function normalizeRun(input: any): RunRecord {
  return {
    runId: String(input?.runId || ''),
    taskId: String(input?.taskId || ''),
    shopId: String(input?.shopId || ''),
    taskType: String(input?.taskType || ''),
    status: String(input?.status || ''),
    statusLabel: String(input?.statusLabel || ''),
    startedAt: String(input?.startedAt || ''),
    finishedAt: String(input?.finishedAt || ''),
    profileId: String(input?.profileId || ''),
    runtime: normalizeRuntime(input?.runtime),
    bindSessionId: String(input?.bindSessionId || ''),
    manualActionRequired: Boolean(input?.manualActionRequired),
    challengeType: String(input?.challengeType || ''),
    failureCode: String(input?.failureCode || ''),
    failureMessage: String(input?.failureMessage || ''),
  }
}

export function normalizeRunEvent(input: any): RunEvent {
  return {
    eventId: String(input?.eventId || ''),
    stage: String(input?.stage || ''),
    message: String(input?.message || ''),
    details: input?.details && typeof input.details === 'object' ? input.details : {},
    createdAt: String(input?.createdAt || ''),
  }
}

export async function fetchWorkspaceRuns(query: { shopId?: string; status?: string; failureCode?: string; limit?: number } = {}): Promise<RunsPayload> {
  const payload = await WorkspaceRuns({
    ShopID: query.shopId || '',
    Status: query.status || '',
    FailureCode: query.failureCode || '',
    Limit: query.limit || 50,
  } as any)
  const items = Array.isArray(payload?.items) ? payload.items.map(normalizeRun) : []
  return { items, total: Number(payload?.total || items.length) }
}

export async function fetchWorkspaceRunEvents(runId: string, limit = 50): Promise<RunEventsPayload> {
  const payload = await WorkspaceRunEvents(runId.trim(), limit)
  const items = Array.isArray(payload?.items) ? payload.items.map(normalizeRunEvent) : []
  return {
    runId: String(payload?.runId || runId),
    items,
    total: Number(payload?.total || items.length),
  }
}
```

- [ ] **Step 3: Add selectors**

Create `frontend/src/modules/runEvidence/selectors.ts`:

```ts
import type { RunEvidenceIndex, RunRecord, ShopRunEvidence } from './types'

const terminalStatuses = new Set(['succeeded', 'failed'])

function parseTime(value: string) {
  const time = Date.parse(value || '')
  return Number.isFinite(time) ? time : 0
}

function newer(current: RunRecord | null, candidate: RunRecord) {
  if (!current) return candidate
  return parseTime(candidate.startedAt) > parseTime(current.startedAt) ? candidate : current
}

function emptyEvidence(): ShopRunEvidence {
  return {
    latestOpen: null,
    latestCredential: null,
    latestValidation: null,
    latestFailure: null,
    activeRun: null,
  }
}

export function buildRunEvidenceIndex(runs: RunRecord[]): RunEvidenceIndex {
  const byShop: RunEvidenceIndex['byShop'] = {}
  runs.forEach((run) => {
    if (!run.shopId) return
    const current = byShop[run.shopId] || emptyEvidence()
    if (run.taskType === 'open') current.latestOpen = newer(current.latestOpen, run)
    if (run.taskType === 'bind') current.latestCredential = newer(current.latestCredential, run)
    if (run.taskType === 'validate') current.latestValidation = newer(current.latestValidation, run)
    if (run.status === 'failed' || run.failureCode || run.failureMessage) {
      current.latestFailure = newer(current.latestFailure, run)
    }
    if (run.status && !terminalStatuses.has(run.status)) {
      current.activeRun = newer(current.activeRun, run)
    }
    byShop[run.shopId] = current
  })
  return { byShop }
}

export function evidenceForShop(index: RunEvidenceIndex, shopId: string): ShopRunEvidence {
  return index.byShop[shopId] || emptyEvidence()
}
```

- [ ] **Step 4: Add timeline component**

Create `frontend/src/modules/runEvidence/components/RunTimeline.tsx`:

```tsx
import { AlertCircle, CheckCircle2, CircleDot } from 'lucide-react'
import type { RunEvent } from '../types'

function iconForStage(stage: string) {
  if (stage === 'succeeded' || stage === 'completed') return <CheckCircle2 className="h-4 w-4 text-emerald-500" />
  if (stage === 'failed' || stage === 'expired') return <AlertCircle className="h-4 w-4 text-red-500" />
  return <CircleDot className="h-4 w-4 text-sky-500" />
}

export function RunTimeline({ events }: { events: RunEvent[] }) {
  if (events.length === 0) {
    return <div className="py-6 text-center text-sm text-[var(--color-text-muted)]">暂无运行事件</div>
  }

  return (
    <div className="space-y-3">
      {events.map((event) => (
        <div key={event.eventId || `${event.stage}-${event.createdAt}`} className="flex gap-3 rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] p-3">
          <div className="mt-0.5">{iconForStage(event.stage)}</div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm font-medium text-[var(--color-text-primary)]">{event.stage || 'event'}</span>
              <span className="text-xs text-[var(--color-text-muted)]">{event.createdAt || '-'}</span>
            </div>
            <p className="mt-1 break-words text-sm text-[var(--color-text-secondary)]">{event.message || '-'}</p>
          </div>
        </div>
      ))}
    </div>
  )
}
```

- [ ] **Step 5: Add module barrel**

Create `frontend/src/modules/runEvidence/index.ts`:

```ts
export * from './api'
export * from './selectors'
export * from './types'
export { RunTimeline } from './components/RunTimeline'
```

- [ ] **Step 6: Typecheck frontend**

```bash
rtk npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
rtk git add frontend/src/modules/runEvidence
rtk git commit -m "Add frontend run evidence module"
```

---

## Task 5: Shop Profile Center

**Files:**
- Create: `frontend/src/modules/shops/types.ts`
- Create: `frontend/src/modules/shops/api.ts`
- Create: `frontend/src/modules/shops/pages/ShopProfileListPage.tsx`
- Create: `frontend/src/modules/shops/pages/ShopProfileDetailPage.tsx`
- Create: `frontend/src/modules/shops/index.ts`

- [ ] **Step 1: Add shop profile types**

Create `frontend/src/modules/shops/types.ts`:

```ts
export interface ShopProfile {
  shopId: string
  shopName: string
  platformCode: string
  asmStatus: string
  authorizationStatus: string
  ownerName: string
  mainCategory: string
  dataCompleteness: string
  lastSyncedAt: string
  source: string
}

export interface ShopProfileStats {
  total: number
  asmConnected: number
  unavailable: number
  incomplete: number
}
```

- [ ] **Step 2: Add shop profile API**

Create `frontend/src/modules/shops/api.ts`:

```ts
import { WorkspaceShopProfile, WorkspaceShopProfiles } from '../../wailsjs/go/main/App'
import type { ShopProfile, ShopProfileStats } from './types'

export function normalizeShopProfile(input: any): ShopProfile {
  return {
    shopId: String(input?.shopId || ''),
    shopName: String(input?.shopName || ''),
    platformCode: String(input?.platformCode || ''),
    asmStatus: String(input?.asmStatus || 'unavailable'),
    authorizationStatus: String(input?.authorizationStatus || ''),
    ownerName: String(input?.ownerName || ''),
    mainCategory: String(input?.mainCategory || ''),
    dataCompleteness: String(input?.dataCompleteness || 'unknown'),
    lastSyncedAt: String(input?.lastSyncedAt || ''),
    source: String(input?.source || ''),
  }
}

export async function fetchShopProfiles(): Promise<ShopProfile[]> {
  const payload = await WorkspaceShopProfiles()
  return Array.isArray(payload) ? payload.map(normalizeShopProfile) : []
}

export async function fetchShopProfile(shopId: string): Promise<ShopProfile> {
  const payload = await WorkspaceShopProfile(shopId.trim())
  return normalizeShopProfile(payload)
}

export function deriveShopProfileStats(profiles: ShopProfile[]): ShopProfileStats {
  return {
    total: profiles.length,
    asmConnected: profiles.filter((profile) => profile.asmStatus === 'connected').length,
    unavailable: profiles.filter((profile) => profile.asmStatus === 'unavailable').length,
    incomplete: profiles.filter((profile) => profile.dataCompleteness !== 'complete').length,
  }
}
```

- [ ] **Step 3: Add profile list page**

Create `frontend/src/modules/shops/pages/ShopProfileListPage.tsx` with this initial structure:

```tsx
import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { RefreshCw, Store } from 'lucide-react'
import { Badge, Button, Card, StatCard, Table, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import { deriveShopProfileStats, fetchShopProfiles } from '../api'
import type { ShopProfile } from '../types'

function asmBadge(status: string) {
  if (status === 'connected') return <Badge variant="success">ASM 已接入</Badge>
  if (status === 'error') return <Badge variant="danger">ASM 异常</Badge>
  return <Badge variant="warning">ASM 待接入</Badge>
}

export function ShopProfileListPage() {
  const [profiles, setProfiles] = useState<ShopProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const stats = useMemo(() => deriveShopProfileStats(profiles), [profiles])

  async function load(silent = false) {
    if (silent) setRefreshing(true)
    else setLoading(true)
    try {
      setProfiles(await fetchShopProfiles())
    } catch (error) {
      console.error('load shop profiles failed', error)
      toast.error('加载店铺资料失败')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const columns: TableColumn<ShopProfile>[] = [
    {
      key: 'shopName',
      title: '店铺',
      render: (_, record) => (
        <div className="flex flex-col gap-1">
          <Link className="text-sm font-medium text-[var(--color-accent)] hover:underline" to={`/shops/${encodeURIComponent(record.shopId)}`}>
            {record.shopName || record.shopId}
          </Link>
          <span className="text-xs text-[var(--color-text-muted)]">{record.shopId}</span>
        </div>
      ),
    },
    { key: 'platformCode', title: '平台', render: (value) => <Badge variant="default">{String(value || '-')}</Badge> },
    { key: 'asmStatus', title: 'ASM 状态', render: (value) => asmBadge(String(value || '')) },
    { key: 'authorizationStatus', title: '授权状态', render: (value) => <span>{String(value || '-')}</span> },
    { key: 'ownerName', title: '负责人', render: (value) => <span>{String(value || '-')}</span> },
    { key: 'mainCategory', title: '主营类目', render: (value) => <span>{String(value || '-')}</span> },
    { key: 'lastSyncedAt', title: '最近同步', render: (value) => <span className="text-xs text-[var(--color-text-muted)]">{String(value || '-')}</span> },
  ]

  return (
    <div className="space-y-5 p-5 animate-fade-in">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">店铺资料</h1>
          <p className="mt-1 text-sm text-[var(--color-text-muted)]">ASM 店铺业务主数据，不混入浏览器实例配置。</p>
        </div>
        <Button variant="secondary" size="sm" onClick={() => void load(true)} loading={refreshing}>
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </div>
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard title="店铺总数" value={loading ? '-' : stats.total} icon={<Store className="h-5 w-5" />} />
        <StatCard title="ASM 已接入" value={loading ? '-' : stats.asmConnected} icon={<Badge variant="success">A</Badge>} />
        <StatCard title="ASM 待接入" value={loading ? '-' : stats.unavailable} icon={<Badge variant="warning">!</Badge>} />
        <StatCard title="资料待完善" value={loading ? '-' : stats.incomplete} icon={<Badge variant="default">D</Badge>} />
      </div>
      <Card padding="none">
        <Table columns={columns} data={profiles} rowKey="shopId" loading={loading} emptyText="暂无 ASM 店铺资料" />
      </Card>
    </div>
  )
}
```

- [ ] **Step 4: Add profile detail page**

Create `frontend/src/modules/shops/pages/ShopProfileDetailPage.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ArrowLeft, ExternalLink } from 'lucide-react'
import { Button, Card, Loading, toast } from '../../../shared/components'
import { fetchShopProfile } from '../api'
import type { ShopProfile } from '../types'

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-[var(--color-border-muted)] py-3 last:border-0">
      <span className="text-sm text-[var(--color-text-muted)]">{label}</span>
      <span className="max-w-[60%] break-all text-right text-sm text-[var(--color-text-primary)]">{value || '-'}</span>
    </div>
  )
}

export function ShopProfileDetailPage() {
  const { shopId = '' } = useParams()
  const [profile, setProfile] = useState<ShopProfile | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    async function load() {
      setLoading(true)
      try {
        const next = await fetchShopProfile(shopId)
        if (!cancelled) setProfile(next)
      } catch (error) {
        console.error('load shop profile failed', error)
        toast.error('加载店铺详情失败')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [shopId])

  if (loading) return <div className="p-8"><Loading text="加载店铺资料..." /></div>
  if (!profile) return <div className="p-8 text-sm text-[var(--color-text-muted)]">店铺资料不存在</div>

  return (
    <div className="space-y-5 p-5 animate-fade-in">
      <div className="flex items-center justify-between gap-3">
        <div>
          <Link to="/shops" className="mb-2 inline-flex items-center gap-1 text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]">
            <ArrowLeft className="h-4 w-4" />
            返回店铺资料
          </Link>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">{profile.shopName || profile.shopId}</h1>
          <p className="mt-1 text-sm text-[var(--color-text-muted)]">{profile.platformCode || '-'} · {profile.shopId}</p>
        </div>
        <div className="flex gap-2">
          <Link to={`/workbench?shopId=${encodeURIComponent(profile.shopId)}`}>
            <Button size="sm">
              <ExternalLink className="h-4 w-4" />
              去工作台
            </Button>
          </Link>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card title="基础资料" subtitle="ASM 店铺业务主数据">
          <DetailRow label="店铺名称" value={profile.shopName} />
          <DetailRow label="Shop ID" value={profile.shopId} />
          <DetailRow label="平台" value={profile.platformCode} />
          <DetailRow label="负责人" value={profile.ownerName} />
          <DetailRow label="主营类目" value={profile.mainCategory} />
        </Card>
        <Card title="ASM 与执行摘要" subtitle="执行详情在店铺工作台查看">
          <DetailRow label="ASM 状态" value={profile.asmStatus} />
          <DetailRow label="授权状态" value={profile.authorizationStatus} />
          <DetailRow label="数据完整度" value={profile.dataCompleteness} />
          <DetailRow label="最近同步" value={profile.lastSyncedAt} />
          <DetailRow label="数据来源" value={profile.source} />
        </Card>
      </div>
    </div>
  )
}
```

- [ ] **Step 5: Add module barrel**

Create `frontend/src/modules/shops/index.ts`:

```ts
export * from './api'
export * from './types'
```

- [ ] **Step 6: Build**

```bash
rtk npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
rtk git add frontend/src/modules/shops
rtk git commit -m "Add ASM shop profile pages"
```

---

## Task 6: Workbench Domain Logic

**Files:**
- Create: `frontend/src/modules/workbench/types.ts`
- Create: `frontend/src/modules/workbench/recovery.ts`
- Create: `frontend/src/modules/workbench/batch.ts`
- Create: `frontend/src/modules/workbench/index.ts`

- [ ] **Step 1: Add workbench types**

Create `frontend/src/modules/workbench/types.ts`:

```ts
import type { ShopRunEvidence } from '../runEvidence'
import type { WorkspaceAuthorizedShop } from '../workspace/types'

export type WorkbenchQueueKey = 'ready' | 'manual' | 'credential' | 'failed' | 'running' | 'reclaim'
export type WorkbenchActionKey = 'open' | 'bind' | 'validate' | 'retry' | 'core_management' | 'refresh' | 'diagnostics' | 'none'

export interface WorkbenchRow {
  shop: WorkspaceAuthorizedShop
  evidence: ShopRunEvidence
  queue: WorkbenchQueueKey
  recommendedAction: WorkbenchActionKey
  failureCode: string
  failureMessage: string
}

export interface RecoveryAction {
  key: WorkbenchActionKey
  label: string
  description: string
  retryable: boolean
  batchSkippable: boolean
}

export interface BatchCandidate {
  shopId: string
  action: WorkbenchActionKey
  eligible: boolean
  skipReason: string
}

export interface BatchSummary {
  total: number
  eligible: number
  skipped: number
  failed: number
  succeeded: number
}
```

- [ ] **Step 2: Add recovery mapping**

Create `frontend/src/modules/workbench/recovery.ts`:

```ts
import type { RecoveryAction, WorkbenchActionKey } from './types'

const actions: Record<WorkbenchActionKey, RecoveryAction> = {
  open: { key: 'open', label: '打开后台', description: '店铺已可执行，可直接打开后台', retryable: true, batchSkippable: false },
  bind: { key: 'bind', label: '更新凭据', description: '共享登录未就绪，需要更新凭据', retryable: true, batchSkippable: false },
  validate: { key: 'validate', label: '本机验证', description: '需要在本机完成验证', retryable: true, batchSkippable: false },
  retry: { key: 'retry', label: '重试', description: '最近失败动作可重试', retryable: true, batchSkippable: false },
  core_management: { key: 'core_management', label: '配置指纹内核', description: '缺少可用指纹内核，需先修复内核配置', retryable: false, batchSkippable: true },
  refresh: { key: 'refresh', label: '刷新同步', description: '刷新授权店铺与本地 profile 映射', retryable: true, batchSkippable: false },
  diagnostics: { key: 'diagnostics', label: '查看诊断', description: '查看运行证据并导出诊断信息', retryable: false, batchSkippable: true },
  none: { key: 'none', label: '不可执行', description: '当前状态不允许执行动作', retryable: false, batchSkippable: true },
}

export function recoveryActionFor(input: { reclaimPending?: boolean; profileExists?: boolean; coreReady?: boolean; sharedLoginStatus?: string; failureCode?: string }): RecoveryAction {
  if (input.reclaimPending) return actions.none
  if (!input.profileExists) return actions.refresh
  if (!input.coreReady || input.failureCode === 'ANT_CORE_UNAVAILABLE' || input.failureCode === 'ANT_CORE_NOT_FOUND' || input.failureCode === 'ANT_FINGERPRINT_CORE_REQUIRED') {
    return actions.core_management
  }
  if (input.sharedLoginStatus === 'awaiting_verification') return actions.validate
  if (input.sharedLoginStatus === 'validation_failed') return actions.bind
  if (input.sharedLoginStatus !== 'ready') return actions.bind
  if (input.failureCode) return actions.retry
  return actions.open
}
```

- [ ] **Step 3: Add batch eligibility helpers**

Create `frontend/src/modules/workbench/batch.ts`:

```ts
import type { WorkbenchActionKey, WorkbenchRow, BatchCandidate, BatchSummary } from './types'

export function buildBatchCandidates(rows: WorkbenchRow[], action: WorkbenchActionKey): BatchCandidate[] {
  return rows.map((row) => {
    if (row.shop.reclaimPending) {
      return { shopId: row.shop.shopId, action, eligible: false, skipReason: '授权已失效，待回收' }
    }
    if (!row.shop.profileExists) {
      return { shopId: row.shop.shopId, action, eligible: false, skipReason: '本地 profile 未映射' }
    }
    if (!row.shop.coreReady) {
      return { shopId: row.shop.shopId, action, eligible: false, skipReason: '指纹内核不可用' }
    }
    if (action === 'open' && row.shop.sharedLoginStatus !== 'ready') {
      return { shopId: row.shop.shopId, action, eligible: false, skipReason: '共享会话未就绪' }
    }
    return { shopId: row.shop.shopId, action, eligible: true, skipReason: '' }
  })
}

export function summarizeBatch(candidates: BatchCandidate[], results: Array<{ shopId: string; success: boolean }>): BatchSummary {
  const succeeded = results.filter((item) => item.success).length
  const failed = results.filter((item) => !item.success).length
  const skipped = candidates.filter((item) => !item.eligible).length
  return {
    total: candidates.length,
    eligible: candidates.length - skipped,
    skipped,
    succeeded,
    failed,
  }
}
```

- [ ] **Step 4: Add barrel**

Create `frontend/src/modules/workbench/index.ts`:

```ts
export * from './batch'
export * from './recovery'
export * from './types'
```

- [ ] **Step 5: Build**

```bash
rtk npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add frontend/src/modules/workbench
rtk git commit -m "Add workbench domain helpers"
```

---

## Task 7: Shop Workbench UI

**Files:**
- Create: `frontend/src/modules/workbench/WorkbenchPage.tsx`
- Create: `frontend/src/modules/workbench/components/WorkbenchQueues.tsx`
- Create: `frontend/src/modules/workbench/components/WorkbenchTable.tsx`
- Create: `frontend/src/modules/workbench/components/ShopWorkbenchDrawer.tsx`

- [ ] **Step 1: Add queue component**

Create `frontend/src/modules/workbench/components/WorkbenchQueues.tsx`:

```tsx
import { Badge, Card } from '../../../shared/components'
import type { WorkbenchQueueKey, WorkbenchRow } from '../types'

const labels: Record<WorkbenchQueueKey, string> = {
  ready: '可直接打开',
  manual: '待人工验证',
  credential: '凭据待处理',
  failed: '打开失败',
  running: '当前运行中',
  reclaim: '授权失效',
}

export function WorkbenchQueues({ rows, active, onSelect }: { rows: WorkbenchRow[]; active: WorkbenchQueueKey | 'all'; onSelect: (queue: WorkbenchQueueKey | 'all') => void }) {
  const count = (queue: WorkbenchQueueKey) => rows.filter((row) => row.queue === queue).length
  const itemClass = (selected: boolean) => `flex w-full items-center justify-between rounded-lg px-3 py-2 text-left text-sm transition-colors ${selected ? 'bg-[var(--color-accent)] text-[var(--color-text-inverse)]' : 'hover:bg-[var(--color-bg-muted)] text-[var(--color-text-secondary)]'}`

  return (
    <Card title="工作队列" padding="sm">
      <div className="space-y-1">
        <button className={itemClass(active === 'all')} onClick={() => onSelect('all')}>
          <span>全部店铺</span>
          <Badge variant="default">{rows.length}</Badge>
        </button>
        {(Object.keys(labels) as WorkbenchQueueKey[]).map((queue) => (
          <button key={queue} className={itemClass(active === queue)} onClick={() => onSelect(queue)}>
            <span>{labels[queue]}</span>
            <Badge variant="default">{count(queue)}</Badge>
          </button>
        ))}
      </div>
    </Card>
  )
}
```

- [ ] **Step 2: Add drawer component**

Create `frontend/src/modules/workbench/components/ShopWorkbenchDrawer.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { Modal, Badge, Button } from '../../../shared/components'
import { fetchWorkspaceRunEvents, RunTimeline, type RunEvent } from '../../runEvidence'
import type { WorkbenchRow } from '../types'

export function ShopWorkbenchDrawer({ row, open, onClose, onAction }: { row: WorkbenchRow | null; open: boolean; onClose: () => void; onAction: (row: WorkbenchRow) => void }) {
  const [events, setEvents] = useState<RunEvent[]>([])
  const selectedRun = row?.evidence.latestFailure || row?.evidence.latestOpen || row?.evidence.latestValidation || null

  useEffect(() => {
    let cancelled = false
    async function load() {
      if (!selectedRun?.runId) {
        setEvents([])
        return
      }
      const payload = await fetchWorkspaceRunEvents(selectedRun.runId, 50)
      if (!cancelled) setEvents(payload.items)
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [selectedRun?.runId])

  if (!row) return null

  return (
    <Modal open={open} onClose={onClose} title={row.shop.shopName || row.shop.shopId} width="820px">
      <div className="space-y-5">
        <div className="flex flex-wrap gap-2">
          <Badge variant={row.shop.sharedLoginStatus === 'ready' ? 'success' : 'warning'}>{row.shop.sharedLoginStatusLabel || row.shop.sharedLoginStatus || 'unknown'}</Badge>
          <Badge variant={row.shop.coreReady ? 'success' : 'warning'}>{row.shop.coreReady ? '内核就绪' : '内核不可用'}</Badge>
          <Badge variant={row.shop.profileExists ? 'default' : 'warning'}>{row.shop.profileExists ? 'Profile 已映射' : 'Profile 未映射'}</Badge>
        </div>
        <div className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] p-4">
          <div className="mb-2 text-sm font-semibold text-[var(--color-text-primary)]">推荐动作</div>
          <p className="mb-3 text-sm text-[var(--color-text-secondary)]">{row.failureMessage || '当前店铺可按推荐动作继续处理。'}</p>
          <Button size="sm" onClick={() => onAction(row)}>执行推荐动作</Button>
        </div>
        <div>
          <div className="mb-3 text-sm font-semibold text-[var(--color-text-primary)]">运行证据</div>
          <RunTimeline events={events} />
        </div>
      </div>
    </Modal>
  )
}
```

- [ ] **Step 3: Add table component**

Create `frontend/src/modules/workbench/components/WorkbenchTable.tsx`:

```tsx
import { Link } from 'react-router-dom'
import { Badge, Button, Table } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import type { WorkbenchRow } from '../types'

export function WorkbenchTable({ rows, loading, onOpenDrawer, onAction }: { rows: WorkbenchRow[]; loading: boolean; onOpenDrawer: (row: WorkbenchRow) => void; onAction: (row: WorkbenchRow) => void }) {
  const columns: TableColumn<WorkbenchRow>[] = [
    {
      key: 'shop',
      title: '店铺',
      render: (_, row) => (
        <div className="flex flex-col gap-1">
          <button className="text-left text-sm font-medium text-[var(--color-accent)] hover:underline" onClick={(event) => { event.stopPropagation(); onOpenDrawer(row) }}>
            {row.shop.shopName || row.shop.shopId}
          </button>
          <span className="text-xs text-[var(--color-text-muted)]">{row.shop.shopId}</span>
        </div>
      ),
    },
    { key: 'queue', title: '执行状态', render: (_, row) => <Badge variant={row.queue === 'ready' ? 'success' : row.queue === 'failed' ? 'danger' : 'warning'}>{row.queue}</Badge> },
    { key: 'latestOpen', title: '最近打开', render: (_, row) => <span className="text-xs text-[var(--color-text-muted)]">{row.evidence.latestOpen?.startedAt || '-'}</span> },
    { key: 'latestValidation', title: '最近验证', render: (_, row) => <span className="text-xs text-[var(--color-text-muted)]">{row.evidence.latestValidation?.startedAt || '-'}</span> },
    { key: 'failure', title: '最近失败', render: (_, row) => <span className="text-xs text-[var(--color-text-muted)]">{row.failureCode || '-'}</span> },
    { key: 'profile', title: '资料', render: (_, row) => <Link className="text-sm text-[var(--color-accent)] hover:underline" to={`/shops/${encodeURIComponent(row.shop.shopId)}`}>查看</Link> },
    {
      key: 'actions',
      title: '操作',
      align: 'right',
      render: (_, row) => (
        <Button size="sm" onClick={(event) => { event.stopPropagation(); onAction(row) }}>
          推荐动作
        </Button>
      ),
    },
  ]

  return <Table columns={columns} data={rows} rowKey={(row) => row.shop.shopId} loading={loading} emptyText="暂无可处理店铺" onRowClick={onOpenDrawer} />
}
```

- [ ] **Step 4: Add workbench page**

Create `frontend/src/modules/workbench/WorkbenchPage.tsx`:

```tsx
import { useEffect, useMemo, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { Button, Card, toast } from '../../shared/components'
import { fetchWorkspaceAuthorizedShops, openWorkspaceShop, startWorkspaceSharedLoginBind, startWorkspaceSharedLoginValidate } from '../workspace/api'
import type { WorkspaceAuthorizedShop } from '../workspace/types'
import { buildRunEvidenceIndex, fetchWorkspaceRuns } from '../runEvidence'
import { recoveryActionFor } from './recovery'
import type { WorkbenchQueueKey, WorkbenchRow } from './types'
import { WorkbenchQueues } from './components/WorkbenchQueues'
import { WorkbenchTable } from './components/WorkbenchTable'
import { ShopWorkbenchDrawer } from './components/ShopWorkbenchDrawer'
import { useAuthStore } from '../../store/authStore'

function queueFor(shop: WorkspaceAuthorizedShop, failureCode: string): WorkbenchQueueKey {
  if (shop.reclaimPending) return 'reclaim'
  if (shop.instanceRunning) return 'running'
  if (failureCode) return 'failed'
  if (shop.sharedLoginStatus === 'awaiting_verification') return 'manual'
  if (shop.sharedLoginStatus !== 'ready') return 'credential'
  return 'ready'
}

export function WorkbenchPage() {
  const accessToken = useAuthStore((state) => state.accessToken)
  const [shops, setShops] = useState<WorkspaceAuthorizedShop[]>([])
  const [runs, setRuns] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [activeQueue, setActiveQueue] = useState<WorkbenchQueueKey | 'all'>('all')
  const [selectedRow, setSelectedRow] = useState<WorkbenchRow | null>(null)

  async function load(silent = false) {
    if (silent) setRefreshing(true)
    else setLoading(true)
    try {
      const [nextShops, nextRuns] = await Promise.all([
        fetchWorkspaceAuthorizedShops(),
        fetchWorkspaceRuns({ limit: 100 }),
      ])
      setShops(nextShops)
      setRuns(nextRuns.items)
    } catch (error) {
      console.error('load workbench failed', error)
      toast.error('加载店铺工作台失败')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const rows = useMemo<WorkbenchRow[]>(() => {
    const index = buildRunEvidenceIndex(runs)
    return shops.map((shop) => {
      const evidence = index.byShop[shop.shopId] || { latestOpen: null, latestCredential: null, latestValidation: null, latestFailure: null, activeRun: null }
      const failureCode = evidence.latestFailure?.failureCode || ''
      const recovery = recoveryActionFor({ reclaimPending: shop.reclaimPending, profileExists: shop.profileExists, coreReady: shop.coreReady, sharedLoginStatus: shop.sharedLoginStatus, failureCode })
      return {
        shop,
        evidence,
        queue: queueFor(shop, failureCode),
        recommendedAction: recovery.key,
        failureCode,
        failureMessage: evidence.latestFailure?.failureMessage || '',
      }
    })
  }, [runs, shops])

  const visibleRows = activeQueue === 'all' ? rows : rows.filter((row) => row.queue === activeQueue)

  async function runRecommendedAction(row: WorkbenchRow) {
    try {
      if (row.recommendedAction === 'open') await openWorkspaceShop(row.shop.shopId)
      else if (row.recommendedAction === 'bind') await startWorkspaceSharedLoginBind(accessToken, row.shop.shopId)
      else if (row.recommendedAction === 'validate') await startWorkspaceSharedLoginValidate(accessToken, row.shop.shopId)
      else {
        toast.warning('当前状态需要先查看诊断或完成配置')
        return
      }
      toast.success('动作已发起')
      await load(true)
    } catch (error: any) {
      console.error('run recommended action failed', error)
      toast.error(error?.message || '动作执行失败')
    }
  }

  return (
    <div className="grid h-full grid-cols-1 gap-5 overflow-auto p-5 lg:grid-cols-[240px_1fr] animate-fade-in">
      <div className="space-y-4">
        <WorkbenchQueues rows={rows} active={activeQueue} onSelect={setActiveQueue} />
      </div>
      <div className="min-w-0 space-y-4">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">店铺工作台</h1>
            <p className="mt-1 text-sm text-[var(--color-text-muted)]">围绕店铺可执行性处理打开、凭据、验证和失败修复。</p>
          </div>
          <Button variant="secondary" size="sm" onClick={() => void load(true)} loading={refreshing}>
            <RefreshCw className="h-4 w-4" />
            刷新
          </Button>
        </div>
        <Card padding="none">
          <WorkbenchTable rows={visibleRows} loading={loading} onOpenDrawer={setSelectedRow} onAction={(row) => void runRecommendedAction(row)} />
        </Card>
      </div>
      <ShopWorkbenchDrawer row={selectedRow} open={Boolean(selectedRow)} onClose={() => setSelectedRow(null)} onAction={(row) => void runRecommendedAction(row)} />
    </div>
  )
}
```

- [ ] **Step 5: Build**

```bash
rtk npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add frontend/src/modules/workbench
rtk git commit -m "Add shop workbench UI"
```

---

## Task 8: Operation Task Center Skeleton

**Files:**
- Create: `frontend/src/modules/operations/types.ts`
- Create: `frontend/src/modules/operations/api.ts`
- Create: `frontend/src/modules/operations/pages/OperationTaskCenterPage.tsx`
- Create: `frontend/src/modules/operations/index.ts`

- [ ] **Step 1: Add operation task types and adapter**

Create `frontend/src/modules/operations/types.ts`:

```ts
export type OperationTaskStatus = 'waiting' | 'running' | 'blocked' | 'failed' | 'completed'

export interface OperationTask {
  taskId: string
  shopId: string
  shopName: string
  taskType: string
  title: string
  status: OperationTaskStatus
  blockedReason: string
  failureMessage: string
  updatedAt: string
}
```

Create `frontend/src/modules/operations/api.ts`:

```ts
import type { OperationTask } from './types'

export async function fetchOperationTasks(): Promise<OperationTask[]> {
  return []
}

export function deriveOperationTaskCounts(tasks: OperationTask[]) {
  return {
    total: tasks.length,
    waiting: tasks.filter((task) => task.status === 'waiting').length,
    running: tasks.filter((task) => task.status === 'running').length,
    blocked: tasks.filter((task) => task.status === 'blocked').length,
    failed: tasks.filter((task) => task.status === 'failed').length,
    completed: tasks.filter((task) => task.status === 'completed').length,
  }
}
```

- [ ] **Step 2: Add operation task center page**

Create `frontend/src/modules/operations/pages/OperationTaskCenterPage.tsx`:

```tsx
import { useEffect, useMemo, useState } from 'react'
import { ListChecks, RefreshCw } from 'lucide-react'
import { Badge, Button, Card, StatCard, Table, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import { deriveOperationTaskCounts, fetchOperationTasks } from '../api'
import type { OperationTask } from '../types'

export function OperationTaskCenterPage() {
  const [tasks, setTasks] = useState<OperationTask[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const counts = useMemo(() => deriveOperationTaskCounts(tasks), [tasks])

  async function load(silent = false) {
    if (silent) setRefreshing(true)
    else setLoading(true)
    try {
      setTasks(await fetchOperationTasks())
    } catch (error) {
      console.error('load operation tasks failed', error)
      toast.error('加载运营任务失败')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const columns: TableColumn<OperationTask>[] = [
    { key: 'title', title: '任务', render: (value, row) => <div><div className="text-sm font-medium text-[var(--color-text-primary)]">{String(value || row.taskId)}</div><div className="text-xs text-[var(--color-text-muted)]">{row.taskType}</div></div> },
    { key: 'shopName', title: '店铺', render: (value, row) => <span>{String(value || row.shopId || '-')}</span> },
    { key: 'status', title: '状态', render: (value) => <Badge variant={value === 'blocked' || value === 'failed' ? 'warning' : 'default'}>{String(value || '-')}</Badge> },
    { key: 'blockedReason', title: '阻塞原因', render: (value) => <span className="text-xs text-[var(--color-text-muted)]">{String(value || '-')}</span> },
    { key: 'updatedAt', title: '更新时间', render: (value) => <span className="text-xs text-[var(--color-text-muted)]">{String(value || '-')}</span> },
  ]

  return (
    <div className="space-y-5 p-5 animate-fade-in">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">运营任务</h1>
          <p className="mt-1 text-sm text-[var(--color-text-muted)]">跨店铺任务视图，本阶段先建立任务归属和状态边界。</p>
        </div>
        <Button variant="secondary" size="sm" onClick={() => void load(true)} loading={refreshing}>
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </div>
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-5">
        <StatCard title="总任务" value={loading ? '-' : counts.total} icon={<ListChecks className="h-5 w-5" />} />
        <StatCard title="等待中" value={loading ? '-' : counts.waiting} icon={<Badge variant="default">W</Badge>} />
        <StatCard title="运行中" value={loading ? '-' : counts.running} icon={<Badge variant="info">R</Badge>} />
        <StatCard title="阻塞" value={loading ? '-' : counts.blocked} icon={<Badge variant="warning">B</Badge>} />
        <StatCard title="失败" value={loading ? '-' : counts.failed} icon={<Badge variant="danger">F</Badge>} />
      </div>
      <Card padding="none">
        <Table columns={columns} data={tasks} rowKey="taskId" loading={loading} emptyText="暂无运营任务" />
      </Card>
    </div>
  )
}
```

- [ ] **Step 3: Add barrel**

Create `frontend/src/modules/operations/index.ts`:

```ts
export * from './api'
export * from './types'
```

- [ ] **Step 4: Build**

```bash
rtk npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add frontend/src/modules/operations
rtk git commit -m "Add operation task center skeleton"
```

---

## Task 9: Routes And Navigation

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/config/project.config.ts`
- Modify: `frontend/src/shared/layout/Sidebar.tsx`

- [ ] **Step 1: Update icon map**

Modify `frontend/src/shared/layout/Sidebar.tsx` imports to include:

```ts
Bot,
Store,
```

Add to `iconMap`:

```ts
Bot,
Store,
```

- [ ] **Step 2: Update routes**

Modify `frontend/src/App.tsx`:

```tsx
const ShopProfileListPage = lazyNamed(() => import('./modules/shops/pages/ShopProfileListPage'), 'ShopProfileListPage')
const ShopProfileDetailPage = lazyNamed(() => import('./modules/shops/pages/ShopProfileDetailPage'), 'ShopProfileDetailPage')
const WorkbenchPage = lazyNamed(() => import('./modules/workbench/WorkbenchPage'), 'WorkbenchPage')
const OperationTaskCenterPage = lazyNamed(() => import('./modules/operations/pages/OperationTaskCenterPage'), 'OperationTaskCenterPage')
```

Add protected routes:

```tsx
<Route path="/shops" element={<ShopProfileListPage />} />
<Route path="/shops/:shopId" element={<ShopProfileDetailPage />} />
<Route path="/workbench" element={<WorkbenchPage />} />
<Route path="/operations" element={<OperationTaskCenterPage />} />
<Route path="/browser/list" element={<Navigate to="/workbench" replace />} />
```

Replace the existing `/browser/list` route with the redirect above.

- [ ] **Step 3: Update navigation**

Modify `frontend/src/config/project.config.ts` `navigationConfig` so the business group appears before low-level browser tools:

```ts
export const navigationConfig: NavSection[] = [
  {
    title: '业务运营',
    items: [
      { name: '控制台', path: '/', icon: 'LayoutDashboard' },
      { name: '店铺资料', path: '/shops', icon: 'Store' },
      { name: '店铺工作台', path: '/workbench', icon: 'Monitor' },
      { name: '运营任务', path: '/operations', icon: 'ListChecks' },
    ],
  },
  {
    title: '指纹浏览器',
    items: [
      { name: '自动化接口（实验）', path: '/browser/automation', icon: 'Bot' },
      { name: '内核管理', path: '/browser/cores', icon: 'Cpu' },
      { name: '代理池配置', path: '/browser/proxy-pool', icon: 'Globe' },
      { name: '默认书签', path: '/browser/bookmarks', icon: 'Bookmark' },
      { name: '标签管理', path: '/browser/tags', icon: 'Tag' },
    ],
  },
  {
    title: '系统维护',
    items: [
      { name: '系统设置', path: '/settings', icon: 'Settings' },
      { name: '使用教程', path: '/system/tutorial', icon: 'BookOpen' },
      { name: '日志查看', path: '/browser/logs', icon: 'FileText' },
      { name: '接口文档', path: '/browser/launch-api', icon: 'BookOpen' },
    ],
  },
]
```

- [ ] **Step 4: Build**

```bash
rtk npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add frontend/src/App.tsx frontend/src/config/project.config.ts frontend/src/shared/layout/Sidebar.tsx
rtk git commit -m "Wire shop operations navigation"
```

---

## Task 10: Safe Batch Operations UI

**Files:**
- Modify: `frontend/src/modules/workbench/WorkbenchPage.tsx`
- Modify: `frontend/src/modules/workbench/components/WorkbenchTable.tsx`

- [ ] **Step 1: Add selected row state and batch open action**

Modify `frontend/src/modules/workbench/WorkbenchPage.tsx` to add:

```tsx
import { buildBatchCandidates, summarizeBatch } from './batch'
```

Add state:

```tsx
const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
const selectedRows = visibleRows.filter((row) => selectedIds.has(row.shop.shopId))
```

Add function:

```tsx
async function runBatchOpen() {
  const candidates = buildBatchCandidates(selectedRows, 'open')
  const eligible = candidates.filter((candidate) => candidate.eligible)
  if (eligible.length === 0) {
    toast.warning('没有可批量打开的店铺')
    return
  }

  const results: Array<{ shopId: string; success: boolean }> = []
  for (const candidate of eligible) {
    try {
      const result = await openWorkspaceShop(candidate.shopId)
      results.push({ shopId: candidate.shopId, success: Boolean(result.success) })
    } catch {
      results.push({ shopId: candidate.shopId, success: false })
    }
  }
  const summary = summarizeBatch(candidates, results)
  toast.success(`批量打开完成：成功 ${summary.succeeded}，失败 ${summary.failed}，跳过 ${summary.skipped}`)
  await load(true)
}
```

Add toolbar before the table:

```tsx
{selectedRows.length > 0 ? (
  <div className="flex items-center gap-3 rounded-lg border border-[var(--color-accent)]/20 bg-[var(--color-accent)]/10 px-4 py-2.5">
    <span className="text-sm font-medium text-[var(--color-accent)]">已选 {selectedRows.length}</span>
    <div className="ml-auto flex gap-2">
      <Button size="sm" variant="secondary" onClick={() => setSelectedIds(new Set())}>取消选择</Button>
      <Button size="sm" onClick={() => void runBatchOpen()}>批量打开 Ready</Button>
    </div>
  </div>
) : null}
```

- [ ] **Step 2: Add selection to table**

Modify `WorkbenchTable` props:

```tsx
selectedIds: Set<string>
onToggleSelect: (shopId: string) => void
```

Add first column:

```tsx
{
  key: 'selection',
  title: '',
  width: 48,
  render: (_, row) => (
    <input
      type="checkbox"
      className="h-4 w-4 cursor-pointer rounded accent-[var(--color-accent)]"
      checked={selectedIds.has(row.shop.shopId)}
      onChange={() => onToggleSelect(row.shop.shopId)}
      onClick={(event) => event.stopPropagation()}
    />
  ),
},
```

Pass props from `WorkbenchPage`:

```tsx
<WorkbenchTable
  rows={visibleRows}
  loading={loading}
  selectedIds={selectedIds}
  onToggleSelect={(shopId) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(shopId)) next.delete(shopId)
      else next.add(shopId)
      return next
    })
  }}
  onOpenDrawer={setSelectedRow}
  onAction={(row) => void runRecommendedAction(row)}
/>
```

- [ ] **Step 3: Build**

```bash
rtk npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 4: Manual smoke in browser**

Run local app in the established project flow. If a Wails dev session is already running, reuse it. Otherwise run:

```bash
rtk scripts/dev-mac.sh stable
```

Expected:

- `/workbench` renders
- selecting rows shows batch toolbar
- batch open does not show the previous "后续任务接入真实流程" warning
- ineligible shops are not executed by the batch helper

- [ ] **Step 5: Commit**

```bash
rtk git add frontend/src/modules/workbench/WorkbenchPage.tsx frontend/src/modules/workbench/components/WorkbenchTable.tsx
rtk git commit -m "Add safe workbench batch open"
```

---

## Task 11: Final Verification And Phase Report

**Files:**
- Create: `docs/reports/2026-05-23-asm-shop-operations-closure-plan-verification.md`

- [ ] **Step 1: Run backend tests**

```bash
rtk go test ./backend ./backend/internal/workspace ./backend/test/workspace -count=1
```

Expected: PASS.

- [ ] **Step 2: Run frontend build**

```bash
rtk npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 3: Run diff check**

```bash
rtk git diff --check
```

Expected: no output.

- [ ] **Step 4: Write verification report**

Create `docs/reports/2026-05-23-asm-shop-operations-closure-plan-verification.md`:

````markdown
# ASM Shop Operations Closure Plan Verification

Date: 2026-05-23
Branch: `codex/windows-phase1-stability`

## Scope

Verified implementation of:

- ASM shop profile list and detail surface
- shop workbench execution readiness surface
- run evidence API and frontend evidence derivation
- operation task center skeleton
- safe batch open eligibility and summary
- business-first navigation

## Commands

```bash
rtk go test ./backend ./backend/internal/workspace ./backend/test/workspace -count=1
rtk npm --prefix frontend run build
rtk git diff --check
```

## Result

- Backend tests: pass
- Frontend build: pass
- Diff check: pass

## Remaining Product Scope

- Full product sourcing workflow remains out of scope.
- Full listing/publishing workflow remains out of scope.
- AI operation report generation remains out of scope.
- Long-running external task scheduler remains out of scope.
````

- [ ] **Step 5: Commit**

```bash
rtk git add docs/reports/2026-05-23-asm-shop-operations-closure-plan-verification.md
rtk git commit -m "Record ASM shop operations verification"
```

---

## Self-Review Checklist

Before execution starts:

- [ ] The plan keeps ASM shop profile, authorized shop projection, operation task, and run evidence separate.
- [ ] The plan does not store product/order business entities in Ant Browser local state.
- [ ] The plan routes frontend calls through Wails/backend APIs.
- [ ] The plan replaces warning-only batch open behavior with real safe batch execution.
- [ ] The plan includes backend tests for new local agent adapters and evidence derivation.
- [ ] The plan includes frontend build verification after each frontend slice.
- [ ] The plan leaves product sourcing, listing, AI reports, and external durable scheduling out of scope.
