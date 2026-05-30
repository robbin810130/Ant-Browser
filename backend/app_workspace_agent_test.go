package backend

import (
	"ant-chrome/backend/internal/authsession"
	"ant-chrome/backend/internal/config"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveWorkspaceInstallRootPrefersExplicitEnv(t *testing.T) {
	appRoot := t.TempDir()
	explicitRoot := t.TempDir()
	createWorkspaceAgentEntry(t, explicitRoot)

	t.Setenv("ANT_BROWSER_WORKSPACE_INSTALL_ROOT", explicitRoot)
	t.Setenv("WORKSPACE_INSTALL_ROOT", "")

	got, err := resolveWorkspaceInstallRoot(appRoot)
	if err != nil {
		t.Fatalf("resolveWorkspaceInstallRoot 返回错误: %v", err)
	}
	if got != explicitRoot {
		t.Fatalf("期望优先返回显式配置目录 %s，实际=%s", explicitRoot, got)
	}
}

func TestResolveWorkspaceInstallRootRejectsInvalidExplicitEnv(t *testing.T) {
	appRoot := t.TempDir()
	fallbackRoot := filepath.Join(appRoot, "workspace")
	createWorkspaceAgentEntry(t, fallbackRoot)

	t.Setenv("ANT_BROWSER_WORKSPACE_INSTALL_ROOT", filepath.Join(t.TempDir(), "missing-root"))
	t.Setenv("WORKSPACE_INSTALL_ROOT", "")

	_, err := resolveWorkspaceInstallRoot(filepath.Join(fallbackRoot, "nested", "app"))
	if err == nil {
		t.Fatal("期望显式配置无效时直接报错")
	}
	if !strings.Contains(err.Error(), "ANT_BROWSER_WORKSPACE_INSTALL_ROOT") {
		t.Fatalf("期望错误信息带上环境变量名，实际=%v", err)
	}
}

func TestResolveWorkspaceInstallRootFallsBackToAppRootDiscovery(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, "desktop-repos", "1688shop-desktop")
	createWorkspaceAgentEntry(t, installRoot)

	t.Setenv("ANT_BROWSER_WORKSPACE_INSTALL_ROOT", "")
	t.Setenv("WORKSPACE_INSTALL_ROOT", "")

	got, err := resolveWorkspaceInstallRoot(filepath.Join(installRoot, "apps", "desktop"))
	if err != nil {
		t.Fatalf("resolveWorkspaceInstallRoot 返回错误: %v", err)
	}
	if got != installRoot {
		t.Fatalf("期望通过 appRoot 向上发现 install root=%s，实际=%s", installRoot, got)
	}
}

func TestResolveWorkspaceInstallRootPrefersConfigWhenEnvUnset(t *testing.T) {
	appRoot := t.TempDir()
	installRoot := t.TempDir()
	createWorkspaceAgentEntry(t, installRoot)

	t.Setenv("ANT_BROWSER_WORKSPACE_INSTALL_ROOT", "")
	t.Setenv("WORKSPACE_INSTALL_ROOT", "")

	got, err := resolveWorkspaceInstallRootWithConfig(appRoot, &config.Config{
		Workspace: config.WorkspaceConfig{
			InstallRoot: installRoot,
		},
	})
	if err != nil {
		t.Fatalf("resolveWorkspaceInstallRootWithConfig 返回错误: %v", err)
	}
	if got != installRoot {
		t.Fatalf("期望优先返回配置文件中的 install root=%s，实际=%s", installRoot, got)
	}
}

func TestResolveWorkspaceInstallRootRejectsInvalidConfigWhenEnvUnset(t *testing.T) {
	appRoot := t.TempDir()
	t.Setenv("ANT_BROWSER_WORKSPACE_INSTALL_ROOT", "")
	t.Setenv("WORKSPACE_INSTALL_ROOT", "")

	_, err := resolveWorkspaceInstallRootWithConfig(appRoot, &config.Config{
		Workspace: config.WorkspaceConfig{
			InstallRoot: filepath.Join(t.TempDir(), "missing-root"),
		},
	})
	if err == nil {
		t.Fatal("期望配置文件中的 install root 无效时直接报错")
	}
	if !strings.Contains(err.Error(), "config.workspace.install_root") {
		t.Fatalf("期望错误信息带上配置路径，实际=%v", err)
	}
}

