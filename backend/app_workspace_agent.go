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

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
)

const (
	defaultWorkspaceAgentListenHost   = "127.0.0.1"
	defaultWorkspaceAgentListenPort   = "47831"
	defaultWorkspaceAntRuntimeBaseURL = "http://127.0.0.1:19877"
	defaultWorkspaceServerOrigin      = "http://127.0.0.1:4174"
	workspaceBootstrapTimeout         = 8 * time.Second
)

type workspaceServerConnectionConfig struct {
	ServerOrigin string `json:"serverOrigin"`
	ApiBaseURL   string `json:"apiBaseUrl"`
	ServerIP     string `json:"serverIp"`
	ServerPort   int    `json:"serverPort"`
}

type workspaceServerOriginResolution struct {
	Origin     string
	Source     string
	ConfigPath string
}

type workspaceAuthIdentity struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type workspaceMeEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID          string                `json:"id"`
		DisplayName string                `json:"displayName"`
		UserSummary workspaceAuthIdentity `json:"userSummary"`
		User        workspaceAuthIdentity `json:"user"`
	} `json:"data"`
}

type workspaceBootstrapUser struct {
	UserID      string
	DisplayName string
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

	installRoot, err := resolveWorkspaceInstallRootWithConfig(a.appRoot, a.config)
	if err != nil {
		appendWorkspaceHostLog(resolveWorkspaceRuntimeDir(), "workspace install root unavailable: app_root=%s error=%v", a.appRoot, err)
		log.Warn("workspace install root unavailable", logger.F("app_root", a.appRoot), logger.F("error", err.Error()))
		return
	}

	runtimeDir := resolveWorkspaceRuntimeDirWithConfig(a.config)
	serverOrigin := resolveWorkspaceServerOriginWithConfig(runtimeDir, a.config)
	appendWorkspaceHostLog(runtimeDir, "bootstrap begin: app_root=%s install_root=%s runtime_dir=%s server_origin=%s", a.appRoot, installRoot, runtimeDir, serverOrigin)
	if strings.TrimSpace(serverOrigin) == "" {
		appendWorkspaceHostLog(runtimeDir, "workspace server origin unavailable")
		log.Warn("workspace server origin unavailable", logger.F("runtime_dir", runtimeDir))
		return
	}

	agentBaseURL := a.resolveWorkspaceAgentBaseURL()
	appendWorkspaceHostLog(runtimeDir, "resolved agent base url: %s", agentBaseURL)
	if !isHTTPReachable(agentBaseURL + "/health") {
		cmd, resolvedAgentBaseURL, err := a.startWorkspaceAgentProcess(installRoot, runtimeDir, serverOrigin)
		if err != nil {
			appendWorkspaceHostLog(runtimeDir, "start workspace agent failed: %v", err)
			log.Error("start workspace agent failed", logger.F("error", err.Error()))
			return
		}
		agentBaseURL = resolvedAgentBaseURL
		a.workspaceAgentURL = resolvedAgentBaseURL
		if a.workspaceService != nil {
			a.workspaceService.SetBaseURL(resolvedAgentBaseURL)
		}
		a.workspaceAgentCmd = cmd
		if !waitForHTTPReachable(agentBaseURL+"/health", workspaceBootstrapTimeout) {
			appendWorkspaceHostLog(runtimeDir, "workspace agent health timeout: url=%s", agentBaseURL+"/health")
			log.Error("workspace agent health timeout", logger.F("url", agentBaseURL+"/health"))
			return
		}
	}

	authSession, err := a.LoadDesktopAuthSession()
	if err != nil {
		appendWorkspaceHostLog(runtimeDir, "load desktop auth session failed: %v", err)
		log.Error("load desktop auth session failed", logger.F("error", err.Error()))
		return
	}
	accessToken := ""
	if authSession != nil {
		accessToken = strings.TrimSpace(authSession.AccessToken)
	}
	if accessToken == "" {
		appendWorkspaceHostLog(runtimeDir, "workspace agent bootstrap skipped: persisted desktop access token unavailable")
		log.Warn("workspace agent bootstrap skipped", logger.F("reason", "persisted desktop access token unavailable"))
		return
	}

	bootstrapUser, err := fetchWorkspaceBootstrapUser(serverOrigin, accessToken)
	if err != nil {
		appendWorkspaceHostLog(runtimeDir, "fetch workspace bootstrap user failed: %v", err)
		log.Error("fetch workspace bootstrap user failed", logger.F("error", err.Error()))
		return
	}

	if err := bootstrapWorkspaceAgentSession(agentBaseURL, serverOrigin, accessToken, bootstrapUser); err != nil {
		appendWorkspaceHostLog(runtimeDir, "workspace agent bootstrap failed: %v", err)
		log.Error("workspace agent bootstrap failed", logger.F("error", err.Error()))
		return
	}

	shopCount, err := warmWorkspaceAuthorizedShops(agentBaseURL)
	if err != nil {
		appendWorkspaceHostLog(runtimeDir, "workspace shops warmup failed: %v", err)
		log.Error("workspace shops warmup failed", logger.F("error", err.Error()))
		return
	}

	appendWorkspaceHostLog(runtimeDir, "workspace agent ready: agent_base_url=%s server_origin=%s shop_count=%d", agentBaseURL, serverOrigin, shopCount)
	log.Info("workspace agent ready",
		logger.F("agent_base_url", agentBaseURL),
		logger.F("server_origin", serverOrigin),
		logger.F("shop_count", shopCount),
	)
}

