package appupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

func (b DarwinBackend) ValidateInstallMode(layout Layout) error {
	install := filepath.Clean(strings.TrimSpace(layout.InstallRoot))
	if install == "." || !strings.HasSuffix(strings.ToLower(filepath.Base(install)), ".app") {
		return ErrUnsupportedInstall
	}

	slash := strings.ToLower(filepath.ToSlash(install))
	if slash == "/applications/ant browser.app" ||
		strings.HasPrefix(slash, "/applications/") ||
		strings.HasPrefix(slash, "/system/applications/") {
		return ErrUnsupportedInstall
	}
	if pathInsideRootDarwin(layout.StateRoot, install) {
		return fmt.Errorf("%w: app update state root is inside app bundle", ErrUnsupportedInstall)
	}

	info, err := os.Stat(install)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return ErrUnsupportedInstall
	}

	parent := filepath.Dir(install)
	probe, err := os.CreateTemp(parent, ".app-update-write-*")
	if err != nil {
		return fmt.Errorf("app bundle parent is not writable: %w", err)
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}

func (b DarwinBackend) PrepareApply(plan ApplyPlan) error {
	if err := ValidateStagedPayload(plan.Target, plan.StagedPath); err != nil {
		return err
	}
	if err := b.ValidateInstallMode(NewLayout(plan.InstallRoot, plan.StateRoot)); err != nil {
		return err
	}

	exe := strings.TrimSpace(plan.CurrentExePath)
	if exe == "" {
		exe = strings.TrimSpace(b.CurrentExePath)
	}
	if exe == "" {
		var err error
		exe, err = os.Executable()
		if err != nil {
			return err
		}
	}

	runner := darwinRunnerPath(plan)
	if pathInsideRootDarwin(runner, plan.InstallRoot) {
		return fmt.Errorf("darwin update runner must be outside app bundle: %s", runner)
	}
	return copyFileMode(exe, runner, 0o700)
}

func (b DarwinBackend) SpawnApplyRunner(planPath string) error {
	plan, err := ReadPlan(planPath)
	if err != nil {
		return err
	}
	exe := darwinRunnerPath(plan)
	if _, err := os.Stat(exe); err != nil {
		exe = strings.TrimSpace(plan.CurrentExePath)
		if exe == "" {
			exe = strings.TrimSpace(b.CurrentExePath)
		}
		if exe == "" {
			exe, err = os.Executable()
			if err != nil {
				return err
			}
		}
	}

	cmd := exec.Command(exe, "--apply-update", planPath)
	return cmd.Start()
}

func (b DarwinBackend) RunApply(string) error {
	return fmt.Errorf("darwin app update backend is not implemented")
}

func (b DarwinBackend) PostUpdateCheck(string) error {
	return fmt.Errorf("darwin app update backend is not implemented")
}

func darwinRunnerPath(plan ApplyPlan) string {
	if path := strings.TrimSpace(plan.RunnerPath); path != "" {
		return filepath.Clean(path)
	}
	return filepath.Join(NewLayout(plan.InstallRoot, plan.StateRoot).RunnerRoot(), "ant-chrome-update-runner")
}

func pathInsideRootDarwin(path, root string) bool {
	path = strings.TrimSpace(path)
	root = strings.TrimSpace(root)
	if path == "" || root == "" {
		return false
	}

	pathAbs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
