package backend

import (
	"net/http"
	"os"
	"os/exec"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
)

func TestRecoverRunningProfilesFromUserDataDirsMarksProfileRunning(t *testing.T) {
	t.Parallel()

	server := startDevToolsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/version":
			_, _ = w.Write([]byte(`{"Browser":"Chrome/142.0","webSocketDebuggerUrl":"ws://127.0.0.1/devtools/browser"}`))
		case "/json/list":
			_, _ = w.Write([]byte(`[{"id":"page-1"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	cfg := config.DefaultConfig()
	mgr := browser.NewManager(cfg, root)
	mgr.Profiles = map[string]*browser.Profile{
		"alibaba:test-shop": {
			ProfileId:   "alibaba:test-shop",
			ProfileName: "test-shop",
			UserDataDir: "managed-profiles/alibaba__test-shop",
			Running:     false,
		},
	}
	mgr.BrowserProcesses = map[string]*exec.Cmd{}

	app := &App{
		browserMgr:   mgr,
		launchServer: launchcode.NewLaunchServer(nil, nil, mgr, 0),
	}

	userDataDir := mgr.ResolveUserDataDir(mgr.Profiles["alibaba:test-shop"])
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		t.Fatalf("创建用户数据目录失败: %v", err)
	}
	writeDevToolsActivePortFile(t, userDataDir, server.port)

	recovered := app.recoverRunningProfilesFromUserDataDirs()
	if recovered != 1 {
		t.Fatalf("期望恢复 1 个运行中实例，实际=%d", recovered)
	}

	profile := mgr.Profiles["alibaba:test-shop"]
	if !profile.Running {
		t.Fatal("期望 profile 被标记为运行中")
	}
	if !profile.DebugReady {
		t.Fatal("期望 profile 调试接口已就绪")
	}
	if profile.DebugPort != server.port {
		t.Fatalf("期望恢复调试端口 %d，实际=%d", server.port, profile.DebugPort)
	}
	if app.launchServer.ActiveDebugPort() != server.port {
		t.Fatalf("期望 launch server 激活调试端口 %d，实际=%d", server.port, app.launchServer.ActiveDebugPort())
	}
}

func TestRecoverRunningProfilesFromUserDataDirsClearsStaleRunningProfile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.DefaultConfig()
	mgr := browser.NewManager(cfg, root)
	mgr.Profiles = map[string]*browser.Profile{
		"alibaba:test-shop": {
			ProfileId:   "alibaba:test-shop",
			ProfileName: "test-shop",
			UserDataDir: "managed-profiles/alibaba__test-shop",
			Running:     true,
			DebugReady:   true,
			DebugPort:    65534,
			Pid:          0,
		},
	}
	mgr.BrowserProcesses = map[string]*exec.Cmd{}

	app := &App{
		browserMgr:   mgr,
		launchServer: launchcode.NewLaunchServer(nil, nil, mgr, 0),
	}

	recovered := app.recoverRunningProfilesFromUserDataDirs()
	if recovered != 0 {
		t.Fatalf("期望不恢复已死亡实例，实际恢复=%d", recovered)
	}

	profile := mgr.Profiles["alibaba:test-shop"]
	if profile.Running {
		t.Fatal("期望已死亡 profile 被标记为停止")
	}
	if profile.DebugReady {
		t.Fatal("期望已死亡 profile 清除调试就绪状态")
	}
	if profile.DebugPort != 0 {
		t.Fatalf("期望已死亡 profile 清除调试端口，实际=%d", profile.DebugPort)
	}
	if profile.Pid != 0 {
		t.Fatalf("期望已死亡 profile 清除 pid，实际=%d", profile.Pid)
	}
}