func (a *App) startWorkspaceAgentProcess(installRoot, runtimeDir, serverOrigin string) (*exec.Cmd, string, error) {
	nodeExe, err := resolveWorkspaceNodeExecutable(installRoot)
	if err != nil {
		appendWorkspaceHostLog(runtimeDir, "resolve node executable failed: %v", err)
		return nil, "", err
	}

	agentEntry := filepath.Join(installRoot, "apps", "agent", "src", "server", "index.mjs")
	if _, statErr := os.Stat(agentEntry); statErr != nil {
		appendWorkspaceHostLog(runtimeDir, "workspace agent entry missing: %v", statErr)
		return nil, "", fmt.Errorf("workspace agent entry missing: %w", statErr)
	}

	listenPort, agentBaseURL, err := reserveWorkspaceAgentPort()
	if err != nil {
		appendWorkspaceHostLog(runtimeDir, "reserve workspace agent port failed: %v", err)
		return nil, "", err
	}

	cleanedCount, cleanupErr := cleanupWorkspaceBootstrapProcesses(installRoot)
	if cleanupErr != nil {
		appendWorkspaceHostLog(runtimeDir, "workspace bootstrap cleanup failed: %v", cleanupErr)
		return nil, "", cleanupErr
	}
	if cleanedCount > 0 {
		appendWorkspaceHostLog(runtimeDir, "workspace bootstrap cleanup removed stale processes: count=%d", cleanedCount)
	}

	logFile, err := openWorkspaceHostLogFile(runtimeDir)
	if err != nil {
		return nil, "", err
	}
	if a.workspaceAgentLog != nil && a.workspaceAgentLog != logFile {
		_ = a.workspaceAgentLog.Close()
	}
	a.workspaceAgentLog = logFile

	cmd := exec.Command(nodeExe, "--enable-source-maps", agentEntry)
	hideWindow(cmd)
	cmd.Dir = installRoot
	cmd.Env = withWorkspaceAgentEnv(os.Environ(), runtimeDir, serverOrigin, listenPort, agentBaseURL)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	appendWorkspaceHostLog(runtimeDir,
		"starting workspace agent: node=%s entry=%s cwd=%s listen=%s:%s server_origin=%s",
		nodeExe,
		agentEntry,
		cmd.Dir,
		defaultWorkspaceAgentListenHost,
		listenPort,
		serverOrigin,
	)

	if err := cmd.Start(); err != nil {
		appendWorkspaceHostLog(runtimeDir, "workspace agent process start error: %v", err)
		return nil, "", err
	}
	appendWorkspaceHostLog(runtimeDir, "workspace agent process started: pid=%d", cmd.Process.Pid)
	return cmd, agentBaseURL, nil
}

func stopWorkspaceAgentProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return stopProcessCmdForShutdown(cmd)
}

