package appupdate

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type WindowsBackend struct {
	CurrentExePath string
	ProcessID      int
}

func (b WindowsBackend) Target() string {
	return "windows-amd64"
}

func (b WindowsBackend) ValidateInstallMode(layout Layout) error {
	install := filepath.Clean(layout.InstallRoot)
	lower := strings.ToLower(filepath.ToSlash(strings.ReplaceAll(install, `\`, `/`)))
	if strings.Contains(lower, "/program files/") ||
		strings.HasSuffix(lower, "/program files") ||
		strings.Contains(lower, "/program files (x86)/") ||
		strings.HasSuffix(lower, "/program files (x86)") {
		return ErrUnsupportedInstall
	}
	if err := os.MkdirAll(install, 0o755); err != nil {
		return err
	}
	probe, err := os.CreateTemp(install, ".app-update-write-*")
	if err != nil {
		return fmt.Errorf("install root is not writable: %w", err)
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}

func (b WindowsBackend) PrepareApply(plan ApplyPlan) error {
	if err := ValidateStagedPayload(plan.Target, plan.StagedPath); err != nil {
		return err
	}
	if _, err := os.Stat(plan.InstallRoot); err != nil {
		return err
	}
	return b.prepareRunner(plan)
}

func (b WindowsBackend) SpawnApplyRunner(planPath string) error {
	plan, err := ReadPlan(planPath)
	if err != nil {
		return err
	}
	exe := runnerExePath(plan)
	if _, err := os.Stat(exe); err != nil {
		exe = strings.TrimSpace(b.CurrentExePath)
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

func (b WindowsBackend) RunApply(planPath string) error {
	plan, err := ReadPlan(planPath)
	if err != nil {
		return err
	}
	if plan.WaitForProcessID > 0 {
		waitForProcessExit(plan.WaitForProcessID, 20*time.Second)
	}

	layout := NewLayout(plan.InstallRoot, plan.StateRoot)
	if err := WriteState(layout, PersistentState{
		Status:           PersistentStatusApplying,
		LocalAppVersion:  plan.OldAppVersion,
		RemoteAppVersion: plan.NewAppVersion,
		PlanPath:         planPath,
		BackupPath:       plan.BackupPath,
	}); err != nil {
		return err
	}
	if err := b.backupInstall(plan); err != nil {
		_ = WriteState(layout, PersistentState{
			Status: PersistentStatusFailedManualRepair,
			LastError: ErrorInfo{
				Code:    "APP-UPDATE-BACKUP-FAILED-MANUAL-REPAIR",
				Message: err.Error(),
			},
		})
		return err
	}
	if err := b.replaceInstall(plan); err != nil {
		if rollbackErr := b.rollbackInstall(plan); rollbackErr != nil {
			_ = WriteState(layout, PersistentState{
				Status:     PersistentStatusFailedManualRepair,
				BackupPath: plan.BackupPath,
				LastError: ErrorInfo{
					Code:    "APP-UPDATE-ROLLBACK-FAILED-MANUAL-REPAIR",
					Message: rollbackErr.Error(),
				},
			})
			return err
		}
		_ = WriteState(layout, PersistentState{
			Status:     PersistentStatusRolledBack,
			BackupPath: plan.BackupPath,
			LastError: ErrorInfo{
				Code:    "APP-UPDATE-APPLY-FAILED-ROLLED-BACK",
				Message: err.Error(),
			},
		})
		return err
	}

	if err := WriteState(layout, PersistentState{
		Status:           PersistentStatusVerifying,
		LocalAppVersion:  plan.OldAppVersion,
		RemoteAppVersion: plan.NewAppVersion,
		PlanPath:         planPath,
		BackupPath:       plan.BackupPath,
	}); err != nil {
		return err
	}
	return b.launchPostUpdateCheck(plan, planPath)
}

func (b WindowsBackend) PostUpdateCheck(planPath string) error {
	plan, err := ReadPlan(planPath)
	if err != nil {
		return err
	}
	layout := NewLayout(plan.InstallRoot, plan.StateRoot)
	return WriteState(layout, PersistentState{
		Status:           PersistentStatusSucceeded,
		LocalAppVersion:  plan.NewAppVersion,
		RemoteAppVersion: plan.NewAppVersion,
		PlanPath:         planPath,
		BackupPath:       plan.BackupPath,
	})
}

func (b WindowsBackend) backupInstall(plan ApplyPlan) error {
	if err := os.RemoveAll(plan.BackupPath); err != nil {
		return err
	}
	return copyDir(plan.InstallRoot, plan.BackupPath)
}

func (b WindowsBackend) replaceInstall(plan ApplyPlan) error {
	entries, err := os.ReadDir(plan.InstallRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(plan.InstallRoot, entry.Name())); err != nil {
			return err
		}
	}
	return copyDir(plan.StagedPath, plan.InstallRoot)
}

func (b WindowsBackend) rollbackInstall(plan ApplyPlan) error {
	entries, err := os.ReadDir(plan.InstallRoot)
	if err == nil {
		for _, entry := range entries {
			if removeErr := os.RemoveAll(filepath.Join(plan.InstallRoot, entry.Name())); removeErr != nil {
				return removeErr
			}
		}
	}
	return copyDir(plan.BackupPath, plan.InstallRoot)
}

func (b WindowsBackend) prepareRunner(plan ApplyPlan) error {
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
	return copyFileMode(exe, runnerExePath(plan), 0o700)
}

func (b WindowsBackend) launchPostUpdateCheck(plan ApplyPlan, planPath string) error {
	exe := filepath.Join(plan.InstallRoot, "ant-chrome.exe")
	if strings.TrimSpace(plan.CurrentExePath) != "" {
		exe = filepath.Join(plan.InstallRoot, filepath.Base(plan.CurrentExePath))
	}
	cmd := exec.Command(exe, "--post-update-check", planPath)
	return cmd.Start()
}

func runnerExePath(plan ApplyPlan) string {
	return filepath.Join(NewLayout(plan.InstallRoot, plan.StateRoot).RunnerRoot(), "ant-chrome-update-runner.exe")
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, dirMode(info.Mode()))
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink copy is not supported: %s", path)
		}
		return copyFileMode(path, target, info.Mode().Perm())
	})
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	if mode == 0 {
		mode = 0o600
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func waitForProcessExit(pid int, timeout time.Duration) {
	if runtime.GOOS != "windows" || pid <= 0 {
		return
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isWindowsProcessRunning(pid) {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func isWindowsProcessRunning(pid int) bool {
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").Output()
	if err != nil {
		return false
	}
	text := strings.ToLower(string(out))
	return strings.Contains(text, fmt.Sprintf("%d", pid)) && !strings.Contains(text, "no tasks")
}
