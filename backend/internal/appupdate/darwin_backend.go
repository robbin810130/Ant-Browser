package appupdate

import "fmt"

type DarwinBackend struct {
	CurrentExePath    string
	CurrentAppVersion string
	ProcessID         int
	SuppressRelaunch  bool
	target            string
}

func (b DarwinBackend) Target() string {
	if b.target != "" {
		return b.target
	}
	return "darwin-arm64"
}

func (b DarwinBackend) ValidateInstallMode(Layout) error {
	return fmt.Errorf("darwin app update backend is not implemented")
}

func (b DarwinBackend) PrepareApply(ApplyPlan) error {
	return fmt.Errorf("darwin app update backend is not implemented")
}

func (b DarwinBackend) SpawnApplyRunner(string) error {
	return fmt.Errorf("darwin app update backend is not implemented")
}

func (b DarwinBackend) RunApply(string) error {
	return fmt.Errorf("darwin app update backend is not implemented")
}

func (b DarwinBackend) PostUpdateCheck(string) error {
	return fmt.Errorf("darwin app update backend is not implemented")
}