func (a *App) resetWorkspaceAgentSessionRuntimeHook(reason string) error {
	if a == nil {
		return nil
	}

	runtimeDir := resolveWorkspaceRuntimeDirWithConfig(a.config)
	if trimmedReason := strings.TrimSpace(reason); trimmedReason != "" {
		appendWorkspaceHostLog(runtimeDir, "workspace agent session/runtime hook reset: reason=%s", trimmedReason)
	}

	var resetErr error
	if err := stopWorkspaceAgentProcess(a.workspaceAgentCmd); err != nil {
		resetErr = err
		appendWorkspaceHostLog(runtimeDir, "workspace agent session/runtime hook stop failed: %v", err)
	}
	a.workspaceAgentCmd = nil

	if a.workspaceAgentLog != nil {
		_ = a.workspaceAgentLog.Close()
		a.workspaceAgentLog = nil
	}

	a.workspaceAgentURL = ""
	if a.workspaceService != nil {
		a.workspaceService.SetBaseURL(resolveWorkspaceAgentBaseURLWithConfig(a.config))
	}

	return resetErr
}

func resolveWorkspaceInstallRoot(appRoot string) (string, error) {
	return resolveWorkspaceInstallRootWithConfig(appRoot, nil)
}

func resolveWorkspaceInstallRootWithConfig(appRoot string, cfg *config.Config) (string, error) {
	if root, configured, err := resolveWorkspaceInstallRootFromEnv(); configured {
		if err != nil {
			return "", err
		}
		return root, nil
	}
	if root, configured, err := resolveWorkspaceInstallRootFromConfig(cfg); configured {
		if err != nil {
			return "", err
		}
		return root, nil
	}

	current := strings.TrimSpace(appRoot)
	if current == "" {
		return "", fmt.Errorf("app root is empty")
	}

	for i := 0; i < 5; i++ {
		if workspaceInstallRootLooksValid(current) {
			return current, nil
		}
		next := filepath.Dir(current)
		if next == current {
			break
		}
		current = next
	}
	return "", fmt.Errorf("workspace agent entry not found from app root: %s", strings.TrimSpace(appRoot))
}

func resolveWorkspaceInstallRootFromConfig(cfg *config.Config) (string, bool, error) {
	if cfg == nil {
		return "", false, nil
	}
	root := strings.TrimSpace(cfg.Workspace.InstallRoot)
	if root == "" {
		return "", false, nil
	}
	root = strings.TrimRight(root, string(os.PathSeparator))
	if workspaceInstallRootLooksValid(root) {
		return root, true, nil
	}
	return "", true, fmt.Errorf("workspace install root configured by config.workspace.install_root is invalid: %s", cfg.Workspace.InstallRoot)
}

func resolveWorkspaceInstallRootFromEnv() (string, bool, error) {
	for _, key := range []string{"ANT_BROWSER_WORKSPACE_INSTALL_ROOT", "WORKSPACE_INSTALL_ROOT"} {
		rawValue := strings.TrimSpace(os.Getenv(key))
		if rawValue == "" {
			continue
		}
		root := strings.TrimRight(rawValue, string(os.PathSeparator))
		if workspaceInstallRootLooksValid(root) {
			return root, true, nil
		}
		return "", true, fmt.Errorf("workspace install root configured by %s is invalid: %s", key, rawValue)
	}
	return "", false, nil
}

func workspaceInstallRootLooksValid(root string) bool {
	if strings.TrimSpace(root) == "" {
		return false
	}
	agentEntry := filepath.Join(root, "apps", "agent", "src", "server", "index.mjs")
	_, err := os.Stat(agentEntry)
	return err == nil
}

func resolveWorkspaceRuntimeDir() string {
	return resolveWorkspaceRuntimeDirWithConfig(nil)
}

