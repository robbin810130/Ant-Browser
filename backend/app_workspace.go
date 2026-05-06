package backend

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/workspace"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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

func (a *App) WorkspaceOpenShop(shopID string) (*workspace.OpenShopResult, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}

	shopID = strings.TrimSpace(shopID)
	if shopID == "" {
		return nil, fmt.Errorf("shop id is required")
	}

	shops, err := a.workspaceService.FetchAuthorizedShops(context.Background())
	if err != nil {
		return nil, err
	}

	shop, ok := findAuthorizedShopProjection(shops, shopID)
	if !ok {
		return nil, fmt.Errorf("shop not found: %s", shopID)
	}

	if shop.SharedLoginStatus != "ready" {
		return &workspace.OpenShopResult{
			ShopID:     shop.ShopID,
			ProfileID:  shop.ProfileID,
			InstanceID: shop.InstanceID,
			Code:       "ANT_BACKEND_LOGIN_REQUIRED",
			Message:    "当前共享会话未就绪，请先执行更新凭据后重试",
		}, nil
	}

	if err := a.ensureManagedShopProfile(shop); err != nil {
		return nil, err
	}

	targetURL := workspace.DefaultBackendURL(shop.ShopID)
	profile, err := a.BrowserInstanceStartWithParams(shop.ProfileID, nil, []string{targetURL}, true)
	if err != nil {
		return &workspace.OpenShopResult{
			ShopID:     shop.ShopID,
			ProfileID:  shop.ProfileID,
			InstanceID: shop.InstanceID,
			Code:       "ANT_INSTANCE_OPEN_FAILED",
			Message:    err.Error(),
		}, nil
	}

	result := a.waitForWorkspaceOpenResult(shop, 10*time.Second)
	result.ProfileID = shop.ProfileID
	if result.InstanceID == "" {
		result.InstanceID = profile.ProfileId
	}
	return result, nil
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

func findAuthorizedShopProjection(shops []workspace.ShopInstanceProjection, shopID string) (workspace.ShopInstanceProjection, bool) {
	for _, shop := range shops {
		if strings.TrimSpace(shop.ShopID) == shopID {
			return shop, true
		}
	}
	return workspace.ShopInstanceProjection{}, false
}

func (a *App) ensureManagedShopProfile(shop workspace.ShopInstanceProjection) error {
	profileID := strings.TrimSpace(shop.ProfileID)
	if profileID == "" {
		return fmt.Errorf("profile id is required")
	}

	_, err := a.UpsertManagedProfile(launchcode.ManagedProfileUpsertInput{
		ProfileID:    profileID,
		ShopID:       shop.ShopID,
		PlatformCode: shop.PlatformCode,
		ProfileName:  firstNonEmptyString(shop.ShopName, shop.ShopID),
		ManagedMode:  true,
		UserDataDir:  filepath.Join("managed-profiles", strings.ReplaceAll(profileID, ":", "__")),
	})
	return err
}

func (a *App) waitForWorkspaceOpenResult(shop workspace.ShopInstanceProjection, timeout time.Duration) *workspace.OpenShopResult {
	deadline := time.Now().Add(timeout)
	lastSnapshot := workspace.OpenRuntimeSnapshot{}

	for time.Now().Before(deadline) {
		snapshot, err := a.browserRuntimeSnapshot(shop.ProfileID)
		if err == nil {
			lastSnapshot = snapshot
			result := workspace.ClassifyOpenResult(snapshot)
			result.ShopID = shop.ShopID
			result.ProfileID = shop.ProfileID
			result.InstanceID = firstNonEmptyString(shop.InstanceID, shop.ProfileID)
			result.CurrentURL = snapshot.CurrentURL
			result.PageTitle = snapshot.PageTitle
			if result.Success || result.Code == "ANT_BACKEND_LOGIN_REQUIRED" {
				return &result
			}
		}
		time.Sleep(350 * time.Millisecond)
	}

	result := workspace.ClassifyOpenResult(lastSnapshot)
	result.ShopID = shop.ShopID
	result.ProfileID = shop.ProfileID
	result.InstanceID = firstNonEmptyString(shop.InstanceID, shop.ProfileID)
	result.CurrentURL = lastSnapshot.CurrentURL
	result.PageTitle = lastSnapshot.PageTitle
	if result.Code == "" && !result.Success {
		result.Code = "ANT_INSTANCE_OPEN_FAILED"
		result.Message = "未能打开目标店铺后台，请稍后重试"
	}
	return &result
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
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
