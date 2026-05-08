package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/managedinstance"
	"ant-chrome/backend/internal/workspace"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *App) UpsertManagedProfile(input launchcode.ManagedProfileUpsertInput) (*launchcode.ManagedProfileUpsertResult, error) {
	profileID := strings.TrimSpace(input.ProfileID)
	shopID := strings.TrimSpace(input.ShopID)
	platformCode := strings.TrimSpace(input.PlatformCode)
	profileName := strings.TrimSpace(input.ProfileName)
	userDataDir := strings.TrimSpace(input.UserDataDir)

	if profileID == "" || shopID == "" || platformCode == "" || profileName == "" || userDataDir == "" {
		return nil, fmt.Errorf("invalid managed profile input")
	}
	if !input.ManagedMode {
		return nil, fmt.Errorf("managed mode required")
	}

	a.browserMgr.InitData()
	now := time.Now().Format(time.RFC3339)
	updated := false

	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileID]
	if exists && profile != nil {
		updated = true
		profile.ProfileName = profileName
		profile.UserDataDir = userDataDir
		profile.Tags = mergeManagedTags(profile.Tags, platformCode, shopID)
		profile.UpdatedAt = now
	} else {
		coreID := ""
		if defaultCore, ok := a.browserMgr.GetDefaultCore(); ok {
			coreID = defaultCore.CoreId
		}
		a.browserMgr.Profiles[profileID] = &browser.Profile{
			ProfileId:       profileID,
			ProfileName:     profileName,
			UserDataDir:     userDataDir,
			CoreId:          coreID,
			FingerprintArgs: []string{},
			LaunchArgs:      []string{},
			Tags:            mergeManagedTags(nil, platformCode, shopID),
			Keywords:        []string{},
			Running:         false,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		profile = a.browserMgr.Profiles[profileID]
	}
	a.browserMgr.Mutex.Unlock()

	if a.browserMgr.ProfileDAO != nil {
		if err := a.browserMgr.ProfileDAO.Upsert(profile); err != nil {
			return nil, err
		}
	} else if err := a.browserMgr.SaveProfiles(); err != nil {
		return nil, err
	}

	return &launchcode.ManagedProfileUpsertResult{
		ProfileID: profileID,
		Updated:   updated,
	}, nil
}

func (a *App) StopInstance(profileID string) (bool, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return false, fmt.Errorf("profile id is required")
	}
	if _, err := a.BrowserInstanceStop(profileID); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "profile not found") {
			return false, err
		}
		return false, err
	}
	return true, nil
}

func (a *App) ClearProfileSession(profileID string, clearCookies bool, clearStorage bool) error {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return fmt.Errorf("profile id is required")
	}

	var userDataDir string
	var running bool

	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileID]
	if !exists || profile == nil {
		a.browserMgr.Mutex.Unlock()
		return fmt.Errorf("profile not found")
	}
	userDataDir = strings.TrimSpace(profile.UserDataDir)
	running = profile.Running
	a.browserMgr.Mutex.Unlock()

	if clearCookies && running {
		if err := a.BrowserClearCookies(profileID); err != nil {
			return err
		}
	}

	if clearStorage {
		if running {
			if _, err := a.BrowserInstanceStop(profileID); err != nil {
				return err
			}
		}
		root := a.browserMgr.ResolveRelativePath(userDataDir)
		if err := clearManagedStorage(root); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) InjectManagedSessionBundle(profileID string, bundle workspace.SessionBundle) error {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return fmt.Errorf("profile id is required")
	}
	if len(bundle.Cookies) == 0 {
		return nil
	}
	return a.browserImportCookies(profileID, bundle.Cookies)
}

