package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/managedinstance"
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

var workspaceFocusManagedShop = func(service *managedinstance.Service, req managedinstance.OpenRequest) (*managedinstance.OpenResult, error) {
	if service == nil {
		return nil, fmt.Errorf("managed instance service is not configured")
	}
	return service.FocusManagedShop(req)
}

func (a *App) WorkspaceSummary() (*workspace.WorkspaceSummary, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	a.ensureWorkspaceAgentReachableForRequest("workspace summary")
	return a.workspaceService.FetchSummary(context.Background())
}

func (a *App) WorkspaceAuthorizedShops() ([]workspace.ShopInstanceProjection, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	a.ensureWorkspaceAgentReachableForRequest("authorized shops")
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

	a.ensureWorkspaceAgentReachableForRequest("open shop")
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
		log.Error("店铺后台打开失败，准备回传结果",
			logger.F("shop_id", result.ShopID),
			logger.F("profile_id", profileID),
			logger.F("open_request_id", openContext.OpenRequestID),
			logger.F("error", err.Error()),
		)
		reportErr := a.reportWorkspaceOpenResult(context.Background(), openContext.OpenRequestID, result, runtimeInfo)
		if reportErr != nil {
			log.Error("店铺后台打开失败结果回传失败",
				logger.F("shop_id", result.ShopID),
				logger.F("profile_id", profileID),
				logger.F("open_request_id", openContext.OpenRequestID),
				logger.F("error", reportErr.Error()),
			)
		}
		return result, reportErr
	}
	if managedResult == nil {
		result.Code = "ANT_INSTANCE_OPEN_FAILED"
		result.Message = "managed open returned nil result"
		log.Error("店铺后台打开返回空结果，准备回传失败",
			logger.F("shop_id", result.ShopID),
			logger.F("profile_id", profileID),
			logger.F("open_request_id", openContext.OpenRequestID),
		)
		reportErr := a.reportWorkspaceOpenResult(context.Background(), openContext.OpenRequestID, result, runtimeInfo)
		if reportErr != nil {
			log.Error("店铺后台空结果回传失败",
				logger.F("shop_id", result.ShopID),
				logger.F("profile_id", profileID),
				logger.F("open_request_id", openContext.OpenRequestID),
				logger.F("error", reportErr.Error()),
			)
		}
		return result, reportErr
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
	log.Info("店铺后台打开完成，准备回传结果",
		logger.F("shop_id", result.ShopID),
		logger.F("profile_id", profileID),
		logger.F("open_request_id", openContext.OpenRequestID),
		logger.F("success", result.Success),
		logger.F("code", result.Code),
		logger.F("current_url", result.CurrentURL),
		logger.F("debug_port", runtimeInfo.DebugPort),
	)
	if err := a.reportWorkspaceOpenResult(context.Background(), openContext.OpenRequestID, result, runtimeInfo); err != nil {
		log.Error("店铺后台打开结果回传失败",
			logger.F("shop_id", result.ShopID),
			logger.F("profile_id", profileID),
			logger.F("open_request_id", openContext.OpenRequestID),
			logger.F("success", result.Success),
			logger.F("error", err.Error()),
		)
		return result, err
	}
	log.Info("店铺后台打开结果已回传",
		logger.F("shop_id", result.ShopID),
		logger.F("profile_id", profileID),
		logger.F("open_request_id", openContext.OpenRequestID),
		logger.F("success", result.Success),
	)
	return result, nil
}

func (a *App) WorkspaceFocusShop(shopID string) (*workspace.OpenShopResult, error) {
	if a == nil || a.workspaceService == nil {
		return nil, fmt.Errorf("workspace service is not configured")
	}
	log := logger.New("WorkspaceFocus")

	shopID = strings.TrimSpace(shopID)
	if shopID == "" {
		return nil, fmt.Errorf("shop id is required")
	}

	a.ensureWorkspaceAgentReachableForRequest("focus shop")
	shops, err := a.workspaceService.FetchAuthorizedShops(context.Background())
	if err != nil {
		return nil, err
	}

	projectedShop, ok := findAuthorizedShopProjection(shops, shopID)
	if !ok {
		return nil, fmt.Errorf("shop not found: %s", shopID)
	}

	profileID := strings.TrimSpace(projectedShop.ProfileID)
	if profileID == "" {
		profileID = strings.TrimSpace(projectedShop.PlatformCode) + ":" + shopID
	}
	result := &workspace.OpenShopResult{
		ShopID:    shopID,
		ProfileID: profileID,
	}
	if a.managedInstanceService == nil {
		result.Code = "ANT_INSTANCE_FOCUS_FAILED"
		result.Message = "managed instance service is not configured"
		return result, nil
	}

	log.Info("准备调起店铺后台窗口",
		logger.F("shop_id", shopID),
		logger.F("profile_id", profileID),
	)
	managedResult, err := workspaceFocusManagedShop(a.managedInstanceService, managedinstance.OpenRequest{
		ShopID:      shopID,
		ProfileID:   profileID,
		TargetURL:   workspace.DefaultBackendURL(shopID),
		ManagedMode: true,
	})
	if err != nil {
		result.Code = "ANT_INSTANCE_FOCUS_FAILED"
		result.Message = err.Error()
		return result, nil
	}
	if managedResult == nil {
		result.Code = "ANT_INSTANCE_FOCUS_FAILED"
		result.Message = "managed focus returned nil result"
		return result, nil
	}

	result.Success = managedResult.Success
	result.Code = managedResult.Code
	result.Message = managedResult.Message
	result.CurrentURL = managedResult.CurrentURL
	result.PageTitle = managedResult.PageTitle
	log.Info("店铺后台窗口调起完成",
		logger.F("shop_id", shopID),
		logger.F("profile_id", profileID),
		logger.F("success", result.Success),
		logger.F("code", result.Code),
		logger.F("current_url", result.CurrentURL),
	)
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
