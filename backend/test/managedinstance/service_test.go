package managedinstance_test

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/managedinstance"
	"ant-chrome/backend/internal/workspace"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testBrowserConfig() *config.Config {
	return config.DefaultConfig()
}

func newManagerWithCore(t *testing.T, coreID, corePath string) *browser.Manager {
	t.Helper()

	appRoot := t.TempDir()
	exePath := filepath.Join(appRoot, filepath.FromSlash(corePath), filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("create core directory: %v", err)
	}
	if err := os.WriteFile(exePath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write core executable: %v", err)
	}

	cfg := testBrowserConfig()
	cfg.Browser.Cores = []config.BrowserCore{
		{
			CoreId:    coreID,
			CoreName:  "Fingerprint Chromium",
			CorePath:  corePath,
			IsDefault: true,
		},
	}

	return browser.NewManager(cfg, appRoot)
}

func TestOpenManagedShopRequestCarriesWorkspaceBusinessContext(t *testing.T) {
	sessionBundle := workspace.SessionBundle{
		PlatformCode:    "1688",
		LastObservedURL: "https://work.1688.com/dashboard",
		UserAgent:       "test-agent",
	}
	launchContext := workspace.ShopLaunchContext{
		TargetURL:     "https://work.1688.com/?shopId=b2b-222082061706256a1a",
		SessionBundle: sessionBundle,
	}
	req := managedinstance.OpenRequest{
		ShopID:        "b2b-222082061706256a1a",
		ProfileID:     "1688:b2b-222082061706256a1a",
		TargetURL:     "https://work.1688.com/?shopId=b2b-222082061706256a1a",
		LaunchContext: launchContext,
		SessionBundle: sessionBundle,
		ManagedMode:   true,
		SessionReady:  true,
		PreferVisible: true,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal request payload: %v", err)
	}
	if got["shopId"] != req.ShopID {
		t.Fatalf("expected shopId %q, got %#v", req.ShopID, got["shopId"])
	}
	if got["profileId"] != req.ProfileID {
		t.Fatalf("expected profileId %q, got %#v", req.ProfileID, got["profileId"])
	}
	if got["targetUrl"] != req.TargetURL {
		t.Fatalf("expected targetUrl %q, got %#v", req.TargetURL, got["targetUrl"])
	}
	launchContextJSON, ok := got["launchContext"].(map[string]any)
	if !ok {
		t.Fatalf("expected launchContext object, got %#v", got["launchContext"])
	}
	if launchContextJSON["targetUrl"] != launchContext.TargetURL {
		t.Fatalf("expected launchContext.targetUrl %q, got %#v", launchContext.TargetURL, launchContextJSON["targetUrl"])
	}
	launchContextSessionBundleJSON, ok := launchContextJSON["sessionBundle"].(map[string]any)
	if !ok {
		t.Fatalf("expected launchContext.sessionBundle object, got %#v", launchContextJSON["sessionBundle"])
	}
	if launchContextSessionBundleJSON["lastObservedUrl"] != sessionBundle.LastObservedURL {
		t.Fatalf("expected launchContext.sessionBundle.lastObservedUrl %q, got %#v", sessionBundle.LastObservedURL, launchContextSessionBundleJSON["lastObservedUrl"])
	}
	sessionBundleJSON, ok := got["sessionBundle"].(map[string]any)
	if !ok {
		t.Fatalf("expected sessionBundle object, got %#v", got["sessionBundle"])
	}
	if sessionBundleJSON["platformCode"] != sessionBundle.PlatformCode {
		t.Fatalf("expected sessionBundle.platformCode %q, got %#v", sessionBundle.PlatformCode, sessionBundleJSON["platformCode"])
	}
	if got["managedMode"] != req.ManagedMode {
		t.Fatalf("expected managedMode %v, got %#v", req.ManagedMode, got["managedMode"])
	}
	if got["sessionReady"] != req.SessionReady {
		t.Fatalf("expected sessionReady %v, got %#v", req.SessionReady, got["sessionReady"])
	}
	if got["preferVisible"] != req.PreferVisible {
		t.Fatalf("expected preferVisible %v, got %#v", req.PreferVisible, got["preferVisible"])
	}
}

