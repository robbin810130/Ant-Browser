package appupdate

import "errors"

var ErrUnsupportedInstall = errors.New("unsupported app update install location")

type PlatformUpdater interface {
	Target() string
	ValidateInstallMode(Layout) error
	PrepareApply(ApplyPlan) error
	SpawnApplyRunner(planPath string) error
	RunApply(planPath string) error
	PostUpdateCheck(planPath string) error
}