func createWorkspaceAgentEntry(t *testing.T, root string) {
	t.Helper()

	entry := filepath.Join(root, "apps", "agent", "src", "server", "index.mjs")
	if err := os.MkdirAll(filepath.Dir(entry), 0o755); err != nil {
		t.Fatalf("创建 agent 目录失败: %v", err)
	}
	if err := os.WriteFile(entry, []byte("export {}\n"), 0o644); err != nil {
		t.Fatalf("写入 agent 入口失败: %v", err)
	}
}

func TestResolveWorkspaceServerOriginFallsBackToDefault(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("DESKTOP_SERVER_BASE_URL", "")

	got := resolveWorkspaceServerOrigin(runtimeDir)
	if got != defaultWorkspaceServerOrigin {
		t.Fatalf("期望默认 server origin=%s，实际=%s", defaultWorkspaceServerOrigin, got)
	}
}

func TestResolveWorkspaceServerOriginPrefersEnvOverDefault(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("DESKTOP_SERVER_BASE_URL", " http://127.0.0.1:4317/ ")

	got := resolveWorkspaceServerOrigin(runtimeDir)
	if got != "http://127.0.0.1:4317" {
		t.Fatalf("期望读取环境变量中的 server origin，实际=%s", got)
	}
}

func TestResolveWorkspaceServerOriginPrefersConfigOverEnv(t *testing.T) {
	runtimeDir := t.TempDir()
	configPath := filepath.Join(runtimeDir, "config", "server-connection.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 runtime config 目录失败: %v", err)
	}
	payload, err := json.Marshal(workspaceServerConnectionConfig{ServerOrigin: "http://127.0.0.1:5123/"})
	if err != nil {
		t.Fatalf("序列化 server config 失败: %v", err)
	}
	if err := os.WriteFile(configPath, payload, 0o644); err != nil {
		t.Fatalf("写入 server config 失败: %v", err)
	}
	t.Setenv("DESKTOP_SERVER_BASE_URL", "http://127.0.0.1:4317")

	got := resolveWorkspaceServerOrigin(runtimeDir)
	if got != "http://127.0.0.1:5123" {
		t.Fatalf("期望优先读取配置文件中的 server origin，实际=%s", got)
	}

	details := resolveWorkspaceServerOriginDetails(runtimeDir, nil)
	if details.Source != "runtime-config" {
		t.Fatalf("期望来源为 runtime-config，实际=%s", details.Source)
	}
	if filepath.Clean(details.ConfigPath) != filepath.Clean(configPath) {
		t.Fatalf("期望返回 configPath=%s，实际=%s", configPath, details.ConfigPath)
	}
}

func TestResolveWorkspaceServerOriginFallsBackToAppConfigWhenRuntimeAndEnvUnset(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("DESKTOP_SERVER_BASE_URL", "")

	got := resolveWorkspaceServerOriginWithConfig(runtimeDir, &config.Config{
		Workspace: config.WorkspaceConfig{
			ServerOrigin: "http://127.0.0.1:4317/",
		},
	})
	if got != "http://127.0.0.1:4317" {
		t.Fatalf("期望读取 app config 中的 server origin，实际=%s", got)
	}

	details := resolveWorkspaceServerOriginDetails(runtimeDir, &config.Config{
		Workspace: config.WorkspaceConfig{
			ServerOrigin: "http://127.0.0.1:4317/",
		},
	})
	if details.Source != "config.yaml" {
		t.Fatalf("期望来源为 config.yaml，实际=%s", details.Source)
	}
}

func TestResolveWorkspaceServerOriginDetailsFallsBackToDefault(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("DESKTOP_SERVER_BASE_URL", "")

	details := resolveWorkspaceServerOriginDetails(runtimeDir, nil)
	if details.Origin != defaultWorkspaceServerOrigin {
		t.Fatalf("期望默认 origin=%s，实际=%s", defaultWorkspaceServerOrigin, details.Origin)
	}
	if details.Source != "default" {
		t.Fatalf("期望默认来源=default，实际=%s", details.Source)
	}
}

