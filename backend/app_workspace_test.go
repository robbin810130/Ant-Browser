package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ant-chrome/backend/internal/workspace"
)

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
