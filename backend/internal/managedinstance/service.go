package managedinstance

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/browser"
	"fmt"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
)

type Service struct {
	browserMgr  *browser.Manager
	runtimeMu   sync.RWMutex
	openRuntime NativeOpenRuntime
	openMu      sync.Mutex
	openRuns    map[string]*openRun
}

type openRun struct {
	done   chan struct{}
	result *OpenResult
	err    error
}

func NewService(deps Dependencies) (*Service, error) {
	if deps.BrowserMgr == nil {
		return nil, fmt.Errorf("managed instance service requires browser manager")
	}
	return &Service{
		browserMgr: deps.BrowserMgr,
		openRuns:   make(map[string]*openRun),
	}, nil
}

func (s *Service) ensureManagedProfileCore(profile *browser.Profile) error {
	corePath, err := s.ResolveManagedCore(profile)
	if err != nil {
		return err
	}

	if s.isSystemChromeExecutablePath(corePath) {
		if replacement, ok := s.findManagedFingerprintCore(profile.CoreId); ok {
			profile.CoreId = replacement.CoreId
			if err := s.persistProfile(profile); err != nil {
				return fmt.Errorf("ANT_CORE_UNAVAILABLE: persist managed fingerprint core migration: %w", err)
			}
			return nil
		}
		return fmt.Errorf("ANT_FINGERPRINT_CORE_REQUIRED: managed shop requires a fingerprint core, got system chrome executable %s", corePath)
	}

	profile.CoreId = strings.TrimSpace(profile.CoreId)
	if profile.CoreId == "" {
		if defaultCore, ok := s.browserMgr.GetDefaultCore(); ok {
			profile.CoreId = defaultCore.CoreId
		}
	}

	_ = corePath
	return nil
}

func (s *Service) findManagedFingerprintCore(currentCoreID string) (browser.Core, bool) {
	if s == nil || s.browserMgr == nil {
		return browser.Core{}, false
	}

	cores := s.browserMgr.ListCores()
	if defaultCore, ok := s.browserMgr.GetDefaultCore(); ok {
		cores = append([]browser.Core{defaultCore}, cores...)
	}

	var fallback browser.Core
	hasFallback := false
	for _, core := range cores {
		if strings.TrimSpace(core.CoreId) == "" || strings.EqualFold(strings.TrimSpace(core.CoreId), strings.TrimSpace(currentCoreID)) {
			continue
		}
		exePath, err := s.browserMgr.ResolveCoreExecutable(core)
		if err != nil || s.isSystemChromeExecutablePath(exePath) {
			continue
		}
		if isLikelyFingerprintCore(core) {
			return core, true
		}
		if !hasFallback {
			fallback = core
			hasFallback = true
		}
	}

	return fallback, hasFallback
}

func isLikelyFingerprintCore(core browser.Core) bool {
	text := strings.ToLower(filepath.ToSlash(strings.TrimSpace(core.CorePath) + " " + strings.TrimSpace(core.CoreName) + " " + strings.TrimSpace(core.CoreId)))
	return strings.Contains(text, "fingerprint") || strings.Contains(text, "指纹")
}

func (s *Service) isSystemChromeExecutablePath(corePath string) bool {
	corePath = filepath.Clean(strings.TrimSpace(corePath))
	if corePath == "." || corePath == "" {
		return false
	}

	appRoot := ""
	stateRoot := ""
	if s != nil && s.browserMgr != nil {
		appRoot = filepath.Clean(strings.TrimSpace(s.browserMgr.AppRoot))
		if appRoot != "" {
			stateRoot = filepath.Clean(apppath.StateRoot(appRoot))
		}
	}
	outsideTrustedRoots := !isPathInsideRoot(corePath, appRoot) && !isPathInsideRoot(corePath, stateRoot)

	switch goruntime.GOOS {
	case "darwin":
		return strings.HasPrefix(corePath, "/Applications/Google Chrome.app/") ||
			strings.HasPrefix(corePath, "/Applications/Chromium.app/") ||
			strings.HasPrefix(corePath, "/System/Applications/Google Chrome.app/") ||
			(outsideTrustedRoots && (strings.Contains(corePath, "/Google Chrome.app/Contents/MacOS/Google Chrome") ||
				strings.Contains(corePath, "/Chromium.app/Contents/MacOS/Chromium")))
	case "windows":
		lower := strings.ToLower(corePath)
		return strings.Contains(lower, `\google\chrome\application\chrome.exe`) ||
			strings.Contains(lower, `\chromium\application\chrome.exe`) ||
			(outsideTrustedRoots && strings.HasSuffix(lower, `\chrome.exe`) &&
				(strings.Contains(lower, `\google\chrome\`) || strings.Contains(lower, `\chromium\`)))
	default:
		return strings.HasPrefix(corePath, "/usr/bin/google-chrome") ||
			strings.HasPrefix(corePath, "/usr/bin/chromium") ||
			strings.HasPrefix(corePath, "/opt/google/chrome/") ||
			strings.HasPrefix(corePath, "/snap/chromium/") ||
			(outsideTrustedRoots && (strings.Contains(corePath, "/google-chrome") || strings.Contains(corePath, "/chromium")))
	}
}

func isPathInsideRoot(path string, root string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	root = filepath.Clean(strings.TrimSpace(root))
	if path == "." || root == "." || path == "" || root == "" {
		return false
	}
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (s *Service) beginOpenRun(profileID string) *openRun {
	if s == nil {
		return nil
	}
	s.openMu.Lock()
	defer s.openMu.Unlock()
	if existing := s.openRuns[profileID]; existing != nil {
		return nil
	}
	run := &openRun{done: make(chan struct{})}
	s.openRuns[profileID] = run
	return run
}

func (s *Service) finishOpenRun(profileID string, run *openRun, result *OpenResult, err error) {
	if s == nil || run == nil {
		return
	}
	s.openMu.Lock()
	run.result = cloneOpenResult(result)
	run.err = err
	delete(s.openRuns, profileID)
	close(run.done)
	s.openMu.Unlock()
}

func (s *Service) waitOpenRun(profileID string) (*OpenResult, error) {
	if s == nil {
		return nil, fmt.Errorf("managed instance service is not ready")
	}
	s.openMu.Lock()
	run := s.openRuns[profileID]
	s.openMu.Unlock()
	if run == nil {
		return nil, fmt.Errorf("managed open run not found")
	}
	<-run.done
	return cloneOpenResult(run.result), run.err
}

func cloneOpenResult(result *OpenResult) *OpenResult {
	if result == nil {
		return nil
	}
	cloned := *result
	return &cloned
}
