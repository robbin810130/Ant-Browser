package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
)

const (
	defaultWorkspaceAgentListenHost   = "127.0.0.1"
	defaultWorkspaceAgentListenPort   = "47831"
	defaultWorkspaceAntRuntimeBaseURL = "http://127.0.0.1:19877"
	defaultWorkspaceBootstrapUsername = "admin"
	defaultWorkspaceBootstrapPassword = "Admin@123"
	workspaceBootstrapTimeout         = 8 * time.Second
)

type workspaceServerConnectionConfig struct {
	ServerOrigin string `json:"serverOrigin"`
	ApiBaseURL   string `json:"apiBaseUrl"`
	ServerIP     string `json:"serverIp"`
	ServerPort   int    `json:"serverPort"`
}

type workspaceLoginEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AccessToken string `json:"accessToken"`
		UserSummary struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"userSummary"`
		User struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"user"`
	} `json:"data"`
}

type workspaceBootstrapEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AgentSessionID string `json:"agentSessionId"`
	} `json:"data"`
}

type workspaceShopsEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Items []json.RawMessage `json:"items"`
	} `json:"data"`
}

func (a *App) ensureWorkspaceAgentBootstrapped() {
	log := logger.New("WorkspaceAgent")

	installRoot, ok := resolveWorkspaceInstallRoot(a.appRoot)
	if !ok {
		log.Warn("workspace install root unavailable", logger.F("app_root", a.appRoot))
		return
	}

	runtimeDir := resolveWorkspaceRuntimeDir()
	serverOrigin := resolveWorkspaceServerOrigin(runtimeDir)
	if strings.TrimSpace(serverOrigin) == "" {
		log.Warn("workspace server origin unavailable", logger.F("runtime_dir", runtimeDir))
		return
	}

	agentBaseURL := resolveWorkspaceLocalAgentBaseURL()
	if !isHTTPReachable(agentBaseURL + "/health") {
		cmd, err := a.startWorkspaceAgentProcess(installRoot, runtimeDir, serverOrigin)
		if err != nil {
			log.Error("start workspace agent failed", logger.F("error", err.Error()))
			return
		}
		a.workspaceAgentCmd = cmd
		if !waitForHTTPReachable(agentBaseURL+"/health", workspaceBootstrapTimeout) {
			log.Error("workspace agent health timeout", logger.F("url", agentBaseURL+"/health"))
			return
		}
	}

	if err := bootstrapWorkspaceAgentSession(agentBaseURL, serverOrigin); err != nil {
		log.Error("workspace agent bootstrap failed", logger.F("error", err.Error()))
		return
	}

	shopCount, err := warmWorkspaceAuthorizedShops(agentBaseURL)
	if err != nil {
		log.Error("workspace shops warmup failed", logger.F("error", err.Error()))
		return
	}

	log.Info("workspace agent ready",
		logger.F("agent_base_url", agentBaseURL),
		logger.F("server_origin", serverOrigin),
		logger.F("shop_count", shopCount),
	)
}

func (a *App) startWorkspaceAgentProcess(installRoot, runtimeDir, serverOrigin string) (*exec.Cmd, error) {
	nodeExe, err := resolveWorkspaceNodeExecutable(installRoot)
	if err != nil {
		return nil, err
	}

	agentEntry := filepath.Join(installRoot, "apps", "agent", "src", "server", "index.mjs")
	if _, statErr := os.Stat(agentEntry); statErr != nil {
		return nil, fmt.Errorf("workspace agent entry missing: %w", statErr)
	}

	cmd := exec.Command(nodeExe, "--enable-source-maps", agentEntry)
	hideWindow(cmd)
	cmd.Dir = installRoot
	cmd.Env = withWorkspaceAgentEnv(os.Environ(), runtimeDir, serverOrigin)

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func stopWorkspaceAgentProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return stopProcessCmdForShutdown(cmd)
}

func resolveWorkspaceInstallRoot(appRoot string) (string, bool) {
	current := strings.TrimSpace(appRoot)
	if current == "" {
		return "", false
	}

	for i := 0; i < 5; i++ {
		agentEntry := filepath.Join(current, "apps", "agent", "src", "server", "index.mjs")
		if _, err := os.Stat(agentEntry); err == nil {
			return current, true
		}
		next := filepath.Dir(current)
		if next == current {
			break
		}
		current = next
	}
	return "", false
}

func resolveWorkspaceRuntimeDir() string {
	for _, candidate := range []string{
		strings.TrimSpace(os.Getenv("DESKTOP_RUNTIME_DIR")),
		strings.TrimSpace(os.Getenv("AGENT_STATE_DIR")),
	} {
		if candidate != "" {
			return candidate
		}
	}

	if programData := strings.TrimSpace(os.Getenv("ProgramData")); programData != "" {
		return filepath.Join(programData, "1688shop-agent", "runtime")
	}

	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".1688shop-desktop-runtime")
	}

	return filepath.Join(os.TempDir(), "1688shop-desktop-runtime")
}

