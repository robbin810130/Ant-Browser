# Cross-Platform App Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Connect macOS application self-update to the existing shared `appupdate` core while preserving the completed Windows Phase 1 updater behavior.

**Architecture:** Keep `appupdate.Manager`, manifest parsing, download, staging, state, and plan files as the shared core. Add a platform backend factory, implement `DarwinBackend` as a `.app` bundle replacer, and extend payload verification for `darwin-arm64` and `darwin-amd64`. Windows remains the known-good backend and should only change where platform selection requires it.

**Tech Stack:** Go standard library, Wails v2 CLI dispatch, Python 3 stdlib zip/hash/json tooling, Markdown runbook docs, `rtk` shell wrapper.

---

## File Structure

Create these backend files:

- `backend/internal/appupdate/darwin_backend.go`: macOS install validation, runner preparation, bundle backup, bundle replacement, rollback, launch, and post-update check.
- `backend/internal/appupdate/darwin_backend_test.go`: fake `.app` bundle tests for install validation, staged payload validation, backup/replace/rollback, and post-check state writing.
- `backend/internal/appupdate/platform_test.go`: platform backend factory mapping tests.

Modify these backend files:

- `backend/internal/appupdate/platform.go`: add `PlatformOptions`, `ErrUnsupportedPlatform`, and `NewPlatformBackend`.
- `backend/internal/appupdate/archive.go`: accept `darwin-arm64` and `darwin-amd64`; validate `.app` staged payloads and forbidden mutable user data.
- `backend/internal/appupdate/archive_test.go`: replace Phase 1 darwin rejection test with darwin acceptance and rejection cases.
- `backend/internal/appupdate/windows_backend.go`: reuse shared runner path naming only if needed; keep Windows behavior stable.
- `backend/app_update.go`: select backend via `NewPlatformBackend`.
- `backend/app_update_cli.go`: select backend via `NewPlatformBackend`; keep CLI mode behavior unchanged.
- `backend/app_update_test.go`: cover selected backend injection and existing state root behavior.

Modify release tooling and docs:

- `tools/app-update/verify-app-update-package.py`: make payload contract target-aware for Windows and macOS.
- `docs/release/windows-packaging-and-update-runbook.md`: add macOS app-update packaging and regression section.

---

### Task 1: Platform Backend Factory

**Files:**
- Modify: `backend/internal/appupdate/platform.go`
- Create: `backend/internal/appupdate/platform_test.go`

- [ ] **Step 1: Write failing platform factory tests**

Create `backend/internal/appupdate/platform_test.go`:

```go
package appupdate

import (
	"errors"
	"testing"
)

func TestNewPlatformBackendMapsWindowsAMD64(t *testing.T) {
	backend, err := NewPlatformBackend("windows", "amd64", PlatformOptions{ProcessID: 123})
	if err != nil {
		t.Fatalf("NewPlatformBackend returned error: %v", err)
	}
	if backend.Target() != "windows-amd64" {
		t.Fatalf("unexpected target: %s", backend.Target())
	}
	if _, ok := backend.(WindowsBackend); !ok {
		t.Fatalf("expected WindowsBackend, got %T", backend)
	}
}

func TestNewPlatformBackendMapsDarwinARM64(t *testing.T) {
	backend, err := NewPlatformBackend("darwin", "arm64", PlatformOptions{CurrentAppVersion: "1.2.0"})
	if err != nil {
		t.Fatalf("NewPlatformBackend returned error: %v", err)
	}
	if backend.Target() != "darwin-arm64" {
		t.Fatalf("unexpected target: %s", backend.Target())
	}
	if _, ok := backend.(DarwinBackend); !ok {
		t.Fatalf("expected DarwinBackend, got %T", backend)
	}
}

func TestNewPlatformBackendMapsDarwinAMD64(t *testing.T) {
	backend, err := NewPlatformBackend("darwin", "amd64", PlatformOptions{})
	if err != nil {
		t.Fatalf("NewPlatformBackend returned error: %v", err)
	}
	if backend.Target() != "darwin-amd64" {
		t.Fatalf("unexpected target: %s", backend.Target())
	}
}

func TestNewPlatformBackendRejectsUnsupportedPlatform(t *testing.T) {
	_, err := NewPlatformBackend("linux", "amd64", PlatformOptions{})
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("expected ErrUnsupportedPlatform, got %v", err)
	}
}
```

- [ ] **Step 2: Run the new tests and verify they fail**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestNewPlatformBackend' -count=1
```

Expected: FAIL with undefined `NewPlatformBackend`, `PlatformOptions`, `DarwinBackend`, and `ErrUnsupportedPlatform`.

- [ ] **Step 3: Implement the platform factory**

Modify `backend/internal/appupdate/platform.go`:

```go
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

