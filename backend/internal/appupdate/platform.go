package appupdate

import (
	"errors"
	"fmt"
	"strings"
)

var ErrUnsupportedInstall = errors.New("unsupported app update install location")
var ErrUnsupportedPlatform = errors.New("unsupported app update platform")

type PlatformOptions struct {
	CurrentExePath    string
	CurrentAppVersion string
	ProcessID         int
	SuppressRelaunch  bool
}

type PlatformUpdater interface {
	Target() string
	ValidateInstallMode(Layout) error
	PrepareApply(ApplyPlan) error
	SpawnApplyRunner(planPath string) error
	RunApply(planPath string) error
	PostUpdateCheck(planPath string) error
}

type UnsupportedBackend struct {
	Err error
}

func (b UnsupportedBackend) Target() string {
	return "unsupported"
}

func (b UnsupportedBackend) ValidateInstallMode(Layout) error {
	if b.Err != nil {
		return b.Err
	}
	return ErrUnsupportedPlatform
}

func (b UnsupportedBackend) PrepareApply(ApplyPlan) error {
	return b.ValidateInstallMode(Layout{})
}

func (b UnsupportedBackend) SpawnApplyRunner(string) error {
	return b.ValidateInstallMode(Layout{})
}

func (b UnsupportedBackend) RunApply(string) error {
	return b.ValidateInstallMode(Layout{})
}

func (b UnsupportedBackend) PostUpdateCheck(string) error {
	return b.ValidateInstallMode(Layout{})
}

func NewPlatformBackend(goos, goarch string, opts PlatformOptions) (PlatformUpdater, error) {
	goos = strings.ToLower(strings.TrimSpace(goos))
	goarch = strings.ToLower(strings.TrimSpace(goarch))

	switch goos + "/" + goarch {
	case "windows/amd64":
		return WindowsBackend{
			CurrentExePath:    opts.CurrentExePath,
			CurrentAppVersion: opts.CurrentAppVersion,
			ProcessID:         opts.ProcessID,
			SuppressRelaunch:  opts.SuppressRelaunch,
		}, nil
	case "darwin/arm64":
		return DarwinBackend{
			CurrentExePath:    opts.CurrentExePath,
			CurrentAppVersion: opts.CurrentAppVersion,
			ProcessID:         opts.ProcessID,
			SuppressRelaunch:  opts.SuppressRelaunch,
			target:            "darwin-arm64",
		}, nil
	case "darwin/amd64":
		return DarwinBackend{
			CurrentExePath:    opts.CurrentExePath,
			CurrentAppVersion: opts.CurrentAppVersion,
			ProcessID:         opts.ProcessID,
			SuppressRelaunch:  opts.SuppressRelaunch,
			target:            "darwin-amd64",
		}, nil
	default:
		return nil, fmt.Errorf("%w: %s/%s", ErrUnsupportedPlatform, goos, goarch)
	}
}
