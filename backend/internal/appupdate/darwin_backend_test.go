package appupdate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDarwinBackendValidateInstallModeAcceptsUserWritableAppBundle(t *testing.T) {
	home := t.TempDir()
	applications := filepath.Join(home, "Applications")
	appRoot := writeFakeDarwinBundle(t, applications)
	layout := NewLayout(appRoot, filepath.Join(home, "Library", "Application Support", "ant-browser"))

	if err := (DarwinBackend{}).ValidateInstallMode(layout); err != nil {
		t.Fatalf("ValidateInstallMode returned error: %v", err)
	}
}

func TestDarwinBackendValidateInstallModeRejectsApplicationsInstall(t *testing.T) {
	layout := NewLayout("/Applications/Ant Browser.app", filepath.Join(t.TempDir(), "state"))
	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected /Applications install to be rejected")
	}
}

func TestDarwinBackendValidateInstallModeRejectsSystemApplicationsInstall(t *testing.T) {
	layout := NewLayout("/System/Applications/Ant Browser.app", filepath.Join(t.TempDir(), "state"))
	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected /System/Applications install to be rejected")
	}
}

func TestDarwinBackendValidateInstallModeRejectsNonAppRoot(t *testing.T) {
	layout := NewLayout(filepath.Join(t.TempDir(), "Ant Browser"), filepath.Join(t.TempDir(), "state"))
	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected non-.app install root to be rejected")
	}
}

func TestDarwinBackendValidateInstallModeRejectsStateRootInsideAppBundle(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	layout := NewLayout(appRoot, filepath.Join(appRoot, "Contents", "MacOS", "data"))
	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected state root inside app bundle to be rejected")
	}
}

func TestDarwinBackendValidateInstallModeRejectsStateRootCaseVariantInsideAppBundle(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	caseVariantAppRoot := filepath.Join(filepath.Dir(appRoot), strings.ToUpper(filepath.Base(appRoot)))
	layout := NewLayout(appRoot, filepath.Join(caseVariantAppRoot, "Contents", "MacOS", "data"))
	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected case-variant state root inside app bundle to be rejected")
	}
}

func TestDarwinBackendValidateInstallModeRejectsStateRootSymlinkInsideAppBundle(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	stateLink := filepath.Join(root, "state-link")
	if err := os.Symlink(filepath.Join(appRoot, "Contents", "MacOS"), stateLink); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		t.Fatalf("create state symlink: %v", err)
	}
	layout := NewLayout(appRoot, stateLink)
	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected symlinked state root inside app bundle to be rejected")
	}
}

func TestDarwinBackendPrepareApplyCopiesRunnerOutsideAppBundle(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	stagedRoot := filepath.Join(root, "staged")
	writeFakeDarwinBundle(t, stagedRoot)
	currentExe := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome")
	runnerPath := filepath.Join(stateRoot, "app-update", "runner", "darwin-test", "ant-chrome-update-runner")
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		StagedPath:     stagedRoot,
		CurrentExePath: currentExe,
		RunnerPath:     runnerPath,
	}

	if err := (DarwinBackend{}).PrepareApply(plan); err != nil {
		t.Fatalf("PrepareApply returned error: %v", err)
	}
	data, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatalf("read runner: %v", err)
	}
	if string(data) != "#!/bin/sh\n" {
		t.Fatalf("unexpected runner content: %q", string(data))
	}
	info, err := os.Stat(runnerPath)
	if err != nil {
		t.Fatalf("stat runner: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("unexpected runner mode: got %v want %v", info.Mode().Perm(), os.FileMode(0o700))
	}
	rel, err := filepath.Rel(appRoot, runnerPath)
	if err != nil {
		t.Fatalf("runner relative path: %v", err)
	}
	if rel == "." || (!filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		t.Fatalf("runner must not live inside app bundle: %s", runnerPath)
	}
}

