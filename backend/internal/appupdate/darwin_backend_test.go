package appupdate

import (
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