func (a *App) configureManagedInstanceRuntime() {
	if a == nil || a.managedInstanceService == nil {
		return
	}

	a.managedInstanceService.SetOpenRuntime(managedinstance.NativeOpenRuntime{
		EnsureManagedProfile: a.ensureManagedProfileForOpen,
		FindRunningProfile:   a.managedRunningProfileSnapshot,
		WaitForDebugReady:    a.waitForBrowserDebugReady,
		StartManagedProfile: func(profileID string, targetURL string, _ bool) (*BrowserProfile, error) {
			return a.BrowserInstanceStartWithParams(profileID, nil, []string{targetURL}, true)
		},
		ImportCookies:  a.importWorkspaceSessionBundle,
		ListTargets:    a.browserRuntimeTargets,
		ActivateTarget: a.browserActivateTarget,
		NavigateTarget: a.browserNavigateTarget,
		CreateTarget:   a.browserCreateTarget,
		WaitForTargetReady: func(profileID string, targetID string, timeout time.Duration) error {
			return a.waitForTargetReady(profileID, targetID, timeout)
		},
		CloseTarget: a.browserCloseTarget,
	})
}

func (a *App) ensureManagedProfileForOpen(req managedinstance.OpenRequest) (*BrowserProfile, error) {
	if a == nil || a.browserMgr == nil {
		return nil, fmt.Errorf("managed instance service is not ready")
	}

	profileID := strings.TrimSpace(req.ProfileID)
	if profileID == "" {
		return nil, fmt.Errorf("profile id is required")
	}

	platformCode := strings.TrimSpace(req.SessionBundle.PlatformCode)
	if platformCode == "" {
		platformCode = platformCodeFromProfileID(profileID)
	}
	if platformCode == "" {
		platformCode = "1688"
	}

	if _, err := a.UpsertManagedProfile(launchcode.ManagedProfileUpsertInput{
		ProfileID:    profileID,
		ShopID:       strings.TrimSpace(req.ShopID),
		PlatformCode: platformCode,
		ProfileName:  firstNonEmptyString(req.ShopID, profileID),
		ManagedMode:  true,
		UserDataDir:  filepath.Join("managed-profiles", strings.ReplaceAll(profileID, ":", "__")),
	}); err != nil {
		return nil, err
	}

	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileID]
	if !exists || profile == nil {
		a.browserMgr.Mutex.Unlock()
		return nil, fmt.Errorf("managed profile not found: %s", profileID)
	}
	snapshot := copyBrowserProfileSnapshot(profile)
	a.browserMgr.Mutex.Unlock()
	return snapshot, nil
}

func (a *App) managedRunningProfileSnapshot(profileID string) (*BrowserProfile, bool) {
	if a == nil || a.browserMgr == nil {
		return nil, false
	}

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
		return nil, false
	}
	snapshot := copyBrowserProfileSnapshot(profile)
	a.browserMgr.Mutex.Unlock()
	return snapshot, true
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
		logger.New("ManagedOpen").Warn("浏览器应用激活失败",
			logger.F("profile_id", profileID),
			logger.F("reason", err.Error()),
		)
		return
	}
	if err := activateBrowserApp(chromeBinaryPath); err != nil {
		logger.New("ManagedOpen").Warn("浏览器应用激活失败",
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

func platformCodeFromProfileID(profileID string) string {
	profileID = strings.TrimSpace(profileID)
	if idx := strings.Index(profileID, ":"); idx > 0 {
		return strings.TrimSpace(profileID[:idx])
	}
	return ""
}

func mergeManagedTags(existing []string, platformCode, shopID string) []string {
	seen := make(map[string]struct{})
	tags := make([]string, 0, len(existing)+4)
	for _, tag := range existing {
		t := strings.TrimSpace(tag)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		tags = append(tags, t)
	}

	addTag := func(tag string) {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return
		}
		if _, ok := seen[tag]; ok {
			return
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}

	addTag("managed")
	addTag("managed:desktop")
	addTag("platform:" + platformCode)
	addTag("shop:" + shopID)
	return tags
}

func clearManagedStorage(userDataRoot string) error {
	userDataRoot = strings.TrimSpace(userDataRoot)
	if userDataRoot == "" {
		return nil
	}

	targets := []string{
		filepath.Join(userDataRoot, "Default", "Local Storage"),
		filepath.Join(userDataRoot, "Default", "Session Storage"),
		filepath.Join(userDataRoot, "Default", "IndexedDB"),
		filepath.Join(userDataRoot, "Default", "Service Worker"),
		filepath.Join(userDataRoot, "Default", "WebStorage"),
		filepath.Join(userDataRoot, "Default", "Code Cache"),
		filepath.Join(userDataRoot, "Default", "Cache"),
	}

	for _, target := range targets {
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	return nil
}
