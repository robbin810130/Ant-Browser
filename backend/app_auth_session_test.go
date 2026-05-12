package backend

import (
	"ant-chrome/backend/internal/authsession"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/workspace"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFetchDesktopAuthProfileUsesAuthMePayload(t *testing.T) {
	root := t.TempDir()
	app := NewApp(root)

	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/auth/me" {
			http.NotFound(w, r)
			return
		}

		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "ok",
			"data": map[string]any{
				"user": map[string]any{
					"id":          "user-1",
					"displayName": "桌面用户",
					"username":    "desktop-admin",
				},
				"roles": []map[string]any{
					{
						"code": "admin",
						"name": "管理员",
					},
				},
				"dataScope": "all",
			},
		})
	}))
	defer server.Close()

	app.config = &config.Config{
		Workspace: config.WorkspaceConfig{
			ServerOrigin: server.URL,
		},
	}

	profile, err := app.FetchDesktopAuthProfile(" desktop-token ")
	if err != nil {
		t.Fatalf("FetchDesktopAuthProfile 返回错误: %v", err)
	}
	if authHeader != "Bearer desktop-token" {
		t.Fatalf("期望 Authorization=Bearer desktop-token，实际=%q", authHeader)
	}
	if profile == nil {
		t.Fatal("expected non-nil profile")
	}
	if profile.User.ID != "user-1" {
		t.Fatalf("期望 user.id=user-1，实际=%q", profile.User.ID)
	}
	if profile.User.DisplayName != "桌面用户" {
		t.Fatalf("期望 displayName=桌面用户，实际=%q", profile.User.DisplayName)
	}
	if profile.User.Username != "desktop-admin" {
		t.Fatalf("期望 username=desktop-admin，实际=%q", profile.User.Username)
	}
	if len(profile.Roles) != 1 || profile.Roles[0].Code != "admin" {
		t.Fatalf("期望 roles[0].code=admin，实际=%#v", profile.Roles)
	}
	if profile.DataScope != "all" {
		t.Fatalf("期望 dataScope=all，实际=%q", profile.DataScope)
	}
}

func TestLoginDesktopUserUsesWorkspaceServerOrigin(t *testing.T) {
	root := t.TempDir()
	app := NewApp(root)

	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/auth/login" {
			http.NotFound(w, r)
			return
		}

		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("解析登录请求失败: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "ok",
			"data": map[string]any{
				"accessToken": "desktop-access-token",
			},
		})
	}))
	defer server.Close()

	app.config = &config.Config{
		Workspace: config.WorkspaceConfig{
			ServerOrigin: server.URL,
		},
	}

	accessToken, err := app.LoginDesktopUser(" desktop-admin ", " desktop-password ")
	if err != nil {
		t.Fatalf("LoginDesktopUser 返回错误: %v", err)
	}
	if accessToken != "desktop-access-token" {
		t.Fatalf("期望 accessToken=desktop-access-token，实际=%q", accessToken)
	}
	if got, _ := requestBody["username"].(string); got != "desktop-admin" {
		t.Fatalf("期望 username=desktop-admin，实际=%q", got)
	}
	if got, _ := requestBody["password"].(string); got != "desktop-password" {
		t.Fatalf("期望 password=desktop-password，实际=%q", got)
	}
}

func TestDesktopAuthSessionRoundTrip(t *testing.T) {
	root := t.TempDir()
	app := NewApp(root)
	app.authSessionStore = authsession.NewStore(root)

	if err := app.SaveDesktopAuthSession("token-123", true); err != nil {
		t.Fatalf("SaveDesktopAuthSession 返回错误: %v", err)
	}

	loaded, err := app.LoadDesktopAuthSession()
	if err != nil {
		t.Fatalf("LoadDesktopAuthSession 返回错误: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil session")
	}
	if loaded.AccessToken != "token-123" || !loaded.RememberMe {
		t.Fatalf("unexpected loaded session: %#v", loaded)
	}

	if err := app.ClearDesktopAuthSession(); err != nil {
		t.Fatalf("ClearDesktopAuthSession 返回错误: %v", err)
	}

	cleared, err := app.LoadDesktopAuthSession()
	if err != nil {
		t.Fatalf("LoadDesktopAuthSession after clear 返回错误: %v", err)
	}
	if cleared == nil {
		t.Fatal("expected non-nil cleared session")
	}
	if cleared.AccessToken != "" || cleared.RememberMe {
		t.Fatalf("expected cleared session, got %#v", cleared)
	}
}

