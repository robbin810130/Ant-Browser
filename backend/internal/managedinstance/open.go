package managedinstance

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/workspace"
	"fmt"
	"strings"
	"time"
)

const (
	managedOpenTimeout       = 10 * time.Second
	managedTargetReadyTimout = 2 * time.Second
)

type NativeOpenRuntime struct {
	EnsureManagedProfile func(req OpenRequest) (*browser.Profile, error)
	FindRunningProfile   func(profileID string) (*browser.Profile, bool)
	WaitForDebugReady    func(profileID string, debugPort int, timeout time.Duration) (*browser.Profile, bool)
	StartManagedProfile  func(profileID string, targetURL string, preferVisible bool) (*browser.Profile, error)
	StopManagedProfile   func(profileID string) error
	ImportCookies        func(profileID string, bundle workspace.SessionBundle) error
	ListTargets          func(profileID string) ([]workspace.OpenRuntimeTarget, error)
	ActivateTarget       func(profileID string, targetID string) error
	NavigateTarget       func(profileID string, targetID string, targetURL string) error
	CreateTarget         func(profileID string, targetURL string) (string, error)
	WaitForTargetReady   func(profileID string, targetID string, timeout time.Duration) error
	CloseTarget          func(profileID string, targetID string) error
}

func (s *Service) SetOpenRuntime(runtime NativeOpenRuntime) {
	if s == nil {
		return
	}
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	s.openRuntime = runtime
}

func (s *Service) OpenManagedShop(req OpenRequest) (*OpenResult, error) {
	if strings.TrimSpace(req.ProfileID) == "" || strings.TrimSpace(req.ShopID) == "" {
		return nil, fmt.Errorf("managed open requires profileID and shopID")
	}
	if !req.ManagedMode {
		return nil, fmt.Errorf("managed open requires managed mode")
	}

	req = normalizeOpenRequest(req)
	if run := s.beginOpenRun(req.ProfileID); run != nil {
		var (
			result *OpenResult
			err    error
		)
		defer func() {
			s.finishOpenRun(req.ProfileID, run, result, err)
		}()

		profile, profileErr := s.ensureManagedProfile(req)
		if profileErr != nil {
			err = profileErr
			return nil, err
		}
		if coreErr := s.ensureManagedProfileCore(profile); coreErr != nil {
			err = coreErr
			return nil, err
		}

		result, err = s.openViaNativeInstance(profile, req)
		return result, err
	}

	return s.waitOpenRun(req.ProfileID)
}

func normalizeOpenRequest(req OpenRequest) OpenRequest {
	req.ProfileID = strings.TrimSpace(req.ProfileID)
	req.ShopID = strings.TrimSpace(req.ShopID)
	req.TargetURL = workspace.ResolveWorkspaceTargetURL(req.ShopID, firstNonEmptyString(req.TargetURL, req.LaunchContext.TargetURL))
	req.LaunchContext.TargetURL = req.TargetURL
	if len(req.SessionBundle.Cookies) == 0 && len(req.LaunchContext.SessionBundle.Cookies) > 0 {
		req.SessionBundle = req.LaunchContext.SessionBundle
	}
	return req
}

func (s *Service) ensureManagedProfile(req OpenRequest) (*browser.Profile, error) {
	runtime, err := s.getOpenRuntime()
	if err != nil {
		return nil, err
	}
	if runtime.EnsureManagedProfile != nil {
		profile, err := runtime.EnsureManagedProfile(req)
		if err != nil {
			return nil, err
		}
		if profile != nil {
			return profile, nil
		}
	}
	return s.ensureAuthorizedProfile(req)
}

func (s *Service) ensureAuthorizedProfile(req OpenRequest) (*browser.Profile, error) {
	if s == nil || s.browserMgr == nil {
		return nil, fmt.Errorf("managed instance service is not ready")
	}

	profileID := strings.TrimSpace(req.ProfileID)
	s.browserMgr.InitData()
	s.browserMgr.Mutex.Lock()
	profile, exists := s.browserMgr.Profiles[profileID]
	if !exists || profile == nil {
		s.browserMgr.Mutex.Unlock()
		return nil, fmt.Errorf("managed profile not found: %s", profileID)
	}
	snapshot := *profile
	s.browserMgr.Mutex.Unlock()
	return &snapshot, nil
}