func pathInsideRoot(path, root string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	root = filepath.Clean(strings.TrimSpace(root))
	if path == "" || root == "" {
		return false
	}
	if strings.EqualFold(path, root) {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
```

- [ ] **Step 4: Add a minimal DarwinBackend skeleton for factory compilation**

Create `backend/internal/appupdate/darwin_backend.go`:

```go
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
```

Add `path/filepath` to the `backend/internal/appupdate/platform.go` imports.

- [ ] **Step 5: Run the factory tests and verify they pass**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestNewPlatformBackend' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 1**

Run:

```bash
rtk git add backend/internal/appupdate/platform.go backend/internal/appupdate/platform_test.go backend/internal/appupdate/darwin_backend.go
rtk git commit -m "Add app update platform backend factory"
```

Expected: commit succeeds.

---

### Task 2: Darwin Staged Payload Validation

**Files:**
- Modify: `backend/internal/appupdate/archive.go`
- Modify: `backend/internal/appupdate/archive_test.go`

- [ ] **Step 1: Replace the Phase 1 darwin rejection test with darwin payload tests**

Modify `backend/internal/appupdate/archive_test.go` by deleting `TestValidateStagedPayloadRejectsMacTargetsForPhaseOne` and adding:

```go
func writeFakeDarwinBundle(t *testing.T, root string) string {
	t.Helper()
	appRoot := filepath.Join(root, "Ant Browser.app")
	macos := filepath.Join(appRoot, "Contents", "MacOS")
	if err := os.MkdirAll(filepath.Join(macos, "publish"), 0o755); err != nil {
		t.Fatalf("mkdir publish: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(macos, "bin"), 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appRoot, "Contents", "Info.plist"), []byte(`<plist></plist>`), 0o600); err != nil {
		t.Fatalf("write Info.plist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macos, "ant-chrome"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write ant-chrome: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macos, "publish", "runtime-manifest.json"), []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write runtime manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macos, "bin", "xray"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write xray: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macos, "bin", "sing-box"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write sing-box: %v", err)
	}
	return appRoot
}

func TestValidateStagedPayloadAcceptsDarwinBundle(t *testing.T) {
	root := t.TempDir()
	writeFakeDarwinBundle(t, root)
	if err := ValidateStagedPayload("darwin-arm64", root); err != nil {
		t.Fatalf("ValidateStagedPayload returned error: %v", err)
	}
}

func TestValidateStagedPayloadRejectsDarwinMissingInfoPlist(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	if err := os.Remove(filepath.Join(appRoot, "Contents", "Info.plist")); err != nil {
		t.Fatalf("remove Info.plist: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err == nil {
		t.Fatal("expected missing Info.plist error")
	}
}

func TestValidateStagedPayloadRejectsDarwinNonExecutableMainBinary(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	mainBinary := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome")
	if err := os.Chmod(mainBinary, 0o600); err != nil {
		t.Fatalf("chmod ant-chrome: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err == nil {
		t.Fatal("expected non-executable ant-chrome error")
	}
}

func TestValidateStagedPayloadRejectsDarwinMutableUserData(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	dataDir := filepath.Join(appRoot, "Contents", "MacOS", "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "app.sqlite"), []byte("db"), 0o600); err != nil {
		t.Fatalf("write sqlite: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err == nil {
		t.Fatal("expected mutable user data rejection")
	}
}
```

- [ ] **Step 2: Run darwin payload tests and verify they fail**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestValidateStagedPayload.*Darwin' -count=1
```

Expected: FAIL because `ValidateStagedPayload` still rejects darwin targets.

- [ ] **Step 3: Implement darwin staged payload validation**

Modify `backend/internal/appupdate/archive.go` by replacing the darwin rejection case in `ValidateStagedPayload` and adding helpers:

```go
func ValidateStagedPayload(target, stagedRoot string) error {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "windows-amd64":
		required := []string{
			"ant-chrome.exe",
			filepath.Join("publish", "runtime-manifest.json"),
		}
		for _, rel := range required {
			info, err := os.Stat(filepath.Join(stagedRoot, rel))
			if err != nil {
				return fmt.Errorf("staged payload missing required file: %s", rel)
			}
			if info.IsDir() {
				return fmt.Errorf("staged payload required path is a directory: %s", rel)
			}
		}
		return rejectMutableUserData(stagedRoot)
	case "darwin-amd64", "darwin-arm64":
		return validateDarwinStagedPayload(stagedRoot)
	default:
		return fmt.Errorf("unsupported app update target: %s", target)
	}
}

func validateDarwinStagedPayload(stagedRoot string) error {
	appRoot := filepath.Join(stagedRoot, "Ant Browser.app")
	if info, err := os.Stat(appRoot); err != nil {
		return fmt.Errorf("staged payload missing app bundle: Ant Browser.app")
	} else if !info.IsDir() {
		return fmt.Errorf("staged payload app bundle is not a directory: Ant Browser.app")
	}

	requiredFiles := []string{
		filepath.Join("Ant Browser.app", "Contents", "Info.plist"),
		filepath.Join("Ant Browser.app", "Contents", "MacOS", "publish", "runtime-manifest.json"),
	}
	for _, rel := range requiredFiles {
		info, err := os.Stat(filepath.Join(stagedRoot, rel))
		if err != nil {
			return fmt.Errorf("staged payload missing required file: %s", rel)
		}
		if info.IsDir() {
			return fmt.Errorf("staged payload required path is a directory: %s", rel)
		}
	}

	executables := []string{
		filepath.Join("Ant Browser.app", "Contents", "MacOS", "ant-chrome"),
		filepath.Join("Ant Browser.app", "Contents", "MacOS", "bin", "xray"),
		filepath.Join("Ant Browser.app", "Contents", "MacOS", "bin", "sing-box"),
	}
	for _, rel := range executables {
		if err := requireExecutable(filepath.Join(stagedRoot, rel), rel); err != nil {
			return err
		}
	}
	return rejectMutableUserData(stagedRoot)
}

func requireExecutable(path, rel string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("staged payload missing required executable: %s", rel)
	}
	if info.IsDir() {
		return fmt.Errorf("staged payload required executable is a directory: %s", rel)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("staged payload required executable is not executable: %s", rel)
	}
	return nil
}

func rejectMutableUserData(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		clean := strings.ToLower(filepath.ToSlash(rel))
		base := strings.ToLower(info.Name())
		if clean == "data" || strings.HasPrefix(clean, "data/") || strings.Contains(clean, "/data/") {
			return fmt.Errorf("staged payload contains mutable user data: %s", rel)
		}
		if strings.HasSuffix(base, ".db") || strings.HasSuffix(base, ".sqlite") || strings.HasSuffix(base, ".sqlite3") {
			return fmt.Errorf("staged payload contains mutable database file: %s", rel)
		}
		return nil
	})
}
```

- [ ] **Step 4: Run archive tests**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestExtractFullPayload|TestValidateStaged' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 2**

Run:

```bash
rtk git add backend/internal/appupdate/archive.go backend/internal/appupdate/archive_test.go
rtk git commit -m "Validate darwin app update payloads"
```

Expected: commit succeeds.

---

### Task 3: Darwin Install Mode And Runner Preparation

**Files:**
- Modify: `backend/internal/appupdate/darwin_backend.go`
- Create or modify: `backend/internal/appupdate/darwin_backend_test.go`

- [ ] **Step 1: Write failing Darwin install and runner tests**

Create `backend/internal/appupdate/darwin_backend_test.go`:

```go
package appupdate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDarwinInstallBundle(t *testing.T, appRoot string) {
	t.Helper()
	macos := filepath.Join(appRoot, "Contents", "MacOS")
	if err := os.MkdirAll(filepath.Join(macos, "publish"), 0o755); err != nil {
		t.Fatalf("mkdir publish: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(macos, "bin"), 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appRoot, "Contents", "Info.plist"), []byte(`<plist></plist>`), 0o600); err != nil {
		t.Fatalf("write Info.plist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macos, "ant-chrome"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write ant-chrome: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macos, "publish", "runtime-manifest.json"), []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write runtime manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macos, "bin", "xray"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write xray: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macos, "bin", "sing-box"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write sing-box: %v", err)
	}
}

func TestDarwinBackendValidateInstallModeAcceptsUserWritableAppBundle(t *testing.T) {
	home := t.TempDir()
	appRoot := filepath.Join(home, "Applications", "Ant Browser.app")
	writeDarwinInstallBundle(t, appRoot)
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

func TestDarwinBackendValidateInstallModeRejectsNonAppRoot(t *testing.T) {
	layout := NewLayout(filepath.Join(t.TempDir(), "Ant Browser"), filepath.Join(t.TempDir(), "state"))
	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected non-.app install root to be rejected")
	}
}

func TestDarwinBackendPrepareApplyCopiesRunnerOutsideAppBundle(t *testing.T) {
	root := t.TempDir()
	appRoot := filepath.Join(root, "Applications", "Ant Browser.app")
	writeDarwinInstallBundle(t, appRoot)
	stateRoot := filepath.Join(root, "state")
	stagedRoot := filepath.Join(root, "staged")
	writeDarwinInstallBundle(t, filepath.Join(stagedRoot, "Ant Browser.app"))
	currentExe := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome")
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		StagedPath:     stagedRoot,
		CurrentExePath: currentExe,
		RunnerPath:     filepath.Join(stateRoot, "app-update", "runner", "darwin-test", "ant-chrome-update-runner"),
	}

	if err := (DarwinBackend{}).PrepareApply(plan); err != nil {
		t.Fatalf("PrepareApply returned error: %v", err)
	}
	data, err := os.ReadFile(plan.RunnerPath)
	if err != nil {
		t.Fatalf("read runner: %v", err)
	}
	if string(data) != "#!/bin/sh\n" {
		t.Fatalf("unexpected runner content: %q", string(data))
	}
	if strings.HasPrefix(filepath.Clean(plan.RunnerPath), filepath.Clean(appRoot)) {
		t.Fatalf("runner must not live inside app bundle: %s", plan.RunnerPath)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestDarwinBackendValidateInstallMode|TestDarwinBackendPrepareApply' -count=1
```

Expected: FAIL because `DarwinBackend` methods still return not implemented.

- [ ] **Step 3: Implement install validation and runner preparation**

Modify `backend/internal/appupdate/darwin_backend.go`:

```go
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
	if !strings.HasSuffix(strings.ToLower(filepath.Base(install)), ".app") {
		return ErrUnsupportedInstall
	}
	slash := filepath.ToSlash(install)
	if slash == "/Applications/Ant Browser.app" || strings.HasPrefix(slash, "/Applications/") || strings.HasPrefix(slash, "/System/Applications/") {
		return ErrUnsupportedInstall
	}
	if pathInsideRoot(layout.StateRoot, install) {
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
	if pathInsideRoot(runner, plan.InstallRoot) {
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

func darwinRunnerPath(plan ApplyPlan) string {
	if path := strings.TrimSpace(plan.RunnerPath); path != "" {
		return filepath.Clean(path)
	}
	return filepath.Join(NewLayout(plan.InstallRoot, plan.StateRoot).RunnerRoot(), "ant-chrome-update-runner")
}
```

Keep temporary stubs at the bottom until later tasks replace them:

```go
func (b DarwinBackend) RunApply(string) error {
	return fmt.Errorf("darwin app update apply is not implemented")
}

func (b DarwinBackend) PostUpdateCheck(string) error {
	return fmt.Errorf("darwin app update post-check is not implemented")
}
```

- [ ] **Step 4: Run tests**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestDarwinBackendValidateInstallMode|TestDarwinBackendPrepareApply|TestNewPlatformBackend' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 3**

Run:

```bash
rtk git add backend/internal/appupdate/darwin_backend.go backend/internal/appupdate/darwin_backend_test.go
rtk git commit -m "Prepare darwin app update runner"
```

Expected: commit succeeds.

---

### Task 4: Darwin Bundle Replace, Rollback, And Post-Check

**Files:**
- Modify: `backend/internal/appupdate/darwin_backend.go`
- Modify: `backend/internal/appupdate/darwin_backend_test.go`

- [ ] **Step 1: Add failing bundle replacement and post-check tests**

Append to `backend/internal/appupdate/darwin_backend_test.go`:

```go
func TestDarwinBackendBackupReplaceAndRollback(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, "Applications", "Ant Browser.app")
	stateRoot := filepath.Join(root, "state")
	writeDarwinInstallBundle(t, installRoot)
	if err := os.WriteFile(filepath.Join(installRoot, "Contents", "MacOS", "old-marker.txt"), []byte("old"), 0o600); err != nil {
		t.Fatalf("write old marker: %v", err)
	}

	stagedRoot := filepath.Join(root, "staged")
	stagedApp := filepath.Join(stagedRoot, "Ant Browser.app")
	writeDarwinInstallBundle(t, stagedApp)
	if err := os.WriteFile(filepath.Join(stagedApp, "Contents", "MacOS", "new-marker.txt"), []byte("new"), 0o600); err != nil {
		t.Fatalf("write new marker: %v", err)
	}

	plan := ApplyPlan{
		InstallRoot:   installRoot,
		StateRoot:     stateRoot,
		Target:        "darwin-arm64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
		StagedPath:    stagedRoot,
		BackupPath:    filepath.Join(NewLayout(installRoot, stateRoot).BackupsRoot(), "1.1.0-test"),
	}
	backend := DarwinBackend{SuppressRelaunch: true}

	if err := backend.backupInstall(plan); err != nil {
		t.Fatalf("backupInstall returned error: %v", err)
	}
	if err := backend.replaceInstall(plan); err != nil {
		t.Fatalf("replaceInstall returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "Contents", "MacOS", "new-marker.txt")); err != nil {
		t.Fatalf("expected new marker after replace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "Contents", "MacOS", "old-marker.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected old marker removed after replace, err=%v", err)
	}
	if err := backend.rollbackInstall(plan); err != nil {
		t.Fatalf("rollbackInstall returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "Contents", "MacOS", "old-marker.txt")); err != nil {
		t.Fatalf("expected old marker after rollback: %v", err)
	}
}

func TestDarwinBackendPostUpdateCheckWritesSucceededState(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, "Applications", "Ant Browser.app")
	stateRoot := filepath.Join(root, "state")
	writeDarwinInstallBundle(t, installRoot)
	layout := NewLayout(installRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:   installRoot,
		StateRoot:     stateRoot,
		Target:        "darwin-arm64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	if err := (DarwinBackend{CurrentAppVersion: "1.2.0", SuppressRelaunch: true}).PostUpdateCheck(planPath); err != nil {
		t.Fatalf("PostUpdateCheck returned error: %v", err)
	}
	state, err := ReadState(layout)
	if err != nil {
		t.Fatalf("ReadState returned error: %v", err)
	}
	if state.Status != PersistentStatusSucceeded || state.LocalAppVersion != "1.2.0" {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestDarwinBackendPostUpdateCheckRejectsVersionMismatch(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, "Applications", "Ant Browser.app")
	stateRoot := filepath.Join(root, "state")
	writeDarwinInstallBundle(t, installRoot)
	layout := NewLayout(installRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:   installRoot,
		StateRoot:     stateRoot,
		Target:        "darwin-arm64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	err = (DarwinBackend{CurrentAppVersion: "1.1.0", SuppressRelaunch: true}).PostUpdateCheck(planPath)
	if err == nil {
		t.Fatal("expected version mismatch error")
	}
	state, readErr := ReadState(layout)
	if readErr != nil {
		t.Fatalf("ReadState returned error: %v", readErr)
	}
	if state.Status != PersistentStatusFailedManualRepair || state.LastError.Code != "APP-UPDATE-POST-CHECK-VERSION-MISMATCH" {
		t.Fatalf("unexpected mismatch state: %+v", state)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestDarwinBackendBackupReplaceAndRollback|TestDarwinBackendPostUpdateCheck' -count=1
```

Expected: FAIL because bundle replacement helpers and post-check are not implemented.

- [ ] **Step 3: Implement Darwin bundle backup, replace, rollback, and post-check**

Extend `backend/internal/appupdate/darwin_backend.go`:

```go
func (b DarwinBackend) RunApply(planPath string) error {
	plan, err := ReadPlan(planPath)
	if err != nil {
		return err
	}
	layout := NewLayout(plan.InstallRoot, plan.StateRoot)
	if plan.WaitForProcessID > 0 && !waitForProcessExit(plan.WaitForProcessID, 20*time.Second) {
		err := fmt.Errorf("app process did not exit before apply timeout: pid %d", plan.WaitForProcessID)
		_ = WriteState(layout, PersistentState{
			Status:           PersistentStatusFailedManualRepair,
			LocalAppVersion:  plan.OldAppVersion,
			RemoteAppVersion: plan.NewAppVersion,
			PlanPath:         planPath,
			BackupPath:       plan.BackupPath,
			LastError: ErrorInfo{
				Code:    "APP-UPDATE-PROCESS-STILL-RUNNING",
				Message: err.Error(),
			},
		})
		return err
	}
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
		return b.writeManualRepair(layout, "APP-UPDATE-BACKUP-FAILED-MANUAL-REPAIR", err)
	}
	if err := b.replaceInstall(plan); err != nil {
		if rollbackErr := b.rollbackInstall(plan); rollbackErr != nil {
			return b.writeManualRepair(layout, "APP-UPDATE-ROLLBACK-FAILED-MANUAL-REPAIR", rollbackErr)
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

func (b DarwinBackend) PostUpdateCheck(planPath string) error {
	plan, err := ReadPlan(planPath)
	if err != nil {
		return err
	}
	layout := NewLayout(plan.InstallRoot, plan.StateRoot)
	if currentVersion := strings.TrimSpace(b.CurrentAppVersion); currentVersion != "" && currentVersion != strings.TrimSpace(plan.NewAppVersion) {
		err := fmt.Errorf("post-update version mismatch: expected %s, got %s", plan.NewAppVersion, currentVersion)
		_ = WriteState(layout, PersistentState{
			Status:           PersistentStatusFailedManualRepair,
			LocalAppVersion:  currentVersion,
			RemoteAppVersion: plan.NewAppVersion,
			PlanPath:         planPath,
			BackupPath:       plan.BackupPath,
			LastError: ErrorInfo{
				Code:    "APP-UPDATE-POST-CHECK-VERSION-MISMATCH",
				Message: err.Error(),
			},
		})
		return err
	}
	if err := validateDarwinStagedPayload(filepath.Dir(plan.InstallRoot)); err != nil {
		_ = WriteState(layout, PersistentState{
			Status:           PersistentStatusFailedManualRepair,
			LocalAppVersion:  plan.OldAppVersion,
			RemoteAppVersion: plan.NewAppVersion,
			PlanPath:         planPath,
			BackupPath:       plan.BackupPath,
			LastError: ErrorInfo{
				Code:    "APP-UPDATE-POST-CHECK-BUNDLE-INVALID",
				Message: err.Error(),
			},
		})
		return err
	}
	if err := WriteState(layout, PersistentState{
		Status:           PersistentStatusSucceeded,
		LocalAppVersion:  plan.NewAppVersion,
		RemoteAppVersion: plan.NewAppVersion,
		PlanPath:         planPath,
		BackupPath:       plan.BackupPath,
	}); err != nil {
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

func (b DarwinBackend) writeManualRepair(layout Layout, code string, err error) error {
	_ = WriteState(layout, PersistentState{
		Status: PersistentStatusFailedManualRepair,
		LastError: ErrorInfo{
			Code:    code,
			Message: err.Error(),
		},
	})
	return err
}

func (b DarwinBackend) launchPostUpdateCheck(plan ApplyPlan, planPath string) error {
	cmd := exec.Command(filepath.Join(plan.InstallRoot, "Contents", "MacOS", "ant-chrome"), "--post-update-check", planPath)
	return cmd.Start()
}

func (b DarwinBackend) launchApplication(plan ApplyPlan) error {
	cmd := exec.Command(filepath.Join(plan.InstallRoot, "Contents", "MacOS", "ant-chrome"))
	return cmd.Start()
}
```

Add `time` to imports.

- [ ] **Step 4: Run Darwin backend tests**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestDarwinBackend' -count=1
```

Expected: PASS.

- [ ] **Step 5: Run appupdate package tests**

Run:

```bash
rtk go test ./backend/internal/appupdate -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 4**

Run:

```bash
rtk git add backend/internal/appupdate/darwin_backend.go backend/internal/appupdate/darwin_backend_test.go
rtk git commit -m "Implement darwin app bundle update backend"
```

Expected: commit succeeds.

---

### Task 5: Wire Platform Selection Into App APIs And CLI

**Files:**
- Modify: `backend/app_update.go`
- Modify: `backend/app_update_cli.go`
- Modify: `backend/app_update_test.go`

- [ ] **Step 1: Add failing backend integration tests**

Append to `backend/app_update_test.go`:

```go
func TestAppUpdateManagerUsesSelectedPlatformBackend(t *testing.T) {
	app := NewApp(t.TempDir(), "1.1.0")
	manager := app.appUpdateManager()
	if manager.Platform == nil {
		t.Fatal("expected app update platform backend")
	}
	if manager.Platform.Target() == "" {
		t.Fatal("expected platform target")
	}
}

func TestRunAppUpdateCLIRejectsUnsupportedModeBeforePlatformWork(t *testing.T) {
	err := RunAppUpdateCLI("bogus", filepath.Join(t.TempDir(), "missing-plan.json"), "1.1.0")
	if err == nil {
		t.Fatal("expected unsupported cli mode error")
	}
}
```

- [ ] **Step 2: Run tests**

Run:

```bash
rtk go test ./backend -run 'TestAppUpdateManagerUsesSelectedPlatformBackend|TestRunAppUpdateCLIRejectsUnsupportedModeBeforePlatformWork' -count=1
```

Expected: `TestAppUpdateManagerUsesSelectedPlatformBackend` passes on Windows because current code has Windows backend, but this test guards non-nil selection. `TestRunAppUpdateCLIRejectsUnsupportedModeBeforePlatformWork` passes only if unsupported mode returns before reading a missing plan.

- [ ] **Step 3: Replace hardcoded WindowsBackend in app_update.go**

Modify `backend/app_update.go` in `appUpdateManager()`:

```go
func (a *App) appUpdateManager() appupdate.Manager {
	layout := a.appUpdateLayout()
	platform, err := appupdate.NewPlatformBackend(goruntime.GOOS, goruntime.GOARCH, appupdate.PlatformOptions{
		ProcessID: os.Getpid(),
	})
	if err != nil {
		platform = appupdate.UnsupportedBackend{Err: err}
	}
	return appupdate.Manager{
		LocalAppVersion: a.appVersion(),
		Layout:          layout,
		Platform:        platform,
		ManifestProvider: appupdate.DefaultManifestProvider(func() appupdate.ManifestSourceResolution {
			return appupdate.ResolveManifestSource(resolveWorkspaceRuntimeDirWithConfig(a.config), a.config)
		}),
	}
}
```

Add `UnsupportedBackend` to `backend/internal/appupdate/platform.go`:

```go
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
```

- [ ] **Step 4: Replace hardcoded WindowsBackend in app_update_cli.go**

Modify `backend/app_update_cli.go`:

```go
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
```

- [ ] **Step 5: Run backend and appupdate tests**

Run:

```bash
rtk go test ./backend ./backend/internal/appupdate -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 5**

Run:

```bash
rtk git add backend/app_update.go backend/app_update_cli.go backend/app_update_test.go backend/internal/appupdate/platform.go
rtk git commit -m "Select app update backend by platform"
```

Expected: commit succeeds.

---

### Task 6: Target-Aware App-Update Verifier

**Files:**
- Modify: `tools/app-update/verify-app-update-package.py`

- [ ] **Step 1: Add local verifier fixture generator command**

Use this one-off shell command to create a darwin fixture under a temp directory:

```bash
tmp="$(mktemp -d)" && \
mkdir -p "$tmp/payload/Ant Browser.app/Contents/MacOS/publish" "$tmp/payload/Ant Browser.app/Contents/MacOS/bin" && \
printf '<plist></plist>' > "$tmp/payload/Ant Browser.app/Contents/Info.plist" && \
printf '#!/bin/sh\n' > "$tmp/payload/Ant Browser.app/Contents/MacOS/ant-chrome" && \
printf '{"schemaVersion":2}' > "$tmp/payload/Ant Browser.app/Contents/MacOS/publish/runtime-manifest.json" && \
printf '#!/bin/sh\n' > "$tmp/payload/Ant Browser.app/Contents/MacOS/bin/xray" && \
printf '#!/bin/sh\n' > "$tmp/payload/Ant Browser.app/Contents/MacOS/bin/sing-box" && \
chmod +x "$tmp/payload/Ant Browser.app/Contents/MacOS/ant-chrome" "$tmp/payload/Ant Browser.app/Contents/MacOS/bin/xray" "$tmp/payload/Ant Browser.app/Contents/MacOS/bin/sing-box" && \
(cd "$tmp/payload" && zip -qr "$tmp/AntBrowser-1.2.0-darwin-arm64.zip" "Ant Browser.app") && \
hash="$(shasum -a 256 "$tmp/AntBrowser-1.2.0-darwin-arm64.zip" | awk '{print $1}')" && \
size="$(stat -f%z "$tmp/AntBrowser-1.2.0-darwin-arm64.zip")" && \
printf '{"schemaVersion":1,"packages":[{"target":"darwin-arm64","payloadType":"full","url":"AntBrowser-1.2.0-darwin-arm64.zip","sha256":"%s","size":%s}]}\n' "$hash" "$size" > "$tmp/app-update-stable.json" && \
printf '%s\n' "$tmp" > /tmp/ant-browser-darwin-app-update-fixture-path && \
echo "$tmp"
```

Expected: prints a temp directory path containing `app-update-stable.json` and `AntBrowser-1.2.0-darwin-arm64.zip`.

- [ ] **Step 2: Run verifier against darwin fixture and verify failure before code change**

Run:

```bash
tmp="$(cat /tmp/ant-browser-darwin-app-update-fixture-path)"
rtk python3 tools/app-update/verify-app-update-package.py "$tmp/app-update-stable.json" "$tmp/AntBrowser-1.2.0-darwin-arm64.zip" darwin-arm64
```

Expected: FAIL because verifier still expects Windows files.

- [ ] **Step 3: Refactor verifier to use target-specific required paths**

Modify `tools/app-update/verify-app-update-package.py`:

```python
def required_paths_for_target(target: str) -> set[str]:
    if target == "windows-amd64":
        return {
            "ant-chrome.exe",
            "publish/runtime-manifest.json",
            "bin/xray.exe",
            "bin/sing-box.exe",
            "apps/agent/src/server/index.mjs",
            "runtime/node/node.exe",
        }
    if target in {"darwin-arm64", "darwin-amd64"}:
        return {
            "Ant Browser.app/Contents/Info.plist",
            "Ant Browser.app/Contents/MacOS/ant-chrome",
            "Ant Browser.app/Contents/MacOS/publish/runtime-manifest.json",
            "Ant Browser.app/Contents/MacOS/bin/xray",
            "Ant Browser.app/Contents/MacOS/bin/sing-box",
        }
    fail(f"unsupported target: {target}")
```

Replace the hardcoded `required = {...}` block in `main()` with:

```python
    required = required_paths_for_target(target)
    missing = sorted(required - names)
    if missing:
        fail("zip missing required files: " + ", ".join(missing))
```

Keep the existing forbidden mutable data check unchanged.

- [ ] **Step 4: Verify darwin fixture passes**

Run:

```bash
rtk python3 tools/app-update/verify-app-update-package.py "$tmp/app-update-stable.json" "$tmp/AntBrowser-1.2.0-darwin-arm64.zip" darwin-arm64
```

Expected:

```text
[OK] app update package verified
```

- [ ] **Step 5: Verify Windows unsupported regression is not introduced**

Run with any existing Windows app-update package if present:

```bash
VERSION="$(python3 -c 'import json; print(json.load(open("wails.json", encoding="utf-8"))["info"]["productVersion"])')"
rtk python3 tools/app-update/verify-app-update-package.py publish/output/app-update-stable.json "publish/output/AntBrowser-${VERSION}-windows-amd64.zip" windows-amd64
```

Expected when artifacts exist: `[OK] app update package verified`. If artifacts are not present, record that package-level Windows verifier regression must be run during release packaging.

- [ ] **Step 6: Commit Task 6**

Run:

```bash
rtk git add tools/app-update/verify-app-update-package.py
rtk git commit -m "Verify darwin app update packages"
```

Expected: commit succeeds.

---

### Task 7: macOS App-Update Runbook Section

**Files:**
- Modify: `docs/release/windows-packaging-and-update-runbook.md`

- [ ] **Step 1: Add macOS section to the runbook**

Append this section near the existing app-update regression section in `docs/release/windows-packaging-and-update-runbook.md`:

```markdown
## macOS Application Self-Update Regression

### Scope

macOS app-update uses the same app-update manifest and shared backend contract as Windows. The platform target must be `darwin-arm64` or `darwin-amd64`.

This phase supports full package updates only. Delta patching and release channel rollout are out of scope.

### Supported Install Location

Supported:

```text
~/Applications/Ant Browser.app
```

Unsupported for automatic update:

```text
/Applications/Ant Browser.app
/System/Applications/...
```

Unsupported installs must return `unsupported_install` and must not delete or replace any bundle files.

### Required Payload Shape

The macOS app-update zip must contain:

```text
Ant Browser.app/
  Contents/
    Info.plist
    MacOS/
      ant-chrome
      publish/runtime-manifest.json
      bin/xray
      bin/sing-box
```

The payload must not contain `data/`, `.db`, `.sqlite`, or `.sqlite3` files.

### Package Verification

Run:

```bash
VERSION="$(python3 -c 'import json; print(json.load(open("wails.json", encoding="utf-8"))["info"]["productVersion"])')"
python3 tools/app-update/verify-app-update-package.py publish/output/app-update-stable.json "publish/output/AntBrowser-${VERSION}-darwin-arm64.zip" darwin-arm64
```

or:

```bash
VERSION="$(python3 -c 'import json; print(json.load(open("wails.json", encoding="utf-8"))["info"]["productVersion"])')"
python3 tools/app-update/verify-app-update-package.py publish/output/app-update-stable.json "publish/output/AntBrowser-${VERSION}-darwin-amd64.zip" darwin-amd64
```

Expected:

```text
[OK] app update package verified
```

### Regression Matrix

1. Local file manifest smoke test.
2. HTTP manifest smoke test.
3. Soft update from `~/Applications/Ant Browser.app`.
4. Required update from `~/Applications/Ant Browser.app`.
5. Unsupported install at `/Applications/Ant Browser.app`.
6. Checksum mismatch.
7. Invalid `.app` payload.
8. Replace failure rollback.
9. Post-check version mismatch rollback.
10. Manual repair state after rollback failure.

### Release Readiness Checks

Before distributing a macOS release candidate:

1. Confirm the app bundle launches before packaging.
2. Confirm the app-update verifier passes for the target.
3. Confirm signing status for the release candidate.
4. Confirm notarization status for the release candidate.
5. Confirm Gatekeeper and quarantine behavior for the distributed artifact.

Signing, notarization, and Gatekeeper checks are release readiness checks. They are not runtime backend gates in this phase.
```

- [ ] **Step 2: Review runbook terminology**

Run:

```bash
rtk rg -n "macOS Application Self-Update|darwin-arm64|unsupported_install|not runtime backend gates" docs/release/windows-packaging-and-update-runbook.md
```

Expected: all four terms appear in the new section.

- [ ] **Step 3: Commit Task 7**

Run:

```bash
rtk git add -f docs/release/windows-packaging-and-update-runbook.md
rtk git commit -m "Document macOS app update regression flow"
```

Expected: commit succeeds.

---

### Task 8: Final Verification And Regression Sweep

**Files:**
- No code changes unless a verification failure identifies a concrete defect.

- [ ] **Step 1: Run focused Go package tests**

Run:

```bash
rtk go test ./backend/internal/appupdate ./backend -count=1
```

Expected: PASS.

- [ ] **Step 2: Run release-related Go tests from the prior phase**

Run:

```bash
rtk go test . ./backend ./backend/internal/appupdate ./backend/internal/config -count=1
```

Expected: PASS.

- [ ] **Step 3: Run frontend build**

Run:

```bash
rtk npm --prefix frontend run build
```

Expected: build succeeds.

- [ ] **Step 4: Verify no accidental implementation drift**

Run:

```bash
rtk git status --short
rtk git log --oneline -8
```

Expected: worktree clean after commits; recent commits show the task commits from this plan.

- [ ] **Step 5: Record macOS manual regression gap if no built app-update artifacts exist**

Compute the app version first:

```bash
VERSION="$(python3 -c 'import json; print(json.load(open("wails.json", encoding="utf-8"))["info"]["productVersion"])')"
```

If `publish/output/AntBrowser-${VERSION}-darwin-arm64.zip` or `publish/output/AntBrowser-${VERSION}-darwin-amd64.zip` is not present, record this in the final handoff:

```text
macOS package-level manual app-update regression was not run because no release payload artifact exists in publish/output.
```

If artifacts exist, run:

```bash
rtk python3 tools/app-update/verify-app-update-package.py publish/output/app-update-stable.json "publish/output/AntBrowser-${VERSION}-darwin-arm64.zip" darwin-arm64
```

Expected: `[OK] app update package verified`.

- [ ] **Step 6: Final commit if verification fixes were needed**

If Step 1, Step 2, Step 3, or Step 5 required code or doc fixes, run:

```bash
rtk git add -A
rtk git commit -m "Stabilize cross-platform app update"
```

Expected: commit succeeds. If no files changed, do not create an empty commit.