func TestOpenManagedShopResultCarriesRuntimeOutcome(t *testing.T) {
	result := managedinstance.OpenResult{
		ProfileID:  "1688:b2b-222082061706256a1a",
		PID:        12345,
		DebugPort:  9222,
		CurrentURL: "https://work.1688.com/dashboard",
		PageTitle:  "1688 Workbench",
		Success:    true,
		Code:       "ok",
		Message:    "opened",
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal result payload: %v", err)
	}
	if got["profileId"] != result.ProfileID {
		t.Fatalf("expected profileId %q, got %#v", result.ProfileID, got["profileId"])
	}
	if got["pid"] != float64(result.PID) {
		t.Fatalf("expected pid %d, got %#v", result.PID, got["pid"])
	}
	if got["debugPort"] != float64(result.DebugPort) {
		t.Fatalf("expected debugPort %d, got %#v", result.DebugPort, got["debugPort"])
	}
	if got["currentUrl"] != result.CurrentURL {
		t.Fatalf("expected currentUrl %q, got %#v", result.CurrentURL, got["currentUrl"])
	}
	if got["pageTitle"] != result.PageTitle {
		t.Fatalf("expected pageTitle %q, got %#v", result.PageTitle, got["pageTitle"])
	}
	if got["success"] != result.Success {
		t.Fatalf("expected success %v, got %#v", result.Success, got["success"])
	}
	if got["code"] != result.Code {
		t.Fatalf("expected code %q, got %#v", result.Code, got["code"])
	}
	if got["message"] != result.Message {
		t.Fatalf("expected message %q, got %#v", result.Message, got["message"])
	}
}

func TestNewManagedInstanceServiceRequiresBrowserManager(t *testing.T) {
	_, err := managedinstance.NewService(managedinstance.Dependencies{})
	if err == nil {
		t.Fatal("expected missing browser manager error")
	}
}

