package managedinstance

import (
	"ant-chrome/backend/internal/browser"
	"fmt"
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