func TestDarwinBackendPrepareApplyMakesExistingRunnerExecutable(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	stagedRoot := filepath.Join(root, "staged")
	writeFakeDarwinBundle(t, stagedRoot)
	currentExe := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome")
	runnerPath := filepath.Join(stateRoot, "app-update", "runner", "darwin-test", "ant-chrome-update-runner")
	if err := os.MkdirAll(filepath.Dir(runnerPath), 0o700); err != nil {
		t.Fatalf("mkdir runner dir: %v", err)
	}
	if err := os.WriteFile(runnerPath, []byte("old runner"), 0o600); err != nil {
		t.Fatalf("write existing runner: %v", err)
	}
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		StagedPath:     stagedRoot,
		CurrentExePath: currentExe,
		RunnerPath:     runnerPath,
	}

	if err := (DarwinBackend{}).PrepareApply(plan); err != nil {
		t.Fatalf("PrepareApply returned error: %v", err)
	}
	info, err := os.Stat(runnerPath)
	if err != nil {
		t.Fatalf("stat runner: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("unexpected runner mode: got %v want %v", info.Mode().Perm(), os.FileMode(0o700))
	}
}

func TestDarwinBackendPrepareApplyRejectsRunnerInsideAppBundle(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	stagedRoot := filepath.Join(root, "staged")
	writeFakeDarwinBundle(t, stagedRoot)
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		StagedPath:     stagedRoot,
		CurrentExePath: filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome"),
		RunnerPath:     filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome-update-runner"),
	}

	if err := (DarwinBackend{}).PrepareApply(plan); err == nil {
		t.Fatal("expected runner inside app bundle to be rejected")
	}
}

func TestDarwinBackendPrepareApplyRejectsRunnerSymlinkInsideAppBundle(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	stagedRoot := filepath.Join(root, "staged")
	writeFakeDarwinBundle(t, stagedRoot)
	insideRunner := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome-update-runner")
	if err := os.WriteFile(insideRunner, []byte("old runner"), 0o700); err != nil {
		t.Fatalf("write inside runner: %v", err)
	}
	runnerPath := filepath.Join(stateRoot, "app-update", "runner", "darwin-test", "runner-link")
	if err := os.MkdirAll(filepath.Dir(runnerPath), 0o700); err != nil {
		t.Fatalf("mkdir runner dir: %v", err)
	}
	if err := os.Symlink(insideRunner, runnerPath); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		t.Fatalf("create runner symlink: %v", err)
	}
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		StagedPath:     stagedRoot,
		CurrentExePath: filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome"),
		RunnerPath:     runnerPath,
	}

	if err := (DarwinBackend{}).PrepareApply(plan); err == nil {
		t.Fatal("expected symlinked runner inside app bundle to be rejected")
	}
}

func TestDarwinBackendSpawnApplyRunnerRejectsMissingPreparedRunner(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	layout := NewLayout(appRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		RunnerPath:     filepath.Join(stateRoot, "app-update", "runner", "missing-runner"),
		CurrentExePath: filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome"),
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("write plan: %v", err)
	}

	if err := (DarwinBackend{}).SpawnApplyRunner(planPath); err == nil {
		t.Fatal("expected missing prepared runner to be rejected")
	}
}

func TestDarwinRunnerPathDefaultUnderRunnerRoot(t *testing.T) {
	root := t.TempDir()
	appRoot := filepath.Join(root, "Ant Browser.app")
	stateRoot := filepath.Join(root, "state")
	layout := NewLayout(appRoot, stateRoot)

	got := darwinRunnerPath(ApplyPlan{InstallRoot: appRoot, StateRoot: stateRoot})
	want := filepath.Join(layout.RunnerRoot(), "ant-chrome-update-runner")
	if got != want {
		t.Fatalf("darwinRunnerPath default = %q, want %q", got, want)
	}
}

func TestPathInsideRootDarwin(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Ant Browser.app")
	cases := []struct {
		name string
		path string
		root string
		want bool
	}{
		{name: "empty path", path: "", root: root, want: false},
		{name: "empty root", path: filepath.Join(root, "Contents"), root: "", want: false},
		{name: "same root", path: root, root: root, want: true},
		{name: "child", path: filepath.Join(root, "Contents", "MacOS"), root: root, want: true},
		{name: "sibling prefix", path: root + "-backup", root: root, want: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathInsideRootDarwin(tt.path, tt.root); got != tt.want {
				t.Fatalf("pathInsideRootDarwin(%q, %q) = %v, want %v", tt.path, tt.root, got, tt.want)
			}
		})
	}
}
