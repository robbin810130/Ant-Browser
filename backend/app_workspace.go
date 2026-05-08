package backend

import (
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/workspace"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultWorkspaceAgentBaseURL = "http://127.0.0.1:47831"

type workspaceOpenRun struct {
	done    chan struct{}
	result  *workspace.OpenShopResult
	runtime *workspace.OpenReportRuntime
	err     error
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

	if err := a.ensureManagedShopProfile(profileID, shop); err != nil {
		result.Code = "ANT_PROFILE_UPSERT_FAILED"
		result.Message = err.Error()
		return result, nil
	}

	targetURL := workspace.ResolveWorkspaceTargetURL(result.ShopID, openContext.LaunchContext.TargetURL)
	log.Info("准备打开店铺后台",
		logger.F("shop_id", result.ShopID),
		logger.F("profile_id", profileID),
		logger.F("open_request_id", openContext.OpenRequestID),
		logger.F("launch_target_url", strings.TrimSpace(openContext.LaunchContext.TargetURL)),
		logger.F("resolved_target_url", targetURL),
		logger.F("success_url_patterns", strings.Join(openContext.LaunchContext.SuccessURLPatterns, ",")),
		logger.F("login_url_patterns", strings.Join(openContext.LaunchContext.LoginURLPatterns, ",")),
	)
	result, runtimeInfo = a.executeWorkspaceOpen(result.ShopID, profileID, targetURL, openContext.LaunchContext, openContext.LaunchContext.SessionBundle, 10*time.Second)
	return result, nil
}

func (a *App) initWorkspaceService() {
	client := workspace.NewWorkspaceClient(a.resolveWorkspaceAgentBaseURL(), nil)
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

func (a *App) executeWorkspaceOpen(shopID string, profileID string, targetURL string, launchContext workspace.ShopLaunchContext, sessionBundle workspace.SessionBundle, timeout time.Duration) (*workspace.OpenShopResult, *workspace.OpenReportRuntime) {
	log := logger.New("WorkspaceOpen")
	if profile, ok := a.workspaceRunningProfile(profileID, timeout); ok {
		log.Info("检测到运行中实例，尝试复用",
			logger.F("shop_id", shopID),
			logger.F("profile_id", profileID),
			logger.F("pid", profile.Pid),
			logger.F("debug_port", profile.DebugPort),
			logger.F("debug_ready", profile.DebugReady),
			logger.F("target_url", targetURL),
		)
		result, runtimeInfo, err := a.reuseWorkspaceRunningProfile(shopID, profile, targetURL, launchContext, sessionBundle, timeout)
		if err == nil {
			return result, runtimeInfo
		}
		log.Error("运行中实例复用失败",
			logger.F("shop_id", shopID),
			logger.F("profile_id", profileID),
			logger.F("debug_port", profile.DebugPort),
			logger.F("error", err.Error()),
		)
		return &workspace.OpenShopResult{
			ShopID:     shopID,
			ProfileID:  profileID,
			InstanceID: "",
			Code:       "ANT_INSTANCE_OPEN_FAILED",
			Message:    err.Error(),
		}, runtimeInfo
	}

	log.Info("未发现可复用实例，执行冷启动",
		logger.F("shop_id", shopID),
		logger.F("profile_id", profileID),
		logger.F("target_url", targetURL),
	)
	profile, err := a.BrowserInstanceStartWithParams(profileID, nil, []string{targetURL}, true)
	if err != nil {
		return &workspace.OpenShopResult{
			ShopID:     shopID,
			ProfileID:  profileID,
			InstanceID: "",
			Code:       "ANT_INSTANCE_OPEN_FAILED",
			Message:    err.Error(),
		}, nil
	}

	if err := a.importWorkspaceSessionBundle(profileID, sessionBundle); err != nil {
		return &workspace.OpenShopResult{
			ShopID:     shopID,
			ProfileID:  profileID,
			InstanceID: "",
			Code:       "ANT_SESSION_RESTORE_FAILED",
			Message:    err.Error(),
		}, nil
	}

	if runtimeProfile, ok := a.workspaceRunningProfile(profileID, timeout); ok {
		result, runtimeInfo, err := a.reuseWorkspaceRunningProfile(shopID, runtimeProfile, targetURL, launchContext, sessionBundle, timeout)
		if err == nil {
			return result, runtimeInfo
		}
		log.Error("冷启动后的实例复用失败",
			logger.F("shop_id", shopID),
			logger.F("profile_id", profileID),
			logger.F("debug_port", runtimeProfile.DebugPort),
			logger.F("error", err.Error()),
		)
		return &workspace.OpenShopResult{
			ShopID:     shopID,
			ProfileID:  profileID,
			InstanceID: "",
			Code:       "ANT_INSTANCE_OPEN_FAILED",
			Message:    err.Error(),
		}, nil
	}

	runtimeInfo := &workspace.OpenReportRuntime{
		PID:       profile.Pid,
		DebugPort: profile.DebugPort,
	}
	result := a.waitForWorkspaceOpenResult(shopID, profileID, "", launchContext, timeout)
	runtimeInfo.CurrentURL = result.CurrentURL
	runtimeInfo.PageTitle = result.PageTitle
	if result.Success {
		_ = a.cleanupWorkspaceBlankTargets(shopID, profileID, launchContext)
	}
	return result, runtimeInfo
}

func (a *App) workspaceRunningProfile(profileID string, waitTimeout time.Duration) (*BrowserProfile, bool) {
	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileID]
	if !exists || profile == nil || !profile.Running {
		a.browserMgr.Mutex.Unlock()
		return nil, false
	}
	trackedCmd := a.browserMgr.BrowserProcesses[profileID]
	if !isBrowserProfileLive(profile, trackedCmd) {
		a.markProfileStoppedLocked(profileID, profile)
		a.browserMgr.Mutex.Unlock()
		logger.New("WorkspaceOpen").Warn("运行中实例已失效，转为冷启动",
			logger.F("profile_id", profileID),
			logger.F("pid", profile.Pid),
			logger.F("debug_port", profile.DebugPort),
		)
		return nil, false
	}
	snapshot := copyBrowserProfileSnapshot(profile)
	a.browserMgr.Mutex.Unlock()

	if snapshot.DebugReady {
		return snapshot, true
	}
	if snapshot.DebugPort > 0 && waitTimeout > 0 {
		if readySnapshot, _ := a.waitForBrowserDebugReady(profileID, snapshot.DebugPort, waitTimeout); readySnapshot != nil {
			return readySnapshot, true
		}
	}
	logger.New("WorkspaceOpen").Warn("实例存活但调试接口未就绪，暂不复用",
		logger.F("profile_id", profileID),
		logger.F("pid", snapshot.Pid),
		logger.F("debug_port", snapshot.DebugPort),
		logger.F("wait_timeout_ms", waitTimeout.Milliseconds()),
	)
	return nil, false
}

func (a *App) reuseWorkspaceRunningProfile(shopID string, profile *BrowserProfile, targetURL string, launchContext workspace.ShopLaunchContext, sessionBundle workspace.SessionBundle, timeout time.Duration) (*workspace.OpenShopResult, *workspace.OpenReportRuntime, error) {
	log := logger.New("WorkspaceOpen")
	targets, err := a.browserRuntimeTargets(profile.ProfileId)
	if err != nil {
		return nil, nil, err
	}

	target, action, ok := workspace.PickOpenTargetForLaunchContext(shopID, launchContext, targets)
	if !ok {
		return nil, nil, fmt.Errorf("未找到可复用的浏览器页面")
	}
	log.Info("运行中实例页面决策",
		logger.F("shop_id", shopID),
		logger.F("profile_id", profile.ProfileId),
		logger.F("action", string(action)),
		logger.F("target_id", target.TargetID),
		logger.F("target_url", target.CurrentURL),
		logger.F("target_title", target.PageTitle),
		logger.F("target_count", len(targets)),
		logger.F("desired_url", targetURL),
	)

	switch action {
	case workspace.OpenTargetActionActivate:
		if err := a.browserActivateTarget(profile.ProfileId, target.TargetID); err != nil {
			return nil, nil, err
		}
	case workspace.OpenTargetActionNavigate:
		if err := a.importWorkspaceSessionBundle(profile.ProfileId, sessionBundle); err != nil {
			return nil, nil, fmt.Errorf("导入会话失败（navigate）: %w", err)
		}
		if err := a.browserNavigateTarget(profile.ProfileId, target.TargetID, targetURL); err != nil {
			return nil, nil, err
		}
	case workspace.OpenTargetActionCreate:
		targetID, err := a.browserCreateTarget(profile.ProfileId, targetURL)
		if err != nil {
			return nil, nil, err
		}
		if err := a.waitForTargetReady(profile.ProfileId, targetID, 2*time.Second); err != nil {
			return nil, nil, err
		}
		target.TargetID = targetID
		if err := a.importWorkspaceSessionBundle(profile.ProfileId, sessionBundle); err != nil {
			return nil, nil, fmt.Errorf("导入会话失败（create）: %w", err)
		}
		if err := a.browserNavigateTarget(profile.ProfileId, target.TargetID, targetURL); err != nil {
			return nil, nil, err
		}
	}

	result := a.waitForWorkspaceOpenResult(shopID, profile.ProfileId, "", launchContext, timeout)
	runtimeInfo := &workspace.OpenReportRuntime{
		PID:        profile.Pid,
		DebugPort:  profile.DebugPort,
		CurrentURL: result.CurrentURL,
		PageTitle:  result.PageTitle,
	}
	if result.Success {
		_ = a.cleanupWorkspaceBlankTargets(shopID, profile.ProfileId, launchContext)
	}
	return result, runtimeInfo, nil
}

func (a *App) browserActivateTarget(profileID string, targetID string) error {
	debugPort, err := a.getDebugPort(profileID)
	if err != nil {
		return err
	}
	if _, err := cdpBrowserCallWithResult(debugPort, "Target.activateTarget", map[string]any{"targetId": strings.TrimSpace(targetID)}); err != nil {
		return fmt.Errorf("激活目标页面失败（target=%s）: %w", strings.TrimSpace(targetID), err)
	}
	_, err = cdpCallTarget(debugPort, targetID, "Page.bringToFront", map[string]any{})
	if err != nil {
		return fmt.Errorf("置顶目标页面失败（target=%s）: %w", strings.TrimSpace(targetID), err)
	}
	a.focusBrowserAppForProfile(profileID)
	return err
}

func (a *App) browserNavigateTarget(profileID string, targetID string, targetURL string) error {
	debugPort, err := a.getDebugPort(profileID)
	if err != nil {
		return err
	}
	if _, err := cdpCallTarget(debugPort, targetID, "Page.enable", map[string]any{}); err != nil {
		return fmt.Errorf("启用目标页面失败（target=%s）: %w", strings.TrimSpace(targetID), err)
	}
	if _, err := cdpCallTarget(debugPort, targetID, "Page.bringToFront", map[string]any{}); err != nil {
		return fmt.Errorf("置顶目标页面失败（target=%s）: %w", strings.TrimSpace(targetID), err)
	}
	_, err = cdpCallTarget(debugPort, targetID, "Page.navigate", map[string]any{
		"url": strings.TrimSpace(targetURL),
	})
	if err != nil {
		return fmt.Errorf("导航目标页面失败（target=%s url=%s）: %w", strings.TrimSpace(targetID), strings.TrimSpace(targetURL), err)
	}
	a.focusBrowserAppForProfile(profileID)
	return err
}

func (a *App) browserCreateTarget(profileID string, targetURL string) (string, error) {
	debugPort, err := a.getDebugPort(profileID)
	if err != nil {
		return "", err
	}
	result, err := cdpBrowserCallWithResult(debugPort, "Target.createTarget", map[string]any{
		"url": strings.TrimSpace(targetURL),
	})
	if err != nil {
		return "", fmt.Errorf("创建目标页面失败（url=%s）: %w", strings.TrimSpace(targetURL), err)
	}
	targetID, _ := result["targetId"].(string)
	if strings.TrimSpace(targetID) == "" {
		return "", fmt.Errorf("创建页面后未返回 targetId")
	}
	return strings.TrimSpace(targetID), nil
}

func (a *App) waitForTargetReady(profileID string, targetID string, timeout time.Duration) error {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return fmt.Errorf("等待目标页面就绪失败：targetId 为空")
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		targets, err := a.browserRuntimeTargets(profileID)
		if err == nil {
			for _, target := range targets {
				if strings.TrimSpace(target.TargetID) == targetID {
					return nil
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("等待目标页面就绪超时（target=%s timeout=%s）", targetID, timeout.Round(100*time.Millisecond))
}

func (a *App) focusBrowserAppForProfile(profileID string) {
	if a == nil || a.browserMgr == nil {
		return
	}

	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileID]
	a.browserMgr.Mutex.Unlock()
	if !exists || profile == nil {
		return
	}

	chromeBinaryPath, err := a.browserMgr.ResolveChromeBinary(profile)
	if err != nil {
		logger.New("WorkspaceOpen").Warn("浏览器应用激活失败",
			logger.F("profile_id", profileID),
			logger.F("reason", err.Error()),
		)
		return
	}
	if err := activateBrowserApp(chromeBinaryPath); err != nil {
		logger.New("WorkspaceOpen").Warn("浏览器应用激活失败",
			logger.F("profile_id", profileID),
			logger.F("chrome", chromeBinaryPath),
			logger.F("reason", err.Error()),
		)
	}
}

func (a *App) browserCloseTarget(profileID string, targetID string) error {
	debugPort, err := a.getDebugPort(profileID)
	if err != nil {
		return err
	}
	_, err = cdpBrowserCallWithResult(debugPort, "Target.closeTarget", map[string]any{"targetId": strings.TrimSpace(targetID)})
	if err != nil {
		return fmt.Errorf("关闭目标页面失败（target=%s）: %w", strings.TrimSpace(targetID), err)
	}
	return err
}

func (a *App) cleanupWorkspaceBlankTargets(shopID string, profileID string, launchContext workspace.ShopLaunchContext) error {
	log := logger.New("WorkspaceOpen")
	var lastErr error
	for attempt := 1; attempt <= 4; attempt++ {
		targets, err := a.browserRuntimeTargets(profileID)
		if err != nil {
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		target, action, ok := workspace.PickOpenTargetForLaunchContext(shopID, launchContext, targets)
		if !ok || action != workspace.OpenTargetActionActivate {
			log.Info("跳过空白页清理：当前无已就绪后台页",
				logger.F("shop_id", shopID),
				logger.F("profile_id", profileID),
				logger.F("attempt", attempt),
				logger.F("action", string(action)),
				logger.F("target_count", len(targets)),
			)
			time.Sleep(200 * time.Millisecond)
			continue
		}
		blankTargetIDs := workspace.CollectClosableBlankTargetIDs(targets, target.TargetID)
		if len(blankTargetIDs) == 0 {
			return nil
		}
		log.Info("清理空白页",
			logger.F("shop_id", shopID),
			logger.F("profile_id", profileID),
			logger.F("attempt", attempt),
			logger.F("keep_target_id", target.TargetID),
			logger.F("blank_target_count", len(blankTargetIDs)),
		)
		for _, targetID := range blankTargetIDs {
			if err := a.browserCloseTarget(profileID, targetID); err != nil {
				lastErr = err
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return lastErr
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

func (a *App) importWorkspaceSessionBundle(profileID string, bundle workspace.SessionBundle) error {
	if len(bundle.Cookies) == 0 {
		return nil
	}
	return a.browserImportCookies(profileID, bundle.Cookies)
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
