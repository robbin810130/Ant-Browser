package backend

import (
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/managedinstance"
	"ant-chrome/backend/internal/workspace"
	"context"
	"fmt"
	"os"
	"strings"
)

const defaultWorkspaceAgentBaseURL = "http://127.0.0.1:47831"

type workspaceOpenRun struct {
	done    chan struct{}
	result  *workspace.OpenShopResult
	runtime *workspace.OpenReportRuntime
	err     error
}

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
	var finalErr error

	run := a.beginWorkspaceOpenRun(profileID)
	if run != nil {
		defer func() {
			if result == nil && finalErr == nil {
				finalErr = fmt.Errorf("workspace open finished without result")
			}
			if finalErr == nil {
				if reportErr := a.reportWorkspaceOpenResult(context.Background(), openContext.OpenRequestID, result, runtimeInfo); reportErr != nil {
					finalErr = reportErr
				}
			}
			a.finishWorkspaceOpenRun(profileID, run, result, runtimeInfo, finalErr)
		}()
	} else {
		result, runtime, err := a.waitWorkspaceOpenRun(profileID)
		if err == nil {
			if reportErr := a.reportWorkspaceOpenResult(context.Background(), openContext.OpenRequestID, result, runtime); reportErr != nil {
				err = reportErr
			}
		}
		if err != nil {
			return nil, err
		}
		return result, nil
	}

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
		return result, nil
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
	return result, nil
}

func (a *App) initWorkspaceService() {
	client := workspace.NewWorkspaceClient(a.resolveWorkspaceAgentBaseURL(), nil)
	a.workspaceService = workspace.NewService(client, a.browserMgr)
	a.configureManagedInstanceRuntime()
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

func (a *App) resolveWorkspaceAgentBaseURL() string {
	if a != nil {
		if value := strings.TrimRight(strings.TrimSpace(a.workspaceAgentURL), "/"); value != "" {
			return value
		}
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

func (a *App) beginWorkspaceOpenRun(profileID string) *workspaceOpenRun {
	a.workspaceOpenMu.Lock()
	defer a.workspaceOpenMu.Unlock()
	if a.workspaceOpenRuns == nil {
		a.workspaceOpenRuns = make(map[string]*workspaceOpenRun)
	}
	if existing := a.workspaceOpenRuns[profileID]; existing != nil {
		return nil
	}
	run := &workspaceOpenRun{done: make(chan struct{})}
	a.workspaceOpenRuns[profileID] = run
	return run
}

func (a *App) finishWorkspaceOpenRun(profileID string, run *workspaceOpenRun, result *workspace.OpenShopResult, runtimeInfo *workspace.OpenReportRuntime, err error) {
	if run == nil {
		return
	}
	a.workspaceOpenMu.Lock()
	run.result = cloneWorkspaceOpenResult(result)
	run.runtime = cloneWorkspaceOpenRuntime(runtimeInfo)
	run.err = err
	delete(a.workspaceOpenRuns, profileID)
	close(run.done)
	a.workspaceOpenMu.Unlock()
}

func (a *App) waitWorkspaceOpenRun(profileID string) (*workspace.OpenShopResult, *workspace.OpenReportRuntime, error) {
	a.workspaceOpenMu.Lock()
	if a.workspaceOpenRuns == nil {
		a.workspaceOpenMu.Unlock()
		return nil, nil, fmt.Errorf("workspace open run not found")
	}
	run := a.workspaceOpenRuns[profileID]
	a.workspaceOpenMu.Unlock()
	if run == nil {
		return nil, nil, fmt.Errorf("workspace open run not found")
	}
	<-run.done
	return cloneWorkspaceOpenRunOutcome(run)
}

func cloneWorkspaceOpenRunOutcome(run *workspaceOpenRun) (*workspace.OpenShopResult, *workspace.OpenReportRuntime, error) {
	if run == nil {
		return nil, nil, nil
	}
	return cloneWorkspaceOpenResult(run.result), cloneWorkspaceOpenRuntime(run.runtime), run.err
}

func cloneWorkspaceOpenResult(result *workspace.OpenShopResult) *workspace.OpenShopResult {
	if result == nil {
		return nil
	}
	cloned := *result
	return &cloned
}

func cloneWorkspaceOpenRuntime(runtimeInfo *workspace.OpenReportRuntime) *workspace.OpenReportRuntime {
	if runtimeInfo == nil {
		return nil
	}
	cloned := *runtimeInfo
	return &cloned
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