func (s *Service) openViaNativeInstance(profile *browser.Profile, req OpenRequest) (*OpenResult, error) {
	runtime, err := s.getOpenRuntime()
	if err != nil {
		return nil, err
	}

	if reusable, ok := s.reusableRunningProfile(runtime, profile.ProfileId); ok {
		return s.reuseRunningProfile(runtime, reusable, req)
	}

	started, err := runtime.StartManagedProfile(profile.ProfileId, req.TargetURL, req.PreferVisible)
	if err != nil {
		return nil, err
	}
	if started == nil {
		return nil, fmt.Errorf("managed profile start returned nil profile")
	}
	if err := runtime.ImportCookies(profile.ProfileId, req.SessionBundle); err != nil {
		return nil, err
	}

	if reusable, ok := s.reusableRunningProfile(runtime, profile.ProfileId); ok {
		return s.reuseRunningProfile(runtime, reusable, req)
	}

	result := s.waitForOpenResult(runtime, req.ShopID, profile.ProfileId, req.LaunchContext, started, managedOpenTimeout)
	if result.Success {
		_ = s.cleanupBlankTargets(runtime, req.ShopID, profile.ProfileId, req.LaunchContext)
	}
	return result, nil
}

func (s *Service) reusableRunningProfile(runtime NativeOpenRuntime, profileID string) (*browser.Profile, bool) {
	if runtime.FindRunningProfile == nil {
		return nil, false
	}

	profile, ok := runtime.FindRunningProfile(strings.TrimSpace(profileID))
	if !ok || profile == nil {
		return nil, false
	}
	if profile.DebugReady {
		return profile, true
	}
	if profile.DebugPort > 0 && runtime.WaitForDebugReady != nil {
		if readyProfile, ok := runtime.WaitForDebugReady(profile.ProfileId, profile.DebugPort, managedOpenTimeout); ok && readyProfile != nil {
			return readyProfile, true
		}
	}
	return nil, false
}

func (s *Service) reuseRunningProfile(runtime NativeOpenRuntime, profile *browser.Profile, req OpenRequest) (*OpenResult, error) {
	targets, err := runtime.ListTargets(profile.ProfileId)
	if err != nil {
		return nil, err
	}

	target, action, ok := workspace.PickOpenTargetForLaunchContext(req.ShopID, req.LaunchContext, targets)
	if !ok {
		return nil, fmt.Errorf("未找到可复用的浏览器页面")
	}

	switch action {
	case workspace.OpenTargetActionActivate:
		if err := runtime.ActivateTarget(profile.ProfileId, target.TargetID); err != nil {
			return nil, err
		}
	case workspace.OpenTargetActionNavigate:
		if err := runtime.ImportCookies(profile.ProfileId, req.SessionBundle); err != nil {
			return nil, fmt.Errorf("导入会话失败（navigate）: %w", err)
		}
		if err := runtime.NavigateTarget(profile.ProfileId, target.TargetID, req.TargetURL); err != nil {
			return nil, err
		}
	case workspace.OpenTargetActionCreate:
		targetID, err := runtime.CreateTarget(profile.ProfileId, req.TargetURL)
		if err != nil {
			return nil, err
		}
		if err := runtime.WaitForTargetReady(profile.ProfileId, targetID, managedTargetReadyTimout); err != nil {
			return nil, err
		}
		if err := runtime.ImportCookies(profile.ProfileId, req.SessionBundle); err != nil {
			return nil, fmt.Errorf("导入会话失败（create）: %w", err)
		}
		if err := runtime.NavigateTarget(profile.ProfileId, targetID, req.TargetURL); err != nil {
			return nil, err
		}
	}

	result := s.waitForOpenResult(runtime, req.ShopID, profile.ProfileId, req.LaunchContext, profile, managedOpenTimeout)
	if result.Success {
		_ = s.cleanupBlankTargets(runtime, req.ShopID, profile.ProfileId, req.LaunchContext)
	}
	return result, nil
}

