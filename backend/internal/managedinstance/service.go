package managedinstance

import (
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

func (s *Service) isSystemChromeExecutablePath(corePath string) bool {
	corePath = filepath.Clean(strings.TrimSpace(corePath))
	if corePath == "." || corePath == "" {
		return false
	}

	appRoot := ""
	if s != nil && s.browserMgr != nil {
		appRoot = filepath.Clean(strings.TrimSpace(s.browserMgr.AppRoot))
	}
	outsideAppRoot := appRoot != "" && corePath != appRoot
	if outsideAppRoot {
		if rel, err := filepath.Rel(appRoot, corePath); err == nil {
			outsideAppRoot = strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".."
		}
	}

	switch goruntime.GOOS {
	case "darwin":
		return strings.HasPrefix(corePath, "/Applications/Google Chrome.app/") ||
			strings.HasPrefix(corePath, "/Applications/Chromium.app/") ||
			strings.HasPrefix(corePath, "/System/Applications/Google Chrome.app/") ||
			(outsideAppRoot && (strings.Contains(corePath, "/Google Chrome.app/Contents/MacOS/Google Chrome") ||
				strings.Contains(corePath, "/Chromium.app/Contents/MacOS/Chromium")))
	case "windows":
		lower := strings.ToLower(corePath)
		return strings.Contains(lower, `\google\chrome\application\chrome.exe`) ||
			strings.Contains(lower, `\chromium\application\chrome.exe`) ||
			(outsideAppRoot && strings.HasSuffix(lower, `\chrome.exe`) &&
				(strings.Contains(lower, `\google\chrome\`) || strings.Contains(lower, `\chromium\`)))
	default:
		return strings.HasPrefix(corePath, "/usr/bin/google-chrome") ||
			strings.HasPrefix(corePath, "/usr/bin/chromium") ||
			strings.HasPrefix(corePath, "/opt/google/chrome/") ||
			strings.HasPrefix(corePath, "/snap/chromium/") ||
			(outsideAppRoot && (strings.Contains(corePath, "/google-chrome") || strings.Contains(corePath, "/chromium")))
	}
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