func resolveWorkspaceServerOrigin(runtimeDir string) string {
	configPath := filepath.Join(runtimeDir, "config", "server-connection.json")
	data, err := os.ReadFile(configPath)
	if err == nil {
		var config workspaceServerConnectionConfig
		if jsonErr := json.Unmarshal(data, &config); jsonErr == nil {
			if origin := strings.TrimSpace(config.ServerOrigin); origin != "" {
				return strings.TrimRight(origin, "/")
			}
		}
	}

	if value := strings.TrimSpace(os.Getenv("DESKTOP_SERVER_BASE_URL")); value != "" {
		return strings.TrimRight(value, "/")
	}

	return ""
}

func resolveWorkspaceLocalAgentBaseURL() string {
	for _, candidate := range []string{
		strings.TrimSpace(os.Getenv("ANT_BROWSER_WORKSPACE_AGENT_BASE_URL")),
		strings.TrimSpace(os.Getenv("AGENT_BASE_URL")),
		defaultWorkspaceAgentBaseURL,
	} {
		if candidate != "" {
			return strings.TrimRight(candidate, "/")
		}
	}
	return defaultWorkspaceAgentBaseURL
}

func resolveWorkspaceNodeExecutable(installRoot string) (string, error) {
	candidates := []string{
		filepath.Join(installRoot, "runtime", "node", "node.exe"),
		filepath.Join(installRoot, "runtime", "node", "win-x64", "node.exe"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	if path, err := exec.LookPath("node"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("node executable not found")
}

func withWorkspaceAgentEnv(base []string, runtimeDir, serverOrigin string) []string {
	envMap := make(map[string]string, len(base)+8)
	for _, entry := range base {
		parts := strings.SplitN(entry, "=", 2)
		key := parts[0]
		value := ""
		if len(parts) == 2 {
			value = parts[1]
		}
		envMap[key] = value
	}

	envMap["DESKTOP_RUNTIME_DIR"] = runtimeDir
	envMap["AGENT_STATE_DIR"] = runtimeDir
	envMap["DESKTOP_SERVER_BASE_URL"] = serverOrigin
	envMap["AGENT_LISTEN_HOST"] = defaultWorkspaceAgentListenHost
	envMap["AGENT_LISTEN_PORT"] = defaultWorkspaceAgentListenPort
	envMap["AGENT_BASE_URL"] = defaultWorkspaceAgentBaseURL
	envMap["ANT_RUNTIME_BASE_URL"] = defaultWorkspaceAntRuntimeBaseURL

	result := make([]string, 0, len(envMap))
	for key, value := range envMap {
		result = append(result, key+"="+value)
	}
	return result
}

func bootstrapWorkspaceAgentSession(agentBaseURL, serverOrigin string) error {
	loginPayload, err := workspaceServerLogin(serverOrigin)
	if err != nil {
		return err
	}

	userID := strings.TrimSpace(loginPayload.Data.UserSummary.ID)
	displayName := strings.TrimSpace(loginPayload.Data.UserSummary.DisplayName)
	if userID == "" {
		userID = strings.TrimSpace(loginPayload.Data.User.ID)
	}
	if displayName == "" {
		displayName = strings.TrimSpace(loginPayload.Data.User.DisplayName)
	}

	requestBody := map[string]any{
		"accessToken": loginPayload.Data.AccessToken,
		"user": map[string]string{
			"userId":      userID,
			"displayName": displayName,
		},
		"serverBaseUrl": serverOrigin,
	}

	var bootstrap workspaceBootstrapEnvelope
	if err := postWorkspaceJSON(agentBaseURL+"/local/session/bootstrap", requestBody, &bootstrap); err != nil {
		return err
	}
	return nil
}

func warmWorkspaceAuthorizedShops(agentBaseURL string) (int, error) {
	var shops workspaceShopsEnvelope
	if err := getWorkspaceJSON(agentBaseURL+"/local/shops", &shops); err != nil {
		return 0, err
	}
	return len(shops.Data.Items), nil
}

func workspaceServerLogin(serverOrigin string) (*workspaceLoginEnvelope, error) {
	body := map[string]string{
		"username": strings.TrimSpace(firstNonEmptyWorkspaceString(os.Getenv("ANT_WORKSPACE_BOOTSTRAP_USERNAME"), defaultWorkspaceBootstrapUsername)),
		"password": firstNonEmptyWorkspaceString(os.Getenv("ANT_WORKSPACE_BOOTSTRAP_PASSWORD"), defaultWorkspaceBootstrapPassword),
	}

	var login workspaceLoginEnvelope
	if err := postWorkspaceJSON(strings.TrimRight(serverOrigin, "/")+"/api/auth/login", body, &login); err != nil {
		return nil, err
	}
	return &login, nil
}

func firstNonEmptyWorkspaceString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func waitForHTTPReachable(target string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isHTTPReachable(target) {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func isHTTPReachable(target string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(target)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode >= 200 && response.StatusCode < 500
}

func postWorkspaceJSON(url string, body any, dest any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("content-type", "application/json")
	request.Header.Set("accept", "application/json")

	return doWorkspaceJSON(request, dest)
}

func getWorkspaceJSON(url string, dest any) error {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("accept", "application/json")
	return doWorkspaceJSON(request, dest)
}

func doWorkspaceJSON(request *http.Request, dest any) error {
	client := &http.Client{
		Timeout: 6 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 3 * time.Second,
			}).DialContext,
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("request failed: %s (%d): %s", request.URL.String(), response.StatusCode, strings.TrimSpace(string(body)))
	}

	if dest == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, dest)
}