func (s *Service) waitForOpenResult(runtime NativeOpenRuntime, shopID string, profileID string, launchContext workspace.ShopLaunchContext, profile *browser.Profile, timeout time.Duration) *OpenResult {
	deadline := time.Now().Add(timeout)
	lastSnapshot := workspace.OpenRuntimeSnapshot{}

	for time.Now().Before(deadline) {
		targets, err := runtime.ListTargets(profileID)
		if err == nil {
			snapshots := runtimeTargetsToSnapshots(targets)
			snapshot := workspace.SelectPreferredOpenSnapshotForLaunchContext(shopID, launchContext, snapshots)
			lastSnapshot = snapshot
			result := workspace.ClassifyOpenResultForLaunchContext(shopID, launchContext, snapshot)
			if result.Success || result.Code == "ANT_BACKEND_LOGIN_REQUIRED" || result.Code == "ANT_BACKEND_TARGET_MISMATCH" || result.Code == "ANT_MANUAL_VERIFICATION_REQUIRED" {
				return buildOpenResult(profileID, profile, result, snapshot)
			}
		}
		time.Sleep(350 * time.Millisecond)
	}

	result := workspace.ClassifyOpenResultForLaunchContext(shopID, launchContext, lastSnapshot)
	if result.Code == "" && !result.Success {
		result.Code = "ANT_INSTANCE_OPEN_FAILED"
		result.Message = "未能打开目标店铺后台，请稍后重试"
	}
	return buildOpenResult(profileID, profile, result, lastSnapshot)
}

func (s *Service) cleanupBlankTargets(runtime NativeOpenRuntime, shopID string, profileID string, launchContext workspace.ShopLaunchContext) error {
	var lastErr error
	for attempt := 1; attempt <= 4; attempt++ {
		targets, err := runtime.ListTargets(profileID)
		if err != nil {
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		target, action, ok := workspace.PickOpenTargetForLaunchContext(shopID, launchContext, targets)
		if !ok || action != workspace.OpenTargetActionActivate {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		blankTargetIDs := workspace.CollectClosableBlankTargetIDs(targets, target.TargetID)
		if len(blankTargetIDs) == 0 {
			return nil
		}
		for _, targetID := range blankTargetIDs {
			if err := runtime.CloseTarget(profileID, targetID); err != nil {
				lastErr = err
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return lastErr
}

func buildOpenResult(profileID string, profile *browser.Profile, classified workspace.OpenShopResult, snapshot workspace.OpenRuntimeSnapshot) *OpenResult {
	result := &OpenResult{
		ProfileID:  strings.TrimSpace(profileID),
		CurrentURL: snapshot.CurrentURL,
		PageTitle:  snapshot.PageTitle,
		Success:    classified.Success,
		Code:       classified.Code,
		Message:    classified.Message,
	}
	if profile != nil {
		result.PID = profile.Pid
		result.DebugPort = profile.DebugPort
	}
	return result
}

func runtimeTargetsToSnapshots(targets []workspace.OpenRuntimeTarget) []workspace.OpenRuntimeSnapshot {
	snapshots := make([]workspace.OpenRuntimeSnapshot, 0, len(targets))
	for _, target := range targets {
		snapshots = append(snapshots, workspace.OpenRuntimeSnapshot{
			CurrentURL: strings.TrimSpace(target.CurrentURL),
			PageTitle:  strings.TrimSpace(target.PageTitle),
		})
	}
	return snapshots
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

func (s *Service) getOpenRuntime() (NativeOpenRuntime, error) {
	if s == nil {
		return NativeOpenRuntime{}, fmt.Errorf("managed instance service is not ready")
	}

	s.runtimeMu.RLock()
	runtime := s.openRuntime
	s.runtimeMu.RUnlock()

	if runtime.StartManagedProfile == nil || runtime.ImportCookies == nil || runtime.ListTargets == nil || runtime.ActivateTarget == nil || runtime.NavigateTarget == nil || runtime.CreateTarget == nil || runtime.WaitForTargetReady == nil || runtime.CloseTarget == nil {
		return NativeOpenRuntime{}, fmt.Errorf("managed open runtime is unavailable")
	}
	return runtime, nil
}
