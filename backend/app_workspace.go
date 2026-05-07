package backend

import (
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/workspace"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultWorkspaceAgentBaseURL = "http://127.0.0.1:47831"

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

	projectedShop, ok := findAuthorizedShopProjection(shops, shopID)
	if !ok {
		return nil, fmt.Errorf("shop not found: %s", shopID)
	}

	if projectedShop.SharedLoginStatus != "ready" {
		return buildUnavailableShopOpenResult(projectedShop), nil
	}

	openContext, err := a.workspaceService.FetchOpenShopContext(context.Background(), shopID)
	if err != nil {
		return nil, err
	}

	shop := openContext.Shop
	profileID := firstNonEmptyString(openContext.Profile.ProfileID, shop.PlatformCode+":"+shopID)
	result := &workspace.OpenShopResult{
		ShopID:     firstNonEmptyString(shop.ShopID, shopID),
		ProfileID:  profileID,
		InstanceID: "",
	}

	if err := a.ensureManagedShopProfile(profileID, shop); err != nil {
		result.Code = "ANT_PROFILE_UPSERT_FAILED"
		result.Message = err.Error()
		if reportErr := a.reportWorkspaceOpenResult(context.Background(), openContext.OpenRequestID, result, nil); reportErr != nil {
			return nil, reportErr
		}
		return result, nil
	}

	if err := a.InjectManagedSessionBundle(profileID, openContext.LaunchContext.SessionBundle); err != nil {
		result.Code = "ANT_SESSION_RESTORE_FAILED"
		result.Message = err.Error()
		if reportErr := a.reportWorkspaceOpenResult(context.Background(), openContext.OpenRequestID, result, nil); reportErr != nil {
			return nil, reportErr
		}
		return result, nil
	}

	targetURL := firstNonEmptyString(openContext.LaunchContext.TargetURL, workspace.DefaultBackendURL(result.ShopID))
	profile, err := a.BrowserInstanceStartWithParams(profileID, nil, []string{targetURL}, true)
	if err != nil {
		result.Code = "ANT_INSTANCE_OPEN_FAILED"
		result.Message = err.Error()
		if reportErr := a.reportWorkspaceOpenResult(context.Background(), openContext.OpenRequestID, result, nil); reportErr != nil {
			return nil, reportErr
		}
		return result, nil
	}

	runtimeInfo := &workspace.OpenReportRuntime{
		PID:       profile.Pid,
		DebugPort: profile.DebugPort,
	}
	result = a.waitForWorkspaceOpenResult(result.ShopID, profileID, "", openContext.LaunchContext, 10*time.Second)
	runtimeInfo.CurrentURL = result.CurrentURL
	runtimeInfo.PageTitle = result.PageTitle
	if reportErr := a.reportWorkspaceOpenResult(context.Background(), openContext.OpenRequestID, result, runtimeInfo); reportErr != nil {
		return nil, reportErr
	}
	return result, nil
}

func (a *App) initWorkspaceService() {
	client := workspace.NewWorkspaceClient(resolveWorkspaceAgentBaseURL(), nil)
	a.workspaceService = workspace.NewService(client, a.browserMgr)
}

func resolveWorkspaceAgentBaseURL() string {
	for _, value := range []string{
		os.Getenv("ANT_BROWSER_WORKSPACE_AGENT_BASE_URL"),
		os.Getenv("AGENT_BASE_URL"),
	} {
		if trimmed := strings.TrimRight(strings.TrimSpace(value), "/"); trimmed != "" {
			return trimmed
		}
	}
	return defaultWorkspaceAgentBaseURL
}

func findAuthorizedShopProjection(shops []workspace.ShopInstanceProjection, shopID string) (workspace.ShopInstanceProjection, bool) {
	for _, shop := range shops {
		if strings.TrimSpace(shop.ShopID) == shopID {
			return shop, true
		}
	}
	return workspace.ShopInstanceProjection{}, false
}

func (a *App) ensureManagedShopProfile(profileID string, shop workspace.ShopDescriptor) error {
	profileID = strings.TrimSpace(profileID)
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

func (a *App) waitForWorkspaceOpenResult(shopID string, profileID string, instanceID string, launchContext workspace.ShopLaunchContext, timeout time.Duration) *workspace.OpenShopResult {
	deadline := time.Now().Add(timeout)
	lastSnapshot := workspace.OpenRuntimeSnapshot{}

	for time.Now().Before(deadline) {
		snapshots, err := a.browserRuntimeSnapshots(profileID)
		if err == nil {
			snapshot := workspace.SelectPreferredOpenSnapshotForLaunchContext(shopID, launchContext, snapshots)
			lastSnapshot = snapshot
			result := workspace.ClassifyOpenResultForLaunchContext(shopID, launchContext, snapshot)
			result.ShopID = shopID
			result.ProfileID = profileID
			result.InstanceID = strings.TrimSpace(instanceID)
			result.CurrentURL = snapshot.CurrentURL
			result.PageTitle = snapshot.PageTitle
			if result.Success || result.Code == "ANT_BACKEND_LOGIN_REQUIRED" || result.Code == "ANT_BACKEND_TARGET_MISMATCH" || result.Code == "ANT_MANUAL_VERIFICATION_REQUIRED" {
				return &result
			}
		}
		time.Sleep(350 * time.Millisecond)
	}

	result := workspace.ClassifyOpenResultForLaunchContext(shopID, launchContext, lastSnapshot)
	result.ShopID = shopID
	result.ProfileID = profileID
	result.InstanceID = strings.TrimSpace(instanceID)
	result.CurrentURL = lastSnapshot.CurrentURL
	result.PageTitle = lastSnapshot.PageTitle
	if result.Code == "" && !result.Success {
		result.Code = "ANT_INSTANCE_OPEN_FAILED"
		result.Message = "未能打开目标店铺后台，请稍后重试"
	}
	return &result
}

func (a *App) reportWorkspaceOpenResult(ctx context.Context, openRequestID string, result *workspace.OpenShopResult, runtime *workspace.OpenReportRuntime) error {
	if a == nil || a.workspaceService == nil || result == nil || strings.TrimSpace(openRequestID) == "" {
		return nil
	}

	payload := workspace.OpenReportRequest{
		Status:  "failed",
		Runtime: runtime,
	}
	if result.Success {
		payload.Status = "succeeded"
	} else {
		payload.FailureCode = result.Code
		payload.FailureMessage = result.Message
	}

	if err := a.workspaceService.ReportOpenShopResult(ctx, openRequestID, payload); err != nil {
		if result.Success {
			return fmt.Errorf("native shop window opened but open result report failed: %w", err)
		}
		return fmt.Errorf("open failed and report failed: %w", err)
	}
	return nil
}

func buildUnavailableShopOpenResult(shop workspace.ShopInstanceProjection) *workspace.OpenShopResult {
	result := &workspace.OpenShopResult{
		ShopID:     shop.ShopID,
		ProfileID:  shop.ProfileID,
		InstanceID: shop.InstanceID,
	}

	switch strings.TrimSpace(shop.SharedLoginStatus) {
	case "awaiting_verification":
		result.Code = "ANT_MANUAL_VERIFICATION_REQUIRED"
		result.Message = "当前共享会话等待人工验证，请先完成验证后重试"
	case "validation_failed":
		result.Code = "ANT_SESSION_RESTORE_FAILED"
		result.Message = "当前共享会话验证失败，请先执行本机验证或更新凭据"
	default:
		result.Code = "ANT_BACKEND_LOGIN_REQUIRED"
		result.Message = "当前共享会话未就绪，请先执行更新凭据后重试"
	}

	return result
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
