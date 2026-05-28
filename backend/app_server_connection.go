package backend

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type DesktopServerConnection struct {
	ServerOrigin string `json:"serverOrigin"`
	Source       string `json:"source"`
	ConfigPath   string `json:"configPath"`
}

func (a *App) GetDesktopServerConnection() (*DesktopServerConnection, error) {
	runtimeDir := resolveWorkspaceRuntimeDirWithConfig(a.config)
	resolution := resolveWorkspaceServerOriginDetails(runtimeDir, a.config)
	return &DesktopServerConnection{
		ServerOrigin: strings.TrimRight(strings.TrimSpace(resolution.Origin), "/"),
		Source:       strings.TrimSpace(resolution.Source),
		ConfigPath:   strings.TrimSpace(resolution.ConfigPath),
	}, nil
}

func (a *App) SaveDesktopServerConnection(serverOrigin string) (*DesktopServerConnection, error) {
	normalizedOrigin, err := normalizeDesktopServerOrigin(serverOrigin)
	if err != nil {
		return nil, err
	}

	runtimeDir := resolveWorkspaceRuntimeDirWithConfig(a.config)
	configPath := filepath.Join(runtimeDir, "config", "server-connection.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return nil, fmt.Errorf("创建服务端连接配置目录失败: %w", err)
	}

	payload, err := json.MarshalIndent(workspaceServerConnectionConfig{
		ServerOrigin: normalizedOrigin,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化服务端连接配置失败: %w", err)
	}
	payload = append(payload, '\n')

	if err := os.WriteFile(configPath, payload, 0o600); err != nil {
		return nil, fmt.Errorf("写入服务端连接配置失败: %w", err)
	}

	return a.GetDesktopServerConnection()
}

func normalizeDesktopServerOrigin(input string) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", fmt.Errorf("服务端地址不能为空")
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("服务端地址格式无效，请输入 http://IP:端口 或 https://域名")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("服务端地址仅支持 http 或 https")
	}
	if strings.TrimSpace(parsed.Path) != "" && parsed.Path != "/" {
		return "", fmt.Errorf("请输入服务端根地址，不要包含接口路径")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("请输入服务端根地址，不要包含 query 或 fragment")
	}

	return strings.TrimRight(parsed.Scheme+"://"+parsed.Host, "/"), nil
}
