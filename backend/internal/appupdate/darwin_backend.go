package appupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	darwinProcessExitTimeout  = 20 * time.Second
	darwinProcessPollInterval = 250 * time.Millisecond
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

	resolvedInstall := install
	if resolved, ok := darwinResolveExistingPrefix(install); ok {
		resolvedInstall = resolved
	}
	if darwinProtectedApplicationInstallRoot(install) || darwinProtectedApplicationInstallRoot(resolvedInstall) {
		return ErrUnsupportedInstall
	}
	if pathInsideRootDarwin(layout.StateRoot, install) {
		return fmt.Errorf("%w: app update state root is inside app bundle", ErrUnsupportedInstall)
	}

	linkInfo, err := os.Lstat(install)
	if err != nil {
		return err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return ErrUnsupportedInstall
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

func darwinProtectedApplicationInstallRoot(path string) bool {
	slash := strings.ToLower(filepath.ToSlash(filepath.Clean(strings.TrimSpace(path))))
	return strings.HasPrefix(slash, "/applications/") ||
		strings.HasPrefix(slash, "/system/applications/")
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
	if err := copyFileMode(exe, runner, 0o700); err != nil {
		return err
	}
	return os.Chmod(runner, 0o700)
}

func (b DarwinBackend) SpawnApplyRunner(planPath string) error {
	plan, err := ReadPlan(planPath)
	if err != nil {
		return err
	}
	exe := darwinRunnerPath(plan)
	info, err := os.Stat(exe)
	if err != nil {
		return fmt.Errorf("darwin update runner is not prepared: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("darwin update runner is not a file: %s", exe)
	}
	if pathInsideRootDarwin(exe, plan.InstallRoot) {
		return fmt.Errorf("darwin update runner must be outside app bundle: %s", exe)
	}

	cmd := exec.Command(exe, "--apply-update", planPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func (b DarwinBackend) RunApply(planPath string) error {
	plan, err := ReadPlan(planPath)
	if err != nil {
		return err
	}
	layout := NewLayout(plan.InstallRoot, plan.StateRoot)
	if err := b.validateApplyPlan(plan); err != nil {
		return err
	}
	if plan.WaitForProcessID > 0 && !waitForDarwinProcessExit(plan.WaitForProcessID, darwinProcessExitTimeout) {
		err := fmt.Errorf("app process did not exit before apply timeout: pid %d", plan.WaitForProcessID)
		_ = WriteState(layout, darwinStateFromPlan(plan, planPath, PersistentStatusFailedManualRepair, "APP-UPDATE-PROCESS-STILL-RUNNING", err))
		return err
	}
	if err := WriteState(layout, darwinStateFromPlan(plan, planPath, PersistentStatusApplying, "", nil)); err != nil {
		return err
	}
	if err := b.backupInstall(plan); err != nil {
		return b.writeManualRepair(layout, plan, planPath, "APP-UPDATE-BACKUP-FAILED-MANUAL-REPAIR", err)
	}
	if err := b.replaceInstall(plan); err != nil {
		if rollbackErr := b.rollbackInstall(plan); rollbackErr != nil {
			return b.writeManualRepair(layout, plan, planPath, "APP-UPDATE-ROLLBACK-FAILED-MANUAL-REPAIR", rollbackErr)
		}
		_ = WriteState(layout, darwinStateFromPlan(plan, planPath, PersistentStatusRolledBack, "APP-UPDATE-APPLY-FAILED-ROLLED-BACK", err))
		return err
	}
	if err := WriteState(layout, darwinStateFromPlan(plan, planPath, PersistentStatusVerifying, "", nil)); err != nil {
		return err
	}
	return b.launchPostUpdateCheck(plan, planPath)
}

func (b DarwinBackend) validateApplyPlan(plan ApplyPlan) error {
	if err := b.ValidateInstallMode(NewLayout(plan.InstallRoot, plan.StateRoot)); err != nil {
		return err
	}
	if strings.TrimSpace(plan.StagedPath) == "" {
		return fmt.Errorf("darwin staged payload path is required")
	}
	if strings.TrimSpace(plan.BackupPath) == "" {
		return fmt.Errorf("darwin backup path is required")
	}
	if pathInsideRootDarwin(plan.StagedPath, plan.InstallRoot) {
		return fmt.Errorf("darwin staged payload must be outside app bundle: %s", plan.StagedPath)
	}
	if pathInsideRootDarwin(plan.BackupPath, plan.InstallRoot) {
		return fmt.Errorf("darwin backup path must be outside app bundle: %s", plan.BackupPath)
	}
	return nil
}

func (b DarwinBackend) PostUpdateCheck(planPath string) error {
	plan, err := ReadPlan(planPath)
	if err != nil {
		return err
	}
	layout := NewLayout(plan.InstallRoot, plan.StateRoot)
	if currentVersion := strings.TrimSpace(b.CurrentAppVersion); currentVersion != "" && currentVersion != strings.TrimSpace(plan.NewAppVersion) {
		err := fmt.Errorf("post-update version mismatch: expected %s, got %s", plan.NewAppVersion, currentVersion)
		state := darwinStateFromPlan(plan, planPath, PersistentStatusFailedManualRepair, "APP-UPDATE-POST-CHECK-VERSION-MISMATCH", err)
		state.LocalAppVersion = currentVersion
		_ = WriteState(layout, state)
		return err
	}
	if err := validateDarwinInstalledBundle(plan.InstallRoot); err != nil {
		_ = WriteState(layout, darwinStateFromPlan(plan, planPath, PersistentStatusFailedManualRepair, "APP-UPDATE-POST-CHECK-BUNDLE-INVALID", err))
		return err
	}
	state := darwinStateFromPlan(plan, planPath, PersistentStatusSucceeded, "", nil)
	state.LocalAppVersion = plan.NewAppVersion
	if err := WriteState(layout, state); err != nil {
		return err
	}
	if b.SuppressRelaunch {
		return nil
	}
	return b.launchApplication(plan)
}

func (b DarwinBackend) backupInstall(plan ApplyPlan) error {
	if err := os.RemoveAll(plan.BackupPath); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plan.BackupPath), 0o700); err != nil {
		return err
	}
	return copyDir(plan.InstallRoot, filepath.Join(plan.BackupPath, filepath.Base(plan.InstallRoot)))
}

func (b DarwinBackend) replaceInstall(plan ApplyPlan) error {
	stagedApp := filepath.Join(plan.StagedPath, "Ant Browser.app")
	if err := ValidateStagedPayload(plan.Target, plan.StagedPath); err != nil {
		return err
	}
	if err := os.RemoveAll(plan.InstallRoot); err != nil {
		return err
	}
	return copyDir(stagedApp, plan.InstallRoot)
}

func (b DarwinBackend) rollbackInstall(plan ApplyPlan) error {
	backupApp := filepath.Join(plan.BackupPath, filepath.Base(plan.InstallRoot))
	if err := os.RemoveAll(plan.InstallRoot); err != nil {
		return err
	}
	return copyDir(backupApp, plan.InstallRoot)
}

func (b DarwinBackend) writeManualRepair(layout Layout, plan ApplyPlan, planPath, code string, err error) error {
	_ = WriteState(layout, darwinStateFromPlan(plan, planPath, PersistentStatusFailedManualRepair, code, err))
	return err
}

func (b DarwinBackend) launchPostUpdateCheck(plan ApplyPlan, planPath string) error {
	cmd := exec.Command(darwinAppExecutablePath(plan.InstallRoot), "--post-update-check", planPath)
	return startDetachedDarwinCommand(cmd)
}

func (b DarwinBackend) launchApplication(plan ApplyPlan) error {
	cmd := exec.Command(darwinAppExecutablePath(plan.InstallRoot))
	return startDetachedDarwinCommand(cmd)
}

func darwinAppExecutablePath(appRoot string) string {
	return filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome")
}

var startDetachedDarwinCommand = func(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func darwinStateFromPlan(plan ApplyPlan, planPath string, status PersistentStatus, code string, err error) PersistentState {
	state := PersistentState{
		Status:           status,
		LocalAppVersion:  plan.OldAppVersion,
		RemoteAppVersion: plan.NewAppVersion,
		ManifestSource:   plan.ManifestSource,
		ManifestURL:      plan.ManifestURL,
		PayloadURL:       plan.PayloadURL,
		Target:           plan.Target,
		PlanPath:         planPath,
		BackupPath:       plan.BackupPath,
	}
	if err != nil {
		state.LastError = ErrorInfo{Code: code, Message: err.Error()}
	}
	return state
}

func validateDarwinInstalledBundle(appRoot string) error {
	root := filepath.Dir(appRoot)
	appName := filepath.Base(appRoot)
	if err := requireDirectoryNoSymlink(root, appName, appName); err != nil {
		return err
	}
	for _, rel := range []string{
		filepath.Join(appName, "Contents", "Info.plist"),
		filepath.Join(appName, "Contents", "MacOS", "publish", "runtime-manifest.json"),
	} {
		if err := requireRegularFileNoSymlink(root, rel, rel); err != nil {
			return err
		}
	}
	for _, rel := range []string{
		filepath.Join(appName, "Contents", "MacOS", "ant-chrome"),
		filepath.Join(appName, "Contents", "MacOS", "bin", "xray"),
		filepath.Join(appName, "Contents", "MacOS", "bin", "sing-box"),
	} {
		if err := requireExecutableNoSymlink(root, rel, rel); err != nil {
			return err
		}
	}
	return rejectDarwinInstalledMutableUserData(appRoot)
}

func waitForDarwinProcessExit(pid int, timeout time.Duration) bool {
	if runtime.GOOS != "darwin" || pid <= 0 {
		return true
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isDarwinProcessRunning(pid) {
			return true
		}
		time.Sleep(darwinProcessPollInterval)
	}
	return !isDarwinProcessRunning(pid)
}

func isDarwinProcessRunning(pid int) bool {
	cmd := exec.Command("kill", "-0", fmt.Sprintf("%d", pid))
	return cmd.Run() == nil
}

func rejectDarwinInstalledMutableUserData(appRoot string) error {
	return filepath.Walk(appRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(appRoot, path)
		if err != nil {
			return err
		}
		clean := strings.ToLower(filepath.ToSlash(rel))
		base := strings.ToLower(info.Name())
		if clean == "contents/macos/data" || strings.HasPrefix(clean, "contents/macos/data/") {
			return fmt.Errorf("staged payload contains mutable user data: %s", rel)
		}
		if clean == "user data" || strings.HasPrefix(clean, "user data/") || strings.Contains(clean, "/user data/") {
			return fmt.Errorf("staged payload contains mutable browser profile data: %s", rel)
		}
		if strings.HasSuffix(base, ".db") || strings.HasSuffix(base, ".sqlite") || strings.HasSuffix(base, ".sqlite3") {
			return fmt.Errorf("staged payload contains mutable database file: %s", rel)
		}
		return nil
	})
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

	pathAbs, pathOK := darwinResolveExistingPrefix(path)
	rootAbs, rootOK := darwinResolveExistingPrefix(root)
	if !pathOK || !rootOK {
		return false
	}
	return pathInsideRootLexicalDarwin(pathAbs, rootAbs)
}

func darwinResolveExistingPrefix(path string) (string, bool) {
	abs, ok := absCleanDarwin(path)
	if !ok {
		return "", false
	}
	if resolved, ok := evalSymlinksCleanDarwin(abs); ok {
		return resolved, true
	}

	for ancestor := filepath.Dir(abs); ancestor != abs; ancestor = filepath.Dir(ancestor) {
		resolvedAncestor, ok := evalSymlinksCleanDarwin(ancestor)
		if ok {
			suffix, err := filepath.Rel(ancestor, abs)
			if err != nil || suffix == "." {
				return resolvedAncestor, true
			}
			return filepath.Clean(filepath.Join(resolvedAncestor, suffix)), true
		}
		next := filepath.Dir(ancestor)
		if next == ancestor {
			break
		}
	}
	return abs, true
}

func evalSymlinksCleanDarwin(path string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return "", false
	}
	return absCleanDarwin(resolved)
}

func absCleanDarwin(path string) (string, bool) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", false
	}
	return filepath.Clean(abs), true
}

func pathInsideRootLexicalDarwin(path, root string) bool {
	path = strings.ToLower(filepath.Clean(path))
	root = strings.ToLower(filepath.Clean(root))
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
