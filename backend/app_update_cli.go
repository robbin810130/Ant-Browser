package backend

import (
	"fmt"
	goruntime "runtime"

	"ant-chrome/backend/internal/appupdate"
)

func RunAppUpdateCLI(mode, planPath, appVersion string) error {
	switch mode {
	case "apply", "post-check":
	default:
		return fmt.Errorf("unsupported app update cli mode: %s", mode)
	}

	backend, err := appupdate.NewPlatformBackend(goruntime.GOOS, goruntime.GOARCH, appupdate.PlatformOptions{
		CurrentAppVersion: appVersion,
	})
	if err != nil {
		return err
	}
	switch mode {
	case "apply":
		return backend.RunApply(planPath)
	case "post-check":
		return backend.PostUpdateCheck(planPath)
	default:
		return fmt.Errorf("unsupported app update cli mode: %s", mode)
	}
}