func TestDesktopAuthStrongCleanupStopsManagedProfilesAndResetsSessionState(t *testing.T) {
	root := t.TempDir()
	app := NewApp(root)
	app.config = config.DefaultConfig()
	app.authSessionStore = authsession.NewStore(root)

	if err := app.SaveDesktopAuthSession("token-cleanup", true); err != nil {
		t.Fatalf("prepare persisted session failed: %v", err)
	}

	browserMgr := browser.NewManager(config.DefaultConfig(), root)
	browserMgr.Profiles = map[string]*browser.Profile{
		"1688:shop-managed": {
			ProfileId:   "1688:shop-managed",
			ProfileName: "managed",
			UserDataDir: "managed-profiles/1688__shop-managed",
			Running:     true,
			Tags:        []string{"managed", "managed:desktop", "shop:shop-managed"},
		},
		"1688:shop-regular": {
			ProfileId:   "1688:shop-regular",
			ProfileName: "regular",
			UserDataDir: "profiles/shop-regular",
			Running:     true,
			Tags:        []string{"shop:shop-regular"},
		},
	}
	browserMgr.BrowserProcesses = map[string]*exec.Cmd{}
	app.browserMgr = browserMgr

	var summaryCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/local/health" {
			summaryCalls++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"serverTime": "2026-05-11T00:00:00Z",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	t.Setenv("ANT_BROWSER_WORKSPACE_AGENT_BASE_URL", server.URL)
	t.Setenv("AGENT_BASE_URL", "")

	app.workspaceAgentURL = "http://127.0.0.1:59999"
	app.workspaceService = workspace.NewService(workspace.NewWorkspaceClient("http://127.0.0.1:58888", nil), browserMgr, nil)

	if err := app.DesktopAuthStrongCleanup("logout"); err != nil {
		t.Fatalf("DesktopAuthStrongCleanup 返回错误: %v", err)
	}

	if browserMgr.Profiles["1688:shop-managed"].Running {
		t.Fatalf("expected managed running profile stopped")
	}
	if !browserMgr.Profiles["1688:shop-regular"].Running {
		t.Fatalf("expected non-managed running profile untouched")
	}

	loaded, err := app.LoadDesktopAuthSession()
	if err != nil {
		t.Fatalf("LoadDesktopAuthSession after cleanup 返回错误: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil session after cleanup")
	}
	if loaded.AccessToken != "" || loaded.RememberMe {
		t.Fatalf("expected persisted auth session cleared, got %#v", loaded)
	}

	if app.workspaceAgentURL != "" {
		t.Fatalf("expected workspaceAgentURL reset, got %q", app.workspaceAgentURL)
	}

	if _, err := app.WorkspaceSummary(); err != nil {
		t.Fatalf("expected workspace service base url reset to env override, got err=%v", err)
	}
	if summaryCalls != 1 {
		t.Fatalf("expected workspace summary request after reset, got calls=%d", summaryCalls)
	}
}

func TestBootstrapDesktopAuthRuntimeBootstrapsAndReadsAuthorizedShops(t *testing.T) {
	root := t.TempDir()
	agentEntry := filepath.Join(root, "apps", "agent", "src", "server", "index.mjs")
	if err := os.MkdirAll(filepath.Dir(agentEntry), 0o755); err != nil {
		t.Fatalf("创建 workspace agent 目录失败: %v", err)
	}
	if err := os.WriteFile(agentEntry, []byte("export {};"), 0o644); err != nil {
		t.Fatalf("写入 workspace agent entry 失败: %v", err)
	}

	app := NewApp(root)
	app.config = &config.Config{
		Workspace: config.WorkspaceConfig{
			InstallRoot:  root,
			RuntimeDir:   t.TempDir(),
			ServerOrigin: "http://workspace-server.invalid",
		},
	}
	app.authSessionStore = authsession.NewStore(root)

	if err := app.SaveDesktopAuthSession("persisted-runtime-token", true); err != nil {
		t.Fatalf("保存 desktop auth session 失败: %v", err)
	}

	var meAuthHeader string
	serverOrigin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/auth/me" {
			http.NotFound(w, r)
			return
		}

		meAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "ok",
			"data": map[string]any{
				"id":          "runtime-user-1",
				"displayName": "Runtime User",
			},
		})
	}))
	defer serverOrigin.Close()
	app.config.Workspace.ServerOrigin = serverOrigin.URL

	var bootstrapCalls int
	var shopsCalls int
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		case r.Method == http.MethodPost && r.URL.Path == "/local/session/bootstrap":
			bootstrapCalls++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"agentSessionId": "agent-session-1",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/local/shops":
			shopsCalls++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"items": []any{},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer agentServer.Close()

	app.workspaceAgentURL = agentServer.URL
	app.workspaceService = workspace.NewService(workspace.NewWorkspaceClient(agentServer.URL, nil), nil, nil)

	if err := app.BootstrapDesktopAuthRuntime(); err != nil {
		t.Fatalf("BootstrapDesktopAuthRuntime 返回错误: %v", err)
	}

	if meAuthHeader != "Bearer persisted-runtime-token" {
		t.Fatalf("期望 auth/me 使用 Bearer persisted-runtime-token，实际=%q", meAuthHeader)
	}
	if bootstrapCalls != 1 {
		t.Fatalf("期望 bootstrap 调用 1 次，实际=%d", bootstrapCalls)
	}
	if shopsCalls < 2 {
		t.Fatalf("期望 shops 至少被读取 2 次（warmup + 显式校验），实际=%d", shopsCalls)
	}
}
