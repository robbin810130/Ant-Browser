package backend

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/workspace"

	"github.com/gorilla/websocket"
)

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
