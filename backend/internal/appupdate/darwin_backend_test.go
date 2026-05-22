package appupdate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestDarwinBackendValidateInstallModeRejectsSymlinkInstallRoot(t *testing.T) {
	root := t.TempDir()
	target := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	link := filepath.Join(root, "Linked Ant Browser.app")
	if err := os.Symlink(target, link); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		t.Fatalf("create install root symlink: %v", err)
	}
	layout := NewLayout(link, filepath.Join(root, "state"))

	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected symlink install root to be rejected")
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

func TestDarwinBackendValidateInstallModeRejectsStateRootWithSymlinkParentInsideAppBundle(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	stateParentLink := filepath.Join(root, "state-parent-link")
	if err := os.Symlink(filepath.Join(appRoot, "Contents", "MacOS"), stateParentLink); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		t.Fatalf("create state parent symlink: %v", err)
	}
	layout := NewLayout(appRoot, filepath.Join(stateParentLink, "state-that-does-not-exist"))

	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected state root with symlinked parent inside app bundle to be rejected")
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

func TestDarwinBackendPrepareApplyRejectsRunnerWithSymlinkParentInsideAppBundle(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	stagedRoot := filepath.Join(root, "staged")
	writeFakeDarwinBundle(t, stagedRoot)
	runnerParentLink := filepath.Join(stateRoot, "runner-parent-link")
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		t.Fatalf("mkdir state root: %v", err)
	}
	if err := os.Symlink(filepath.Join(appRoot, "Contents", "MacOS"), runnerParentLink); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		t.Fatalf("create runner parent symlink: %v", err)
	}
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		StagedPath:     stagedRoot,
		CurrentExePath: filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome"),
		RunnerPath:     filepath.Join(runnerParentLink, "runner-that-does-not-exist"),
	}

	if err := (DarwinBackend{}).PrepareApply(plan); err == nil {
		t.Fatal("expected runner with symlinked parent inside app bundle to be rejected")
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

func TestDarwinBackendSpawnApplyRunnerUsesPreparedRunner(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	runnerPath := filepath.Join(stateRoot, "app-update", "runner", "darwin-test", "ant-chrome-update-runner")
	argsPath := filepath.Join(root, "args.txt")
	fallbackMarkerPath := filepath.Join(root, "fallback-used.txt")
	currentExe := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome")
	if err := os.MkdirAll(filepath.Dir(runnerPath), 0o700); err != nil {
		t.Fatalf("mkdir runner dir: %v", err)
	}
	if err := os.WriteFile(runnerPath, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \""+argsPath+"\"\n"), 0o700); err != nil {
		t.Fatalf("write runner: %v", err)
	}
	if err := os.WriteFile(currentExe, []byte("#!/bin/sh\nprintf fallback > \""+fallbackMarkerPath+"\"\n"), 0o700); err != nil {
		t.Fatalf("write fallback executable: %v", err)
	}

	layout := NewLayout(appRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		RunnerPath:     runnerPath,
		CurrentExePath: currentExe,
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("write plan: %v", err)
	}

	backend := DarwinBackend{CurrentExePath: currentExe}
	if err := backend.SpawnApplyRunner(planPath); err != nil {
		t.Fatalf("SpawnApplyRunner returned error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(argsPath); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat args file: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for runner args file: %s", argsPath)
		}
		time.Sleep(10 * time.Millisecond)
	}
	data, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	want := "--apply-update\n" + planPath + "\n"
	if string(data) != want {
		t.Fatalf("runner args = %q, want %q", string(data), want)
	}
	if _, err := os.Stat(fallbackMarkerPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fallback executable was used, stat err = %v", err)
	}
}

func TestDarwinBackendSpawnApplyRunnerRejectsExistingRunnerInsideAppBundle(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	runnerPath := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome-update-runner")
	if err := os.WriteFile(runnerPath, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write runner: %v", err)
	}
	layout := NewLayout(appRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		RunnerPath:     runnerPath,
		CurrentExePath: filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome"),
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("write plan: %v", err)
	}

	if err := (DarwinBackend{}).SpawnApplyRunner(planPath); err == nil {
		t.Fatal("expected existing runner inside app bundle to be rejected")
	}
}

func TestDarwinBackendSpawnApplyRunnerRejectsExistingRunnerSymlinkInsideAppBundle(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	insideRunner := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome-update-runner")
	if err := os.WriteFile(insideRunner, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write inside runner: %v", err)
	}
	runnerPath := filepath.Join(stateRoot, "runner-link")
	if err := os.MkdirAll(filepath.Dir(runnerPath), 0o700); err != nil {
		t.Fatalf("mkdir runner dir: %v", err)
	}
	if err := os.Symlink(insideRunner, runnerPath); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		t.Fatalf("create runner symlink: %v", err)
	}
	layout := NewLayout(appRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		RunnerPath:     runnerPath,
		CurrentExePath: filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome"),
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("write plan: %v", err)
	}

	if err := (DarwinBackend{}).SpawnApplyRunner(planPath); err == nil {
		t.Fatal("expected existing symlinked runner inside app bundle to be rejected")
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

func TestDarwinProtectedApplicationInstallRoot(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "applications app", path: "/Applications/Ant Browser.app", want: true},
		{name: "system applications app", path: "/System/Applications/Ant Browser.app", want: true},
		{name: "case variant", path: "/applications/Ant Browser.app", want: true},
		{name: "protected target bypass", path: "/Applications/Ant Browser.app", want: true},
		{name: "user applications", path: filepath.Join(t.TempDir(), "Applications", "Ant Browser.app"), want: false},
		{name: "prefix sibling", path: "/ApplicationsBackup/Ant Browser.app", want: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := darwinProtectedApplicationInstallRoot(tt.path); got != tt.want {
				t.Fatalf("darwinProtectedApplicationInstallRoot(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestDarwinProtectedApplicationInstallRootRejectsResolvedProtectedTarget(t *testing.T) {
	raw := filepath.Join(t.TempDir(), "Applications", "Ant Browser.app")
	resolved := "/Applications/Ant Browser.app"

	if darwinProtectedApplicationInstallRoot(raw) {
		t.Fatalf("raw user install path should not be protected: %s", raw)
	}
	if !darwinProtectedApplicationInstallRoot(resolved) {
		t.Fatalf("resolved protected install path should be protected: %s", resolved)
	}
	if !(darwinProtectedApplicationInstallRoot(raw) || darwinProtectedApplicationInstallRoot(resolved)) {
		t.Fatal("expected raw/resolved protected check to reject resolved protected target")
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
