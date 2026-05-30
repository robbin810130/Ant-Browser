package backend

import (
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/managedinstance"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/workspace"
	"context"
	"fmt"
	"os"
	"strings"
)

const defaultWorkspaceAgentBaseURL = "http://127.0.0.1:47831"

var workspaceOpenManagedShop = func(service *managedinstance.Service, req managedinstance.OpenRequest) (*managedinstance.OpenResult, error) {
	if service == nil {
		return nil, fmt.Errorf("managed instance service is not configured")
	}
	return service.OpenManagedShop(req)
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
	a.recoverRunningProfilesFromUserDataDirs()
	return a.workspaceService.FetchAuthorizedShops(context.Background())
}

func (a *App) WorkspaceOpenShop(shopID string) (*workspace.OpenShopResult, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	log := logger.New("WorkspaceOpen")

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
	var runtimeInfo *workspace.OpenReportRuntime

	log.Info("准备打开店铺后台",
		logger.F("shop_id", result.ShopID),
		logger.F("profile_id", profileID),
		logger.F("open_request_id", openContext.OpenRequestID),
		logger.F("launch_target_url", strings.TrimSpace(openContext.LaunchContext.TargetURL)),
		logger.F("success_url_patterns", strings.Join(openContext.LaunchContext.SuccessURLPatterns, ",")),
		logger.F("login_url_patterns", strings.Join(openContext.LaunchContext.LoginURLPatterns, ",")),
	)
	if a.managedInstanceService == nil {
		result.Code = "ANT_INSTANCE_OPEN_FAILED"
		result.Message = "managed instance service is not configured"
		return result, nil
	}

	managedResult, err := workspaceOpenManagedShop(a.managedInstanceService, managedinstance.OpenRequest{
		ShopID:        result.ShopID,
		ProfileID:     profileID,
		TargetURL:     openContext.LaunchContext.TargetURL,
		LaunchContext: openContext.LaunchContext,
		SessionBundle: openContext.LaunchContext.SessionBundle,
		ManagedMode:   true,
		SessionReady:  projectedShop.SharedLoginStatus == "ready",
		PreferVisible: true,
	})
	if err != nil {
		result.Code = "ANT_INSTANCE_OPEN_FAILED"
		result.Message = err.Error()
		return result, a.reportWorkspaceOpenResult(context.Background(), openContext.OpenRequestID, result, runtimeInfo)
	}

	result.Success = managedResult.Success
	result.Code = managedResult.Code
	result.Message = managedResult.Message
	result.CurrentURL = managedResult.CurrentURL
	result.PageTitle = managedResult.PageTitle
	runtimeInfo = &workspace.OpenReportRuntime{
		PID:        managedResult.PID,
		DebugPort:  managedResult.DebugPort,
		CurrentURL: managedResult.CurrentURL,
		PageTitle:  managedResult.PageTitle,
	}
	if err := a.reportWorkspaceOpenResult(context.Background(), openContext.OpenRequestID, result, runtimeInfo); err != nil {
		return result, err
	}
	return result, nil
}

func (a *App) initWorkspaceService() {
	client := workspace.NewWorkspaceClient(a.resolveWorkspaceAgentBaseURL(), nil)
	a.workspaceService = workspace.NewService(client, a.browserMgr, a.managedInstanceService)
	a.configureManagedInstanceRuntime()
}

func resolveWorkspaceAgentBaseURL() string {
	return resolveWorkspaceAgentBaseURLWithConfig(nil)
}

func resolveWorkspaceAgentBaseURLWithConfig(cfg *config.Config) string {
	for _, value := range []string{
		os.Getenv("ANT_BROWSER_WORKSPACE_AGENT_BASE_URL"),
		os.Getenv("AGENT_BASE_URL"),
	} {
		if trimmed := strings.TrimRight(strings.TrimSpace(value), "/"); trimmed != "" {
			return trimmed
		}
	}
	if cfg != nil {
		if trimmed := strings.TrimRight(strings.TrimSpace(cfg.Workspace.AgentBaseURL), "/"); trimmed != "" {
			return trimmed
		}
	}
	return defaultWorkspaceAgentBaseURL
}

func (a *App) resolveWorkspaceAgentBaseURL() string {
	if a != nil {
		if value := strings.TrimRight(strings.TrimSpace(a.workspaceAgentURL), "/"); value != "" {
			return value
		}
	}
	if a != nil {
		return resolveWorkspaceAgentBaseURLWithConfig(a.config)
	}
	return resolveWorkspaceAgentBaseURL()
}

func findAuthorizedShopProjection(shops []workspace.ShopInstanceProjection, shopID string) (workspace.ShopInstanceProjection, bool) {
	for _, shop := range shops {
		if strings.TrimSpace(shop.ShopID) == shopID {
			return shop, true
		}
	}
	return workspace.ShopInstanceProjection{}, false
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
