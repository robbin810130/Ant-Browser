package managedinstance

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/workspace"
)

type Dependencies struct {
	BrowserMgr *browser.Manager
}

type OpenRequest struct {
	ShopID        string                      `json:"shopId"`
	ProfileID     string                      `json:"profileId"`
	TargetURL     string                      `json:"targetUrl"`
	LaunchContext workspace.ShopLaunchContext `json:"launchContext"`
	SessionBundle workspace.SessionBundle     `json:"sessionBundle"`
	ManagedMode   bool                        `json:"managedMode"`
	SessionReady  bool                        `json:"sessionReady"`
	PreferVisible bool                        `json:"preferVisible"`
}

type OpenResult struct {
	ProfileID  string `json:"profileId"`
	PID        int    `json:"pid"`
	DebugPort  int    `json:"debugPort"`
	CurrentURL string `json:"currentUrl"`
	PageTitle  string `json:"pageTitle"`
	Success    bool   `json:"success"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}
