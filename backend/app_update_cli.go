package backend

import (
	"fmt"

	"ant-chrome/backend/internal/appupdate"
)

func RunAppUpdateCLI(mode, planPath, appVersion string) error {
	backend := appupdate.WindowsBackend{CurrentAppVersion: appVersion}
	switch mode {
	case "apply":
		return backend.RunApply(planPath)
	case "post-check":
		return backend.PostUpdateCheck(planPath)
	default:
		return fmt.Errorf("unsupported app update cli mode: %s", mode)
	}
}
