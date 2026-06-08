package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/managedinstance"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/workspace"

	"github.com/gorilla/websocket"
)

func TestUpsertManagedProfilePrefersFingerprintCoreOverSystemChromeDefault(t *testing.T) {
	t.Parallel()

	appRoot := t.TempDir()
	systemCoreRoot := t.TempDir()
	systemExe := filepath.Join(systemCoreRoot, filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(systemExe), 0o755); err != nil {
		t.Fatalf("create system core dir: %v", err)
	}
	if err := os.WriteFile(systemExe, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write system core exe: %v", err)
	}

	fingerprintCorePath := "chrome/fingerprint-core"
	fingerprintExe := filepath.Join(appRoot, fingerprintCorePath, filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(fingerprintExe), 0o755); err != nil {
		t.Fatalf("create fingerprint core dir: %v", err)
	}
	if err := os.WriteFile(fingerprintExe, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write fingerprint core exe: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Browser.Cores = []config.BrowserCore{
		{
			CoreId:    "system-chrome",
			CoreName:  "System Chrome",
			CorePath:  systemCoreRoot,
			IsDefault: true,
		},
		{
			CoreId:   "fingerprint-core",
			CoreName: "Fingerprint Chromium",
			CorePath: fingerprintCorePath,
		},
	}
	browserMgr := browser.NewManager(cfg, appRoot)
	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: browserMgr})
	if err != nil {
		t.Fatalf("new managed service: %v", err)
	}
	app := &App{
		browserMgr:             browserMgr,
		managedInstanceService: service,
	}

	result, err := app.UpsertManagedProfile(launchcode.ManagedProfileUpsertInput{
		ProfileID:    "alibaba:b2b-2220216067652a3709",
		ShopID:       "b2b-2220216067652a3709",
		PlatformCode: "alibaba",
		ProfileName:  "艾优品供应链",
		UserDataDir:  "managed-profiles/alibaba__b2b-2220216067652a3709",
		ManagedMode:  true,
	})
	if err != nil {
		t.Fatalf("upsert managed profile: %v", err)
	}
	if result == nil || result.ProfileID != "alibaba:b2b-2220216067652a3709" {
		t.Fatalf("unexpected result: %+v", result)
	}
	profile := browserMgr.Profiles["alibaba:b2b-2220216067652a3709"]
	if profile == nil {
		t.Fatal("expected profile created")
	}
	if profile.CoreId != "fingerprint-core" {
		t.Fatalf("expected fingerprint core, got %s", profile.CoreId)
	}
}

func TestInjectManagedSessionBundleImportsCookiesAndStorages(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		methods []string
	)
	upgrader := websocket.Upgrader{}
	server := startDevToolsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json":
			_, _ = w.Write([]byte(`[{"id":"page-1","type":"page","webSocketDebuggerUrl":"ws://127.0.0.1:` + itoa(serverPortFromRequestHost(r.Host)) + `/devtools/page/page-1"}]`))
		case "/devtools/page/page-1":
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Errorf("升级 websocket 失败: %v", err)
				return
			}
			defer conn.Close()

			var msg cdpMessage
			if err := conn.ReadJSON(&msg); err != nil {
				t.Errorf("读取 CDP 消息失败: %v", err)
				return
			}

			mu.Lock()
			methods = append(methods, strings.TrimSpace(msg.Method))
			mu.Unlock()

			if err := conn.WriteJSON(cdpResponse{
				Id:     msg.Id,
				Result: map[string]any{"result": map[string]any{"value": "ok"}},
			}); err != nil {
				t.Errorf("写入 CDP 响应失败: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app := &App{
		browserMgr: &browser.Manager{
			Profiles: map[string]*browser.Profile{
				"1688:test-shop": {
					ProfileId:  "1688:test-shop",
					Running:    true,
					DebugReady: true,
					DebugPort:  server.port,
				},
			},
		},
	}

	err := app.InjectManagedSessionBundle("1688:test-shop", workspace.SessionBundle{
		Cookies: []workspace.SessionCookie{
			{
				Name:   "sid",
				Value:  "cookie-1",
				Domain: ".1688.com",
				Path:   "/",
			},
		},
		Storages: []workspace.SessionStorageEntry{
			{
				Origin:       "https://work.1688.com",
				LocalStorage: map[string]string{"k": "v"},
			},
		},
	})
	if err != nil {
		t.Fatalf("InjectManagedSessionBundle 返回错误: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	payload, err := json.Marshal(methods)
	if err != nil {
		t.Fatalf("序列化 methods 失败: %v", err)
	}
	if !containsMethod(methods, "Network.enable") {
		t.Fatalf("期望注入 cookies 前启用 network，实际=%s", string(payload))
	}
	if !containsMethod(methods, "Network.setCookies") {
		t.Fatalf("期望注入 cookies，实际=%s", string(payload))
	}
	if !containsMethod(methods, "Runtime.evaluate") {
		t.Fatalf("期望注入 storages，实际=%s", string(payload))
	}
}

func containsMethod(methods []string, want string) bool {
	for _, method := range methods {
		if strings.TrimSpace(method) == want {
			return true
		}
	}
	return false
}

func serverPortFromRequestHost(host string) int {
	host = strings.TrimSpace(host)
	lastColon := strings.LastIndex(host, ":")
	if lastColon <= 0 || lastColon+1 >= len(host) {
		return 0
	}
	port := 0
	for _, char := range host[lastColon+1:] {
		if char < '0' || char > '9' {
			return 0
		}
		port = port*10 + int(char-'0')
	}
	return port
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	buf := [20]byte{}
	index := len(buf)
	for value > 0 {
		index--
		buf[index] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[index:])
}
