package backend

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/workspace"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultWorkspaceBaseURL = "http://127.0.0.1:4174"

type serverConnectionConfig struct {
	ServerProtocol string `json:"serverProtocol"`
	ServerIP       string `json:"serverIp"`
	ServerPort     int    `json:"serverPort"`
}

func (a *App) WorkspaceSummary() (*workspace.WorkspaceSummary, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return a.workspaceService.FetchSummary(context.Background())
}

func (a *App) WorkspaceAuthorizedShops() ([]workspace.ShopInstanceProjection, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	return a.workspaceService.FetchAuthorizedShops(context.Background())
}

func (a *App) initWorkspaceService() {
	baseURL := resolveWorkspaceBaseURL(a.appRoot)
	client := workspace.NewWorkspaceClient(baseURL, nil)
	a.workspaceService = workspace.NewService(client, a.browserMgr)
}

func resolveWorkspaceBaseURL(appRoot string) string {
	for _, path := range workspaceServerConnectionConfigCandidates(appRoot) {
		baseURL, ok := readWorkspaceBaseURLFromConfig(path)
		if ok {
			return baseURL
		}
	}
	return defaultWorkspaceBaseURL
}

func workspaceServerConnectionConfigCandidates(appRoot string) []string {
	stateRoot := apppath.StateRoot(appRoot)
	installRoot := apppath.InstallRoot(appRoot)

	candidates := []string{
		filepath.Join(stateRoot, "runtime", "config", "server-connection.json"),
	}
	if installRoot != stateRoot {
		candidates = append(candidates, filepath.Join(installRoot, "runtime", "config", "server-connection.json"))
	}
	return candidates
}

func readWorkspaceBaseURLFromConfig(configPath string) (string, bool) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return "", false
	}

	var cfg serverConnectionConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", false
	}

	protocol := strings.ToLower(strings.TrimSpace(cfg.ServerProtocol))
	if protocol != "https" {
		protocol = "http"
	}

	host := strings.TrimSpace(cfg.ServerIP)
	if host == "" {
		host = "127.0.0.1"
	}

	port := cfg.ServerPort
	if port <= 0 {
		port = 4174
	}

	return protocol + "://" + host + ":" + strconv.Itoa(port), true
}