func resolveWorkspaceRuntimeDirWithConfig(cfg *config.Config) string {
	for _, candidate := range []string{
		configuredWorkspaceRuntimeDir(cfg),
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
	return resolveWorkspaceServerOriginWithConfig(runtimeDir, nil)
}

func resolveWorkspaceServerOriginWithConfig(runtimeDir string, cfg *config.Config) string {
	return resolveWorkspaceServerOriginDetails(runtimeDir, cfg).Origin
}

func resolveWorkspaceServerOriginDetails(runtimeDir string, cfg *config.Config) workspaceServerOriginResolution {
	configPath := filepath.Join(runtimeDir, "config", "server-connection.json")
	data, err := os.ReadFile(configPath)
	if err == nil {
		var config workspaceServerConnectionConfig
		if jsonErr := json.Unmarshal(data, &config); jsonErr == nil {
			if origin := strings.TrimSpace(config.ServerOrigin); origin != "" {
				return workspaceServerOriginResolution{
					Origin:     strings.TrimRight(origin, "/"),
					Source:     "runtime-config",
					ConfigPath: configPath,
				}
			}
		}
	}

	if value := strings.TrimSpace(os.Getenv("DESKTOP_SERVER_BASE_URL")); value != "" {
		return workspaceServerOriginResolution{
			Origin: strings.TrimRight(value, "/"),
			Source: "env:DESKTOP_SERVER_BASE_URL",
		}
	}
	if cfg != nil {
		if value := strings.TrimSpace(cfg.Workspace.ServerOrigin); value != "" {
			return workspaceServerOriginResolution{
				Origin: strings.TrimRight(value, "/"),
				Source: "config.yaml",
			}
		}
	}

	return workspaceServerOriginResolution{
		Origin: defaultWorkspaceServerOrigin,
		Source: "default",
	}
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

func configuredWorkspaceRuntimeDir(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Workspace.RuntimeDir)
}

func reserveWorkspaceAgentPort() (string, string, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(defaultWorkspaceAgentListenHost, "0"))
	if err != nil {
		return "", "", fmt.Errorf("reserve workspace agent port: %w", err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 {
		return "", "", fmt.Errorf("workspace agent reserved port unavailable")
	}
	port := fmt.Sprintf("%d", addr.Port)
	baseURL := fmt.Sprintf("http://%s:%s", defaultWorkspaceAgentListenHost, port)
	return port, baseURL, nil
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

func openWorkspaceHostLogFile(runtimeDir string) (*os.File, error) {
	logPath := resolveWorkspaceHostLogPath(runtimeDir)
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, fmt.Errorf("create workspace host log dir failed: %w", err)
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open workspace host log failed: %w", err)
	}
	return file, nil
}

func resolveWorkspaceHostLogPath(runtimeDir string) string {
	return filepath.Join(runtimeDir, "logs", "host-workspace-agent.log")
}

func appendWorkspaceHostLog(runtimeDir, format string, args ...interface{}) {
	if strings.TrimSpace(runtimeDir) == "" {
		runtimeDir = resolveWorkspaceRuntimeDir()
	}
	file, err := openWorkspaceHostLogFile(runtimeDir)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "[%s] %s\n", time.Now().Format(time.RFC3339Nano), fmt.Sprintf(format, args...))
}

func withWorkspaceAgentEnv(base []string, runtimeDir, serverOrigin, listenPort, agentBaseURL string) []string {
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
	envMap["AGENT_LISTEN_PORT"] = listenPort
	envMap["AGENT_BASE_URL"] = agentBaseURL
	envMap["ANT_RUNTIME_BASE_URL"] = defaultWorkspaceAntRuntimeBaseURL

	result := make([]string, 0, len(envMap))
	for key, value := range envMap {
		result = append(result, key+"="+value)
	}
	return result
}

func fetchWorkspaceBootstrapUser(serverOrigin, accessToken string) (workspaceBootstrapUser, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return workspaceBootstrapUser{}, fmt.Errorf("workspace bootstrap access token is required")
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, strings.TrimRight(serverOrigin, "/")+"/api/auth/me", nil)
	if err != nil {
		return workspaceBootstrapUser{}, err
	}
	request.Header.Set("accept", "application/json")
	request.Header.Set("authorization", "Bearer "+accessToken)

	var me workspaceMeEnvelope
	if err := doWorkspaceJSON(request, &me); err != nil {
		return workspaceBootstrapUser{}, err
	}

	user := workspaceBootstrapUser{
		UserID: strings.TrimSpace(firstNonEmptyWorkspaceString(
			me.Data.UserSummary.ID,
			me.Data.User.ID,
			me.Data.ID,
		)),
		DisplayName: strings.TrimSpace(firstNonEmptyWorkspaceString(
			me.Data.UserSummary.DisplayName,
			me.Data.User.DisplayName,
			me.Data.DisplayName,
		)),
	}
	if user.UserID == "" {
		return workspaceBootstrapUser{}, fmt.Errorf("workspace bootstrap user id is required")
	}
	return user, nil
}

func bootstrapWorkspaceAgentSession(agentBaseURL, serverOrigin, accessToken string, user workspaceBootstrapUser) error {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return fmt.Errorf("workspace bootstrap access token is required")
	}

	userID := strings.TrimSpace(user.UserID)
	if userID == "" {
		return fmt.Errorf("workspace bootstrap user id is required")
	}
	displayName := strings.TrimSpace(user.DisplayName)

	requestBody := map[string]any{
		"accessToken": accessToken,
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
