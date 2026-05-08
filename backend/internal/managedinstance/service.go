package managedinstance

import (
	"ant-chrome/backend/internal/browser"
	"fmt"
	"strings"
)

type Service struct {
	browserMgr *browser.Manager
}

func NewService(deps Dependencies) (*Service, error) {
	if deps.BrowserMgr == nil {
		return nil, fmt.Errorf("managed instance service requires browser manager")
	}
	return &Service{browserMgr: deps.BrowserMgr}, nil
}

func (s *Service) ensureManagedProfileCore(profile *browser.Profile) error {
	corePath, err := s.ResolveManagedCore(profile)
	if err != nil {
		return err
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
