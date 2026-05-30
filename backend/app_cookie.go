package backend

import (
	"ant-chrome/backend/internal/workspace"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// ============================================================================
// Cookie 管理 API（通过 CDP）
// ============================================================================

// CookieInfo 表示单条浏览器 Cookie
type CookieInfo struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HttpOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite"`
}

// cdpTarget 表示 /json 接口返回的调试目标
type cdpTarget struct {
	ID                   string `json:"id"`
	WebSocketDebuggerUrl string `json:"webSocketDebuggerUrl"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
}

type cdpBrowserVersion struct {
	WebSocketDebuggerUrl string `json:"webSocketDebuggerUrl"`
}

// cdpMessage 是 CDP 协议消息结构
type cdpMessage struct {
	Id     int            `json:"id"`
	Method string         `json:"method,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

// cdpResponse 是 CDP 协议响应结构
type cdpResponse struct {
	Id     int            `json:"id"`
	Result map[string]any `json:"result,omitempty"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// cdpCall 向指定 debugPort 发送单次 CDP 命令并返回 result 字段
func cdpCall(debugPort int, method string, params map[string]any) (map[string]any, error) {
	// 1. 获取 WebSocket 调试地址
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", debugPort))
	if err != nil {
		return nil, fmt.Errorf("CDP /json 请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil || len(targets) == 0 {
		return nil, fmt.Errorf("CDP targets 解析失败或为空")
	}

	wsURL := ""
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerUrl != "" {
			wsURL = t.WebSocketDebuggerUrl
			break
		}
	}
	if wsURL == "" && targets[0].WebSocketDebuggerUrl != "" {
		wsURL = targets[0].WebSocketDebuggerUrl
	}
	if wsURL == "" {
		return nil, fmt.Errorf("未找到可用的 WebSocket 调试地址")
	}

	// 2. 建立 WebSocket 连接
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("WebSocket 连接失败: %w", err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// 3. 发送 CDP 命令
	msg := cdpMessage{Id: 1, Method: method, Params: params}
	if err := conn.WriteJSON(msg); err != nil {
		return nil, fmt.Errorf("CDP 命令发送失败: %w", err)
	}

	// 4. 等待响应
	var cdpResp cdpResponse
	if err := conn.ReadJSON(&cdpResp); err != nil {
		return nil, fmt.Errorf("CDP 响应读取失败: %w", err)
	}
	if cdpResp.Error != nil {
		return nil, fmt.Errorf("CDP 错误: %s", cdpResp.Error.Message)
	}
	return cdpResp.Result, nil
}

func cdpBrowserCall(debugPort int, method string, params map[string]any) error {
	_, err := cdpBrowserCallWithResult(debugPort, method, params)
	return err
}

func cdpBrowserCallWithResult(debugPort int, method string, params map[string]any) (map[string]any, error) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", debugPort))
	if err != nil {
		return nil, fmt.Errorf("CDP /json/version 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var version cdpBrowserVersion
	if err := json.Unmarshal(body, &version); err != nil {
		return nil, fmt.Errorf("CDP browser target 解析失败: %w", err)
	}
	wsURL := strings.TrimSpace(version.WebSocketDebuggerUrl)
	if wsURL == "" {
		return nil, fmt.Errorf("未找到浏览器级 WebSocket 调试地址")
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("浏览器级 WebSocket 连接失败: %w", err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	msg := cdpMessage{Id: 1, Method: method, Params: params}
	if err := conn.WriteJSON(msg); err != nil {
		return nil, fmt.Errorf("浏览器级 CDP 命令发送失败: %w", err)
	}

	var cdpResp cdpResponse
	if err := conn.ReadJSON(&cdpResp); err != nil {
		// Browser.close 可能会直接关闭 websocket，视为成功。
		if strings.EqualFold(method, "Browser.close") {
			return nil, nil
		}
		return nil, fmt.Errorf("浏览器级 CDP 响应读取失败: %w", err)
	}
	if cdpResp.Error != nil {
		return nil, fmt.Errorf("浏览器级 CDP 错误: %s", cdpResp.Error.Message)
	}
	return cdpResp.Result, nil
}

// getDebugPort 获取运行中实例的调试端口
func (a *App) getDebugPort(profileId string) (int, error) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		return 0, fmt.Errorf("profile not found: %s", profileId)
	}
	if !profile.Running {
		return 0, fmt.Errorf("实例未运行")
	}
	if profile.DebugPort == 0 || !profile.DebugReady {
		return 0, fmt.Errorf("实例调试接口尚未就绪，请稍后重试")
	}
	return profile.DebugPort, nil
}

// BrowserGetCookies 通过 CDP 获取实例所有 Cookie
func (a *App) BrowserGetCookies(profileId string) ([]CookieInfo, error) {
	debugPort, err := a.getDebugPort(profileId)
	if err != nil {
		return nil, err
	}

	result, err := cdpCall(debugPort, "Network.getAllCookies", nil)
	if err != nil {
		return nil, err
	}

	cookiesRaw, ok := result["cookies"]
	if !ok {
		return []CookieInfo{}, nil
	}

	// 通过 JSON 二次解析
	data, _ := json.Marshal(cookiesRaw)
	var cookies []CookieInfo
	if err := json.Unmarshal(data, &cookies); err != nil {
		return nil, fmt.Errorf("Cookie 解析失败: %w", err)
	}
	return cookies, nil
}

// BrowserClearCookies 通过 CDP 清除实例所有 Cookie
func (a *App) BrowserClearCookies(profileId string) error {
	debugPort, err := a.getDebugPort(profileId)
	if err != nil {
		return err
	}
	_, err = cdpCall(debugPort, "Network.clearBrowserCookies", nil)
	return err
}

// BrowserExportCookies 导出 Netscape 格式 Cookie 字符串
func (a *App) BrowserExportCookies(profileId string) (string, error) {
	cookies, err := a.BrowserGetCookies(profileId)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("# Netscape HTTP Cookie File\n")
	sb.WriteString("# Generated by BrowserManager\n\n")

	for _, c := range cookies {
		includeSubdomains := "FALSE"
		if strings.HasPrefix(c.Domain, ".") {
			includeSubdomains = "TRUE"
		}
		secure := "FALSE"
		if c.Secure {
			secure = "TRUE"
		}
		expires := int64(c.Expires)
		if expires < 0 {
			expires = 0
		}
		sb.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			c.Domain, includeSubdomains, c.Path, secure, expires, c.Name, c.Value))
	}
	return sb.String(), nil
}

func (a *App) browserImportCookies(profileID string, cookies []workspace.SessionCookie) error {
	if len(cookies) == 0 {
		return nil
	}

	debugPort, err := a.getDebugPort(profileID)
	if err != nil {
		return err
	}

	if _, err := cdpCall(debugPort, "Network.enable", map[string]any{}); err != nil {
		return err
	}

	payload := make([]map[string]any, 0, len(cookies))
	for _, cookie := range cookies {
		item := map[string]any{
			"name":     strings.TrimSpace(cookie.Name),
			"value":    cookie.Value,
			"domain":   strings.TrimSpace(cookie.Domain),
			"path":     defaultString(cookie.Path, "/"),
			"httpOnly": cookie.HttpOnly,
			"secure":   cookie.Secure,
		}
		if cookie.Expires > 0 {
			item["expires"] = cookie.Expires
		}
		if sameSite := strings.TrimSpace(cookie.SameSite); sameSite != "" {
			item["sameSite"] = sameSite
		}
		if domain := strings.TrimSpace(cookie.Domain); domain == "" {
			if url := strings.TrimSpace(cookie.URL); url != "" {
				item["url"] = url
			}
		}
		payload = append(payload, item)
	}

	_, err = cdpCall(debugPort, "Network.setCookies", map[string]any{
		"cookies": payload,
	})
	return err
}

func (a *App) browserRuntimeSnapshot(profileID string) (workspace.OpenRuntimeSnapshot, error) {
	snapshots, err := a.browserRuntimeSnapshots(profileID)
	if err != nil {
		return workspace.OpenRuntimeSnapshot{}, err
	}
	if len(snapshots) == 0 {
		return workspace.OpenRuntimeSnapshot{}, fmt.Errorf("未找到可用页面")
	}
	return snapshots[0], nil
}

func (a *App) browserRuntimeSnapshots(profileID string) ([]workspace.OpenRuntimeSnapshot, error) {
	targets, err := a.browserRuntimeTargets(profileID)
	if err != nil {
		return nil, err
	}

	snapshots := make([]workspace.OpenRuntimeSnapshot, 0, len(targets))
	for _, target := range targets {
		snapshots = append(snapshots, workspace.OpenRuntimeSnapshot{
			CurrentURL: strings.TrimSpace(target.CurrentURL),
			PageTitle:  strings.TrimSpace(target.PageTitle),
		})
	}
	return snapshots, nil
}

func (a *App) browserRuntimeTargets(profileID string) ([]workspace.OpenRuntimeTarget, error) {
	debugPort, err := a.getDebugPort(profileID)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 2 * time.Second}
	var targets []cdpTarget
	if err := fetchBrowserDebugJSON(client, debugPort, "/json/list", &targets); err != nil {
		return nil, err
	}

	runtimeTargets := make([]workspace.OpenRuntimeTarget, 0, len(targets))
	for _, target := range targets {
		if strings.TrimSpace(target.Type) != "page" {
			continue
		}
		runtimeTargets = append(runtimeTargets, workspace.OpenRuntimeTarget{
			TargetID:   strings.TrimSpace(target.ID),
			CurrentURL: strings.TrimSpace(target.URL),
			PageTitle:  strings.TrimSpace(target.Title),
		})
	}
	return runtimeTargets, nil
}

func (a *App) browserNavigate(profileID string, targetURL string) error {
	debugPort, err := a.getDebugPort(profileID)
	if err != nil {
		return err
	}

	if _, err := cdpCall(debugPort, "Page.enable", map[string]any{}); err != nil {
		return err
	}
	_, err = cdpCall(debugPort, "Page.navigate", map[string]any{
		"url": strings.TrimSpace(targetURL),
	})
	return err
}

func cdpCallTarget(debugPort int, targetID string, method string, params map[string]any) (map[string]any, error) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", debugPort))
	if err != nil {
		return nil, fmt.Errorf("CDP /json 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return nil, fmt.Errorf("CDP targets 解析失败: %w", err)
	}

	targetID = strings.TrimSpace(targetID)
	availableTargetIDs := make([]string, 0, len(targets))
	for _, target := range targets {
		availableTargetIDs = append(availableTargetIDs, strings.TrimSpace(target.ID))
		if strings.TrimSpace(target.ID) != targetID || strings.TrimSpace(target.WebSocketDebuggerUrl) == "" {
			continue
		}
		conn, _, err := websocket.DefaultDialer.Dial(target.WebSocketDebuggerUrl, nil)
		if err != nil {
			return nil, fmt.Errorf("WebSocket 连接失败: %w", err)
		}
		defer conn.Close()
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		msg := cdpMessage{Id: 1, Method: method, Params: params}
		if err := conn.WriteJSON(msg); err != nil {
			return nil, fmt.Errorf("CDP 命令发送失败: %w", err)
		}

		var cdpResp cdpResponse
		if err := conn.ReadJSON(&cdpResp); err != nil {
			return nil, fmt.Errorf("CDP 响应读取失败: %w", err)
		}
		if cdpResp.Error != nil {
			return nil, fmt.Errorf("CDP 错误: %s", cdpResp.Error.Message)
		}
		return cdpResp.Result, nil
	}
	return nil, fmt.Errorf("未找到目标页面: %s（method=%s available=%s）", targetID, strings.TrimSpace(method), strings.Join(availableTargetIDs, ","))
}

func cdpEvaluateString(debugPort int, expression string) (string, error) {
	result, err := cdpCall(debugPort, "Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
	})
	if err != nil {
		return "", err
	}

	valueNode, ok := result["result"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("CDP Runtime.evaluate 返回结构无效")
	}
	value, _ := valueNode["value"].(string)
	return strings.TrimSpace(value), nil
}

func (a *App) importWorkspaceSessionBundle(profileID string, bundle workspace.SessionBundle) error {
	if len(bundle.Cookies) > 0 {
		if err := a.browserImportCookies(profileID, bundle.Cookies); err != nil {
			return err
		}
	}
	if len(bundle.Storages) > 0 {
		if err := a.browserImportStorages(profileID, bundle.Storages); err != nil {
			return err
		}
	}
	return nil
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func (a *App) browserImportStorages(profileID string, storages []workspace.SessionStorageEntry) error {
	debugPort, err := a.getDebugPort(profileID)
	if err != nil {
		return err
	}

	for _, entry := range storages {
		origin := strings.TrimSpace(entry.Origin)
		if origin == "" {
			continue
		}
		expression, err := buildStorageInjectionExpression(origin, entry)
		if err != nil {
			return err
		}
		if _, err := cdpEvaluateString(debugPort, expression); err != nil {
			return fmt.Errorf("注入 storage 失败（origin=%s）: %w", origin, err)
		}
	}
	return nil
}

func buildStorageInjectionExpression(origin string, entry workspace.SessionStorageEntry) (string, error) {
	originJSON, err := json.Marshal(origin)
	if err != nil {
		return "", err
	}
	localJSON, err := json.Marshal(entry.LocalStorage)
	if err != nil {
		return "", err
	}
	sessionJSON, err := json.Marshal(entry.SessionStorage)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`(() => {
  const targetOrigin = %s;
  if (window.location.origin !== targetOrigin) {
    window.location.href = targetOrigin;
    return "origin_navigate_requested";
  }
  const localStoragePayload = %s || {};
  const sessionStoragePayload = %s || {};
  for (const [key, value] of Object.entries(localStoragePayload)) {
    window.localStorage.setItem(key, String(value));
  }
  for (const [key, value] of Object.entries(sessionStoragePayload)) {
    window.sessionStorage.setItem(key, String(value));
  }
  return "storage_injected";
})()`, string(originJSON), string(localJSON), string(sessionJSON)), nil
}