func TestSaveDesktopServerConnectionPersistsRuntimeConfig(t *testing.T) {
	runtimeDir := t.TempDir()
	appRoot := t.TempDir()
	app := NewApp(appRoot)
	app.config = &config.Config{
		Workspace: config.WorkspaceConfig{
			RuntimeDir: runtimeDir,
		},
	}
	t.Setenv("DESKTOP_SERVER_BASE_URL", "http://127.0.0.1:4174")

	connection, err := app.SaveDesktopServerConnection(" 192.168.210.169:4174/ ")
	if err != nil {
		t.Fatalf("SaveDesktopServerConnection 返回错误: %v", err)
	}
	if connection.ServerOrigin != "http://192.168.210.169:4174" {
		t.Fatalf("期望规范化 server origin，实际=%s", connection.ServerOrigin)
	}
	if connection.Source != "runtime-config" {
		t.Fatalf("期望配置来源为 runtime-config，实际=%s", connection.Source)
	}

	configPath := filepath.Join(runtimeDir, "config", "server-connection.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("读取 server connection config 失败: %v", err)
	}
	var payload workspaceServerConnectionConfig
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("解析 server connection config 失败: %v", err)
	}
	if payload.ServerOrigin != "http://192.168.210.169:4174" {
		t.Fatalf("期望落盘 server origin，实际=%s", payload.ServerOrigin)
	}

	mirrorPath := filepath.Join(appRoot, "runtime", "config", "server-connection.json")
	mirrorData, err := os.ReadFile(mirrorPath)
	if err != nil {
		t.Fatalf("读取 install runtime server connection mirror 失败: %v", err)
	}
	var mirrorPayload workspaceServerConnectionConfig
	if err := json.Unmarshal(mirrorData, &mirrorPayload); err != nil {
		t.Fatalf("解析 install runtime server connection mirror 失败: %v", err)
	}
	if mirrorPayload.ServerOrigin != "http://192.168.210.169:4174" {
		t.Fatalf("期望 mirror 落盘 server origin，实际=%s", mirrorPayload.ServerOrigin)
	}
}

func TestGetDesktopServerConnectionPrefersNewestRuntimeConfigCandidate(t *testing.T) {
	appRoot := t.TempDir()
	runtimeDir := t.TempDir()
	app := NewApp(appRoot)
	app.config = &config.Config{
		Workspace: config.WorkspaceConfig{
			RuntimeDir: runtimeDir,
		},
	}

	stalePath := filepath.Join(runtimeDir, "config", "server-connection.json")
	writeServerConnectionConfigFixture(t, stalePath, "http://192.168.131.123:4174")
	staleTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stalePath, staleTime, staleTime); err != nil {
		t.Fatalf("调整旧 server connection mtime 失败: %v", err)
	}

	freshPath := filepath.Join(appRoot, "runtime", "config", "server-connection.json")
	writeServerConnectionConfigFixture(t, freshPath, "http://192.168.210.169:4174")

	connection, err := app.GetDesktopServerConnection()
	if err != nil {
		t.Fatalf("GetDesktopServerConnection 返回错误: %v", err)
	}
	if connection.ServerOrigin != "http://192.168.210.169:4174" {
		t.Fatalf("期望使用最新写入的 server origin，实际=%s", connection.ServerOrigin)
	}
	if filepath.Clean(connection.ConfigPath) != filepath.Clean(freshPath) {
		t.Fatalf("期望使用 install runtime mirror configPath=%s，实际=%s", freshPath, connection.ConfigPath)
	}
}

func TestSaveDesktopServerConnectionRejectsNonOriginURL(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = &config.Config{
		Workspace: config.WorkspaceConfig{
			RuntimeDir: t.TempDir(),
		},
	}

	_, err := app.SaveDesktopServerConnection("http://192.168.210.169:4174/api/auth/login")
	if err == nil {
		t.Fatal("期望带 path 的 server origin 被拒绝")
	}
	if !strings.Contains(err.Error(), "服务端根地址") {
		t.Fatalf("期望错误提示服务端根地址，实际=%v", err)
	}
}

func writeServerConnectionConfigFixture(t *testing.T, path, serverOrigin string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建 server connection fixture 目录失败: %v", err)
	}
	payload, err := json.Marshal(workspaceServerConnectionConfig{ServerOrigin: serverOrigin})
	if err != nil {
		t.Fatalf("序列化 server connection fixture 失败: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("写入 server connection fixture 失败: %v", err)
	}
}

func TestBootstrapWorkspaceAgentSessionRejectsEmptyAccessToken(t *testing.T) {
	serverOrigin := "http://127.0.0.1:4174"
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("empty token should fail before agent request, got %s %s", r.Method, r.URL.Path)
	}))
	defer agentServer.Close()

	err := bootstrapWorkspaceAgentSession(agentServer.URL, serverOrigin, "", workspaceBootstrapUser{})
	if err == nil {
		t.Fatal("期望空 access token 直接报错")
	}
	if !strings.Contains(err.Error(), "access token") {
		t.Fatalf("期望错误信息包含 access token，实际=%v", err)
	}
}

