package managedinstance

import (
	"ant-chrome/backend/internal/browser"
	"fmt"
	"strings"
)

func (s *Service) ResolveManagedCore(profile *browser.Profile) (string, error) {
	if s == nil || s.browserMgr == nil || profile == nil {
		return "", fmt.Errorf("ANT_CORE_UNAVAILABLE: managed instance service is not ready")
	}

	coreID := strings.TrimSpace(profile.CoreId)
	if coreID == "" {
		if defaultCore, ok := s.browserMgr.GetDefaultCore(); ok {
			coreID = defaultCore.CoreId
		}
	}
	if coreID == "" {
		return "", fmt.Errorf("ANT_FINGERPRINT_CORE_REQUIRED: managed shop requires a configured fingerprint core")
	}

	core, ok := s.browserMgr.GetCore(coreID)
	if !ok {
		return "", fmt.Errorf("ANT_CORE_NOT_FOUND: managed shop core %s is not registered", coreID)
	}

	corePath, err := s.browserMgr.ResolveCoreExecutable(core)
	if err != nil {
		return "", fmt.Errorf("ANT_CORE_UNAVAILABLE: resolve core executable %s: %w", coreID, err)
	}

	return corePath, nil
}