func TestNewManagedInstanceServiceAcceptsBrowserManager(t *testing.T) {
	service, err := managedinstance.NewService(managedinstance.Dependencies{
		BrowserMgr: &browser.Manager{},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if service == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestResolveManagedCoreFailsWithoutConfiguredFingerprintCore(t *testing.T) {
	mgr := browser.NewManager(testBrowserConfig(), t.TempDir())
	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = service.ResolveManagedCore(&browser.Profile{
		ProfileId: "1688:b2b-222082061706256a1a",
		CoreId:    "",
	})
	if err == nil {
		t.Fatal("expected managed core resolution to fail")
	}
	if !strings.Contains(err.Error(), "ANT_FINGERPRINT_CORE_REQUIRED") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveManagedCoreUsesProfileCoreWhenConfigured(t *testing.T) {
	mgr := newManagerWithCore(t, "core-1688", "fingerprint-core")
	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	corePath, err := service.ResolveManagedCore(&browser.Profile{
		ProfileId: "1688:b2b-222082061706256a1a",
		CoreId:    "core-1688",
	})
	if err != nil {
		t.Fatalf("resolve core: %v", err)
	}

	expectedPath := filepath.Join(mgr.AppRoot, "fingerprint-core", filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if corePath != expectedPath {
		t.Fatalf("unexpected core path: got=%s want=%s", corePath, expectedPath)
	}
}

func TestOpenManagedShopReusesRunningProfileInServiceLayer(t *testing.T) {
	mgr := newManagerWithCore(t, "core-1688", "fingerprint-core")
	profileID := "1688:b2b-222082061706256a1a"
	profile := &browser.Profile{
		ProfileId:  profileID,
		CoreId:     "core-1688",
		Running:    true,
		DebugReady: true,
		DebugPort:  9222,
		Pid:        12345,
	}
	mgr.Profiles[profileID] = profile

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var startCalls atomic.Int32
	var activateCalls atomic.Int32
	var importCalls atomic.Int32
	service.SetOpenRuntime(managedinstance.NativeOpenRuntime{
		EnsureManagedProfile: func(req managedinstance.OpenRequest) (*browser.Profile, error) {
			return profile, nil
		},
		FindRunningProfile: func(profileID string) (*browser.Profile, bool) {
			return profile, true
		},
		StartManagedProfile: func(profileID string, targetURL string, preferVisible bool) (*browser.Profile, error) {
			startCalls.Add(1)
			return nil, nil
		},
		ImportCookies: func(profileID string, bundle workspace.SessionBundle) error {
			importCalls.Add(1)
			return nil
		},
		ListTargets: func(profileID string) ([]workspace.OpenRuntimeTarget, error) {
			return []workspace.OpenRuntimeTarget{{
				TargetID:   "backend-1",
				CurrentURL: "https://work.1688.com/?tracelog=login_target_is_blank_1688",
				PageTitle:  "1688-卖家工作台",
			}}, nil
		},
		ActivateTarget: func(profileID string, targetID string) error {
			activateCalls.Add(1)
			return nil
		},
		NavigateTarget: func(profileID string, targetID string, targetURL string) error { return nil },
		CreateTarget:   func(profileID string, targetURL string) (string, error) { return "", nil },
		WaitForTargetReady: func(profileID string, targetID string, timeout time.Duration) error {
			return nil
		},
		CloseTarget: func(profileID string, targetID string) error { return nil },
	})

	result, err := service.OpenManagedShop(managedinstance.OpenRequest{
		ShopID:      "b2b-222082061706256a1a",
		ProfileID:   profileID,
		ManagedMode: true,
		TargetURL:   "https://work.1688.com/?shopId=b2b-222082061706256a1a",
		LaunchContext: workspace.ShopLaunchContext{
			SuccessURLPatterns: []string{"https://work.1688.com/"},
			LoginURLPatterns:   []string{"https://login.1688.com/"},
		},
	})
	if err != nil {
		t.Fatalf("open managed shop: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success result, got %+v", result)
	}
	if startCalls.Load() != 0 {
		t.Fatalf("expected no cold start, got=%d", startCalls.Load())
	}
	if activateCalls.Load() != 1 {
		t.Fatalf("expected activate path once, got=%d", activateCalls.Load())
	}
	if importCalls.Load() != 0 {
		t.Fatalf("expected no cookie import on activate path, got=%d", importCalls.Load())
	}
}

func TestOpenManagedShopColdStartFlowRunsInsideServiceLayer(t *testing.T) {
	mgr := newManagerWithCore(t, "core-1688", "fingerprint-core")
	profileID := "1688:b2b-222082061706256a1a"
	profile := &browser.Profile{
		ProfileId: profileID,
		CoreId:    "core-1688",
		DebugPort: 9333,
		Pid:       54321,
	}
	mgr.Profiles[profileID] = profile

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var runningState atomic.Int32
	var listCalls atomic.Int32
	var startCalls atomic.Int32
	var importCalls atomic.Int32
	var navigateCalls atomic.Int32
	readyProfile := &browser.Profile{
		ProfileId:  profileID,
		CoreId:     "core-1688",
		Running:    true,
		DebugReady: true,
		DebugPort:  9333,
		Pid:        54321,
	}
	service.SetOpenRuntime(managedinstance.NativeOpenRuntime{
		EnsureManagedProfile: func(req managedinstance.OpenRequest) (*browser.Profile, error) {
			return profile, nil
		},
		FindRunningProfile: func(profileID string) (*browser.Profile, bool) {
			if runningState.Load() == 0 {
				return nil, false
			}
			return readyProfile, true
		},
		StartManagedProfile: func(profileID string, targetURL string, preferVisible bool) (*browser.Profile, error) {
			startCalls.Add(1)
			runningState.Store(1)
			return readyProfile, nil
		},
		ImportCookies: func(profileID string, bundle workspace.SessionBundle) error {
			importCalls.Add(1)
			return nil
		},
		ListTargets: func(profileID string) ([]workspace.OpenRuntimeTarget, error) {
			switch listCalls.Add(1) {
			case 1:
				return []workspace.OpenRuntimeTarget{{
					TargetID:   "blank-1",
					CurrentURL: "about:blank",
					PageTitle:  "新标签页",
				}}, nil
			default:
				return []workspace.OpenRuntimeTarget{{
					TargetID:   "blank-1",
					CurrentURL: "https://work.1688.com/?tracelog=login_target_is_blank_1688",
					PageTitle:  "1688-卖家工作台",
				}}, nil
			}
		},
		ActivateTarget: func(profileID string, targetID string) error { return nil },
		NavigateTarget: func(profileID string, targetID string, targetURL string) error {
			navigateCalls.Add(1)
			return nil
		},
		CreateTarget: func(profileID string, targetURL string) (string, error) { return "", nil },
		WaitForTargetReady: func(profileID string, targetID string, timeout time.Duration) error {
			return nil
		},
		CloseTarget: func(profileID string, targetID string) error { return nil },
	})

	result, err := service.OpenManagedShop(managedinstance.OpenRequest{
		ShopID:      "b2b-222082061706256a1a",
		ProfileID:   profileID,
		ManagedMode: true,
		TargetURL:   "about:blank",
		LaunchContext: workspace.ShopLaunchContext{
			SuccessURLPatterns: []string{"https://work.1688.com/"},
			LoginURLPatterns:   []string{"https://login.1688.com/"},
		},
		SessionBundle: workspace.SessionBundle{
			Cookies: []workspace.SessionCookie{{Name: "sid", Value: "1", Domain: ".1688.com", Path: "/"}},
		},
	})
	if err != nil {
		t.Fatalf("open managed shop: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success result, got %+v", result)
	}
	if startCalls.Load() != 1 {
		t.Fatalf("expected one cold start, got=%d", startCalls.Load())
	}
	if importCalls.Load() != 2 {
		t.Fatalf("expected import before reuse and navigate, got=%d", importCalls.Load())
	}
	if navigateCalls.Load() != 1 {
		t.Fatalf("expected one navigate, got=%d", navigateCalls.Load())
	}
	if result.CurrentURL != "https://work.1688.com/?tracelog=login_target_is_blank_1688" {
		t.Fatalf("unexpected current url: %s", result.CurrentURL)
	}
}