func TestBootstrapWorkspaceAgentSessionUsesPersistedAccessToken(t *testing.T) {
	serverOrigin := "http://127.0.0.1:4174"
	user := workspaceBootstrapUser{
		UserID:      "user-123",
		DisplayName: "Persisted User",
	}

	var bootstrapBody map[string]any
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/local/session/bootstrap" {
			t.Fatalf("unexpected agent request: %s %s", r.Method, r.URL.Path)
		}
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("读取 bootstrap 请求体失败: %v", err)
		}
		if err := json.Unmarshal(payload, &bootstrapBody); err != nil {
			t.Fatalf("解析 bootstrap 请求体失败: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "ok",
			"data": map[string]any{
				"agentSessionId": "agent-session-1",
			},
		})
	}))
	defer agentServer.Close()

	if err := bootstrapWorkspaceAgentSession(agentServer.URL, serverOrigin, "persisted-token-123", user); err != nil {
		t.Fatalf("bootstrapWorkspaceAgentSession 返回错误: %v", err)
	}

	if got, _ := bootstrapBody["accessToken"].(string); got != "persisted-token-123" {
		t.Fatalf("期望 bootstrap 使用持久化 access token，实际=%q", got)
	}
	userPayload, ok := bootstrapBody["user"].(map[string]any)
	if !ok {
		t.Fatalf("期望 bootstrap 请求包含 user，实际=%#v", bootstrapBody["user"])
	}
	if got, _ := userPayload["userId"].(string); got != user.UserID {
		t.Fatalf("期望 userId=%q，实际=%q", user.UserID, got)
	}
	if got, _ := userPayload["displayName"].(string); got != user.DisplayName {
		t.Fatalf("期望 displayName=%q，实际=%q", user.DisplayName, got)
	}
	if got, _ := bootstrapBody["serverBaseUrl"].(string); got != serverOrigin {
		t.Fatalf("期望 serverBaseUrl=%q，实际=%q", serverOrigin, got)
	}
}

func TestEnsureWorkspaceAgentBootstrappedUsesPersistedDesktopAccessToken(t *testing.T) {
	installRoot := t.TempDir()
	createWorkspaceAgentEntry(t, installRoot)
	runtimeDir := t.TempDir()
	appRoot := t.TempDir()

	var meAuthHeader string
	serverOrigin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/auth/me":
			meAuthHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"id":          "user-from-me",
					"displayName": "Desktop User",
				},
			})
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth/login":
			t.Fatalf("should not call legacy bootstrap login")
		default:
			http.NotFound(w, r)
		}
	}))
	defer serverOrigin.Close()

	var bootstrapBody map[string]any
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		case r.Method == http.MethodPost && r.URL.Path == "/local/session/bootstrap":
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("读取 bootstrap 请求体失败: %v", err)
			}
			if err := json.Unmarshal(payload, &bootstrapBody); err != nil {
				t.Fatalf("解析 bootstrap 请求体失败: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"agentSessionId": "agent-session-1",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/local/shops":
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

	app := NewApp(appRoot)
	app.authSessionStore = authsession.NewStore(appRoot)
	app.config = &config.Config{
		Workspace: config.WorkspaceConfig{
			InstallRoot:  installRoot,
			RuntimeDir:   runtimeDir,
			ServerOrigin: serverOrigin.URL,
			AgentBaseURL: agentServer.URL,
		},
	}
	if err := app.SaveDesktopAuthSession("persisted-token-xyz", true); err != nil {
		t.Fatalf("保存 desktop auth session 失败: %v", err)
	}

	if err := app.ensureWorkspaceAgentBootstrapped(); err != nil {
		t.Fatalf("ensureWorkspaceAgentBootstrapped 返回错误: %v", err)
	}

	if meAuthHeader != "Bearer persisted-token-xyz" {
		t.Fatalf("期望 auth/me 使用 Bearer persisted-token-xyz，实际=%q", meAuthHeader)
	}
	if got, _ := bootstrapBody["accessToken"].(string); got != "persisted-token-xyz" {
		t.Fatalf("期望 bootstrap 使用持久化 access token，实际=%q", got)
	}
	userPayload, ok := bootstrapBody["user"].(map[string]any)
	if !ok {
		t.Fatalf("期望 bootstrap 请求包含 user，实际=%#v", bootstrapBody["user"])
	}
	if got, _ := userPayload["userId"].(string); got != "user-from-me" {
		t.Fatalf("期望 bootstrap userId 取自 auth/me，实际=%q", got)
	}
	if got, _ := userPayload["displayName"].(string); got != "Desktop User" {
		t.Fatalf("期望 bootstrap displayName 取自 auth/me，实际=%q", got)
	}
}
