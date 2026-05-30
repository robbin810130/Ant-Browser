# Ant-Browser Release Stability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a manifest-driven release/runtime stability layer for Ant-Browser so Windows self-install is reliable, macOS can auto-fetch large runtime resources on first launch, startup blocks on environment readiness, updates are staged and rollback-safe, and users can export actionable diagnostics.

**Architecture:** Add a backend release/runtime subsystem that owns manifest loading, compatibility checks, environment self-checks, auto-repair, staged resource activation, and diagnostic export. Expose that subsystem through Wails bindings and a dedicated frontend bootstrap gate plus settings entry, while updating Windows/macOS publish assets to match the new runtime contract.

**Tech Stack:** Go, Wails v2.12.0, React 18, TypeScript, Zustand, NSIS, shell/publish scripts, existing `internal/browser`, `internal/proxy`, `internal/apppath`, and Wails bindings generation.

---

## File Structure

### Backend release/runtime units

- `backend/internal/release/manifest.go`
  Loads app/resource manifests, validates schema, resolves platform package requirements, and answers compatibility questions.
- `backend/internal/release/manifest_test.go`
  Covers manifest parsing, compatibility, and required-package selection.
- `backend/internal/release/runtime_layout.go`
  Resolves versioned runtime directories, active pointer files, staging directories, and diagnostic bundle output locations.
- `backend/internal/release/runtime_layout_test.go`
  Verifies directory and pointer resolution for detached/runtime state roots.
- `backend/internal/release/checker.go`
  Runs environment self-checks and emits structured failure items.
- `backend/internal/release/checker_test.go`
  Covers pass, repairable, and blocked outcomes.
- `backend/internal/release/repair.go`
  Maps check failures into deterministic repair actions and reruns full checks.
- `backend/internal/release/repair_test.go`
  Covers cleanup of half-installed resources, bad hashes, and writable-path issues.
- `backend/internal/release/updater.go`
  Performs startup update classification, staged installs, pointer switching, and resource rollback.
- `backend/internal/release/updater_test.go`
  Covers soft update, required update, and rollback-on-activation-failure.
- `backend/internal/release/diagnostics.go`
  Writes structured events, redacts sensitive data, and exports diagnostic bundles.
- `backend/internal/release/diagnostics_test.go`
  Verifies event payloads, redaction, and bundle contents.

### Backend app bridge units

- `backend/app_release_runtime.go`
  Wails bridge for bootstrap gate, repair, update confirmation, environment diagnostics, and settings-entry actions.
- `backend/app_release_runtime_test.go`
  Covers Wails-facing behavior with fake manifests and temporary runtime layouts.
- `backend/app.go`
  Wires the release manager into startup and existing runtime path handling.
- `backend/runtime_paths.go`
  Re-exports any new runtime path helpers used by tests or publish tooling.
- `backend/app_paths.go`
  Adds helpers for install/runtime/user-data directories where needed.

### Frontend runtime shell units

- `frontend/src/modules/runtime/types.ts`
  Shared TS types for environment status, failure items, update prompts, and diagnostics export state.
- `frontend/src/modules/runtime/api.ts`
  Wails binding wrappers for the release/runtime bridge.
- `frontend/src/store/runtimeStore.ts`
  Owns bootstrap-gate state, repair progress, and update prompt state.
- `frontend/src/modules/runtime/pages/EnvironmentGatePage.tsx`
  Startup gate UI with checking/repairing/blocked states.
- `frontend/src/modules/runtime/components/EnvironmentStatusCard.tsx`
  Reusable detail card for repair status, failure reasons, and retry/export actions.
- `frontend/src/modules/runtime/components/UpdatePromptModal.tsx`
  User-confirmed update dialog for soft/required updates.
- `frontend/src/App.tsx`
  Inserts environment bootstrap gate ahead of protected routes.
- `frontend/src/modules/settings/SettingsPage.tsx`
  Adds persistent “环境检查与修复 / 导出诊断” entry.
- `frontend/src/modules/settings/api.ts`
  Exposes new runtime settings actions.
- `frontend/src/wailsjs/go/main/App.d.ts`
- `frontend/src/wailsjs/go/main/App.js`
- `frontend/src/wailsjs/go/models.ts`
  Regenerated after backend bridge changes.

### Publish/release assets

- `publish/runtime-manifest.json`
  Upgrade from pinned-file manifest to manifest that also describes app/resource compatibility and required packages.
- `publish/runtime-sources.json`
  Declares fetchable runtime package sources for macOS and future refreshes.
- `publish/installer.nsi`
  Ensures Windows bundled runtime layout matches the new manifest/runtime directories.
- `publish/mac/publish-mac.sh`
  Ships light package metadata and excludes auto-fetched large resources.
- `bat/publish.ps1`
- `bat/publish.bat`
  Populate staged files and manifest metadata consistently on Windows.
- `tools/public-release/README.md`
  Documents the new package/resource release contract and operator workflow.

### Existing files expected to stay aligned

- `backend/internal/browser/download_core.go`
  Reuse or extract generic download/hash/extract helpers instead of duplicating network logic.
- `backend/internal/browser/core.go`
  Continue using existing executable validation helpers for runtime/browser core probes.
- `backend/internal/apppath/apppath.go`
  Keep detached state-root semantics intact while adding runtime version directories.
- `frontend/src/modules/browser/pages/CoreManagementPage.tsx`
  Remains the user-facing browser core management page; runtime bootstrap should not fork its logic.

### Task 1: Add Release Manifest Domain Model

**Files:**
- Create: `backend/internal/release/manifest.go`
- Create: `backend/internal/release/manifest_test.go`
- Modify: `publish/runtime-manifest.json`
- Modify: `publish/runtime-sources.json`

- [ ] **Step 1: Write the failing manifest compatibility tests**

```go
package release

import "testing"

func TestManifestSelectPackagesForTarget(t *testing.T) {
	manifest := Manifest{
		AppVersion:              "1.2.0",
		MinimumResourceVersion:  "2026.05.12",
		Packages: []RuntimePackage{
			{ID: "win-core", Target: "windows-amd64", Kind: "browser-core", Required: true, Version: "136.0.0"},
			{ID: "mac-core", Target: "darwin-arm64", Kind: "browser-core", Required: true, Version: "136.0.0"},
		},
	}

	pkgs, err := manifest.RequiredPackages("windows-amd64")
	if err != nil {
		t.Fatalf("RequiredPackages returned error: %v", err)
	}
	if len(pkgs) != 1 || pkgs[0].ID != "win-core" {
		t.Fatalf("expected win-core package, got %#v", pkgs)
	}
}

func TestManifestDetectsResourceVersionBelowFloor(t *testing.T) {
	manifest := Manifest{MinimumResourceVersion: "2026.05.12"}
	if manifest.ResourceCompatible("2026.05.01") {
		t.Fatal("expected old resource version to be incompatible")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./backend/internal/release -run 'TestManifest' -count=1`

Expected: FAIL with `undefined: Manifest` and `undefined: RuntimePackage`

- [ ] **Step 3: Implement the manifest types and compatibility helpers**

```go
package release

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Manifest struct {
	SchemaVersion           int              `json:"schemaVersion"`
	AppVersion              string           `json:"appVersion"`
	MinimumResourceVersion  string           `json:"minimumResourceVersion"`
	Packages                []RuntimePackage `json:"packages"`
}

type RuntimePackage struct {
	ID       string `json:"id"`
	Target   string `json:"target"`
	Kind     string `json:"kind"`
	Required bool   `json:"required"`
	Version  string `json:"version"`
	SHA256   string `json:"sha256"`
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != 2 {
		return Manifest{}, fmt.Errorf("unsupported manifest schema: %d", manifest.SchemaVersion)
	}
	return manifest, nil
}

func (m Manifest) RequiredPackages(target string) ([]RuntimePackage, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("target is required")
	}
	var out []RuntimePackage
	for _, pkg := range m.Packages {
		if pkg.Required && strings.EqualFold(pkg.Target, target) {
			out = append(out, pkg)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no required packages for target %s", target)
	}
	return out, nil
}

func (m Manifest) ResourceCompatible(version string) bool {
	return strings.TrimSpace(version) >= strings.TrimSpace(m.MinimumResourceVersion)
}
```

- [ ] **Step 4: Update publish manifests to schema v2**

```json
{
  "schemaVersion": 2,
  "appVersion": "1.1.0",
  "minimumResourceVersion": "2026.05.12",
  "packages": [
    {
      "id": "windows-amd64-browser-core",
      "target": "windows-amd64",
      "kind": "browser-core",
      "required": true,
      "version": "136.0.0",
      "sha256": "6d9f6c2d9d2ce9d6f53fd8d46f3d2de9f7d5222a52bc20d7f7c42d31a8f7ac10",
      "path": "chrome/default-core"
    }
  ]
}
```

- [ ] **Step 5: Run tests to verify the manifest layer passes**

Run: `rtk go test ./backend/internal/release -run 'TestManifest' -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance add backend/internal/release/manifest.go backend/internal/release/manifest_test.go publish/runtime-manifest.json publish/runtime-sources.json
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance commit -m "feat: add release manifest model"
```

### Task 2: Introduce Versioned Runtime Layout

**Files:**
- Create: `backend/internal/release/runtime_layout.go`
- Create: `backend/internal/release/runtime_layout_test.go`
- Modify: `backend/runtime_paths.go`
- Modify: `backend/app_paths.go`
- Modify: `backend/internal/apppath/apppath.go`
- Test: `backend/internal/apppath/apppath_test.go`

- [ ] **Step 1: Write the failing layout tests**

```go
package release

import "testing"

func TestRuntimeLayoutPaths(t *testing.T) {
	layout := NewRuntimeLayout("/install/root", "/state/root")
	if got := layout.ActivePointerPath(); got != "/state/root/runtime/current.json" {
		t.Fatalf("unexpected active pointer path: %s", got)
	}
	if got := layout.VersionDir("2026.05.12"); got != "/state/root/runtime/versions/2026.05.12" {
		t.Fatalf("unexpected version dir: %s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./backend/internal/release -run 'TestRuntimeLayoutPaths' -count=1`

Expected: FAIL with `undefined: NewRuntimeLayout`

- [ ] **Step 3: Implement versioned runtime layout helpers**

```go
package release

import "path/filepath"

type RuntimeLayout struct {
	InstallRoot string
	StateRoot   string
}

func NewRuntimeLayout(installRoot, stateRoot string) RuntimeLayout {
	return RuntimeLayout{InstallRoot: installRoot, StateRoot: stateRoot}
}

func (l RuntimeLayout) RuntimeRoot() string {
	return filepath.Join(l.StateRoot, "runtime")
}

func (l RuntimeLayout) VersionsRoot() string {
	return filepath.Join(l.RuntimeRoot(), "versions")
}

func (l RuntimeLayout) VersionDir(version string) string {
	return filepath.Join(l.VersionsRoot(), version)
}

func (l RuntimeLayout) StagingRoot() string {
	return filepath.Join(l.RuntimeRoot(), "staging")
}

func (l RuntimeLayout) ActivePointerPath() string {
	return filepath.Join(l.RuntimeRoot(), "current.json")
}

func (l RuntimeLayout) DiagnosticsRoot() string {
	return filepath.Join(l.StateRoot, "diagnostics")
}
```

- [ ] **Step 4: Re-export the new layout through backend helpers**

```go
package backend

import "ant-chrome/backend/internal/release"

func RuntimeReleaseLayout(appRoot string) release.RuntimeLayout {
	return release.NewRuntimeLayout(apppath.InstallRoot(appRoot), apppath.StateRoot(appRoot))
}
```

- [ ] **Step 5: Run path-related tests**

Run: `rtk go test ./backend/internal/release ./backend/internal/apppath -run 'TestRuntimeLayoutPaths|TestEnsureWritableLayout' -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance add backend/internal/release/runtime_layout.go backend/internal/release/runtime_layout_test.go backend/runtime_paths.go backend/app_paths.go backend/internal/apppath/apppath.go backend/internal/apppath/apppath_test.go
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance commit -m "feat: add versioned runtime layout"
```

### Task 3: Build Environment Self-Check Engine

**Files:**
- Create: `backend/internal/release/checker.go`
- Create: `backend/internal/release/checker_test.go`
- Modify: `backend/internal/browser/core.go`
- Modify: `backend/app.go`
- Modify: `backend/app_release_runtime.go`
- Create: `backend/app_release_runtime_test.go`

- [ ] **Step 1: Write the failing checker tests**

```go
package release

import "testing"

func TestCheckerMarksMissingManifestBlocked(t *testing.T) {
	checker := Checker{}
	result := checker.Run(CheckInput{
		ManifestPath: "/missing/runtime-manifest.json",
		Target:       "darwin-arm64",
	})
	if result.State != StateBlocked {
		t.Fatalf("expected blocked state, got %s", result.State)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-MANIFEST-MISSING" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
}

func TestCheckerMarksMissingRuntimeRepairable(t *testing.T) {
	checker := Checker{Manifest: Manifest{
		MinimumResourceVersion: "2026.05.12",
		Packages: []RuntimePackage{{ID: "mac-core", Target: "darwin-arm64", Kind: "browser-core", Required: true}},
	}}
	result := checker.Run(CheckInput{
		Target:          "darwin-arm64",
		ResourceVersion: "2026.05.12",
	})
	if result.State != StateRepairable {
		t.Fatalf("expected repairable state, got %s", result.State)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./backend/internal/release -run 'TestChecker' -count=1`

Expected: FAIL with `undefined: Checker`, `undefined: CheckInput`, `undefined: StateBlocked`

- [ ] **Step 3: Implement the checker and result model**

```go
package release

type CheckState string

const (
	StatePass       CheckState = "pass"
	StateRepairable CheckState = "repairable"
	StateBlocked    CheckState = "blocked"
)

type FailureItem struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	Repairable bool   `json:"repairable"`
}

type CheckInput struct {
	ManifestPath     string
	Target           string
	ResourceVersion  string
	BrowserCorePath  string
}

type CheckResult struct {
	State CheckState    `json:"state"`
	Items []FailureItem `json:"items"`
}

type Checker struct {
	Manifest Manifest
}

func (c Checker) Run(input CheckInput) CheckResult {
	if input.ManifestPath == "" {
		return CheckResult{State: StateBlocked, Items: []FailureItem{{
			Code: "ENV-MANIFEST-MISSING", Severity: "error", Message: "未找到运行时 manifest", Repairable: false,
		}}}
	}
	if !c.Manifest.ResourceCompatible(input.ResourceVersion) {
		return CheckResult{State: StateRepairable, Items: []FailureItem{{
			Code: "PKG-RESOURCE-OUTDATED", Severity: "error", Message: "资源版本过旧，需要修复", Repairable: true,
		}}}
	}
	return CheckResult{State: StatePass}
}
```

- [ ] **Step 4: Expose a bootstrap gate bridge from the backend**

```go
func (a *App) GetDesktopEnvironmentStatus() (release.CheckResult, error) {
	manager, err := a.releaseManager()
	if err != nil {
		return release.CheckResult{}, err
	}
	return manager.RunStartupCheck(a.ctx)
}
```

- [ ] **Step 5: Run checker and app bridge tests**

Run: `rtk go test ./backend/internal/release ./backend -run 'TestChecker|TestGetDesktopEnvironmentStatus' -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance add backend/internal/release/checker.go backend/internal/release/checker_test.go backend/internal/browser/core.go backend/app.go backend/app_release_runtime.go backend/app_release_runtime_test.go
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance commit -m "feat: add environment self-check gate"
```

### Task 4: Implement Deterministic Auto-Repair

**Files:**
- Create: `backend/internal/release/repair.go`
- Create: `backend/internal/release/repair_test.go`
- Modify: `backend/internal/browser/download_core.go`
- Modify: `backend/app_release_runtime.go`
- Modify: `backend/app_release_runtime_test.go`

- [ ] **Step 1: Write the failing repair tests**

```go
package release

import "testing"

func TestRepairPlanForOutdatedResource(t *testing.T) {
	plan := BuildRepairPlan(CheckResult{
		State: StateRepairable,
		Items: []FailureItem{{
			Code: "PKG-RESOURCE-OUTDATED",
			Repairable: true,
		}},
	})
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != "fetch-package" {
		t.Fatalf("unexpected repair plan: %#v", plan)
	}
}

func TestRepairPlanRejectsBlockedItems(t *testing.T) {
	_, err := ExecuteRepair(nil, CheckResult{
		State: StateBlocked,
		Items: []FailureItem{{Code: "NET-PROXY-AUTH-FAILED", Repairable: false}},
	})
	if err == nil {
		t.Fatal("expected blocked result to reject auto repair")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./backend/internal/release -run 'TestRepairPlan' -count=1`

Expected: FAIL with `undefined: BuildRepairPlan` and `undefined: ExecuteRepair`

- [ ] **Step 3: Extract generic fetch/validate helpers and implement repair execution**

```go
package release

type RepairAction struct {
	Kind      string `json:"kind"`
	PackageID string `json:"packageId,omitempty"`
}

type RepairPlan struct {
	Actions []RepairAction `json:"actions"`
}

func BuildRepairPlan(result CheckResult) RepairPlan {
	var plan RepairPlan
	for _, item := range result.Items {
		switch item.Code {
		case "PKG-RESOURCE-OUTDATED", "PKG-RUNTIME-MISSING":
			plan.Actions = append(plan.Actions, RepairAction{Kind: "fetch-package", PackageID: item.Code})
		case "ENV-TEMP-LEFTOVER":
			plan.Actions = append(plan.Actions, RepairAction{Kind: "cleanup-temp"})
		}
	}
	return plan
}

func ExecuteRepair(manager *Manager, result CheckResult) (CheckResult, error) {
	if result.State == StateBlocked {
		return result, fmt.Errorf("blocked failures must not enter auto repair")
	}
	plan := BuildRepairPlan(result)
	for _, action := range plan.Actions {
		if err := manager.ApplyRepairAction(action); err != nil {
			return CheckResult{}, err
		}
	}
	return manager.RunStartupCheck(context.Background())
}
```

- [ ] **Step 4: Wire backend bridge progress events**

```go
func (a *App) RepairDesktopEnvironment() (release.CheckResult, error) {
	manager, err := a.releaseManager()
	if err != nil {
		return release.CheckResult{}, err
	}
	return manager.RepairAndRecheck(a.ctx)
}
```

- [ ] **Step 5: Run repair tests**

Run: `rtk go test ./backend/internal/release ./backend -run 'TestRepairPlan|TestRepairDesktopEnvironment' -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance add backend/internal/release/repair.go backend/internal/release/repair_test.go backend/internal/browser/download_core.go backend/app_release_runtime.go backend/app_release_runtime_test.go
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance commit -m "feat: add runtime auto repair flow"
```

### Task 5: Add Update Classification, Activation, and Rollback

**Files:**
- Create: `backend/internal/release/updater.go`
- Create: `backend/internal/release/updater_test.go`
- Modify: `backend/internal/release/runtime_layout.go`
- Modify: `backend/app_release_runtime.go`
- Modify: `backend/app_release_runtime_test.go`

- [ ] **Step 1: Write the failing update/rollback tests**

```go
package release

import "testing"

func TestClassifyUpdateRequired(t *testing.T) {
	manager := Manager{
		LocalManifest:  Manifest{AppVersion: "1.1.0", MinimumResourceVersion: "2026.05.12"},
		RemoteManifest: Manifest{AppVersion: "1.2.0", MinimumResourceVersion: "2026.06.01"},
	}
	state := manager.ClassifyUpdate("2026.05.12")
	if state.Kind != "required" {
		t.Fatalf("expected required update, got %#v", state)
	}
}

func TestActivateRuntimeRollbackOnProbeFailure(t *testing.T) {
	layout := NewRuntimeLayout("/install", "/state")
	manager := Manager{Layout: layout}
	err := manager.ActivateVersion("2026.06.01", func(string) error { return assertErrProbeFailed })
	if err == nil {
		t.Fatal("expected activation to fail")
	}
	if got := manager.CurrentVersion(); got != "2026.05.12" {
		t.Fatalf("expected rollback to previous version, got %s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./backend/internal/release -run 'TestClassifyUpdateRequired|TestActivateRuntimeRollbackOnProbeFailure' -count=1`

Expected: FAIL with `undefined: Manager`, `undefined: assertErrProbeFailed`

- [ ] **Step 3: Implement update state, staged activation, and rollback**

```go
package release

type UpdateState struct {
	Kind              string `json:"kind"`
	LocalAppVersion   string `json:"localAppVersion"`
	RemoteAppVersion  string `json:"remoteAppVersion"`
	ResourceVersion   string `json:"resourceVersion"`
}

func (m Manager) ClassifyUpdate(localResourceVersion string) UpdateState {
	if !m.RemoteManifest.ResourceCompatible(localResourceVersion) {
		return UpdateState{
			Kind:             "required",
			LocalAppVersion:  m.LocalManifest.AppVersion,
			RemoteAppVersion: m.RemoteManifest.AppVersion,
			ResourceVersion:  m.RemoteManifest.MinimumResourceVersion,
		}
	}
	if m.RemoteManifest.AppVersion > m.LocalManifest.AppVersion {
		return UpdateState{Kind: "soft", LocalAppVersion: m.LocalManifest.AppVersion, RemoteAppVersion: m.RemoteManifest.AppVersion}
	}
	return UpdateState{Kind: "none", LocalAppVersion: m.LocalManifest.AppVersion, RemoteAppVersion: m.RemoteManifest.AppVersion}
}

func (m *Manager) ActivateVersion(version string, probe func(string) error) error {
	previous := m.CurrentVersion()
	if err := m.writeCurrentVersion(version); err != nil {
		return err
	}
	if err := probe(m.Layout.VersionDir(version)); err != nil {
		_ = m.writeCurrentVersion(previous)
		return err
	}
	return nil
}
```

- [ ] **Step 4: Expose startup update check and user-confirmed apply methods**

```go
func (a *App) CheckDesktopReleaseUpdate() (release.UpdateState, error) {
	manager, err := a.releaseManager()
	if err != nil {
		return release.UpdateState{}, err
	}
	return manager.CheckForUpdate(a.ctx)
}

func (a *App) ApplyDesktopReleaseUpdate() (release.UpdateState, error) {
	manager, err := a.releaseManager()
	if err != nil {
		return release.UpdateState{}, err
	}
	return manager.ApplyConfirmedUpdate(a.ctx)
}
```

- [ ] **Step 5: Run update tests**

Run: `rtk go test ./backend/internal/release ./backend -run 'TestClassifyUpdateRequired|TestActivateRuntimeRollbackOnProbeFailure|TestCheckDesktopReleaseUpdate' -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance add backend/internal/release/updater.go backend/internal/release/updater_test.go backend/internal/release/runtime_layout.go backend/app_release_runtime.go backend/app_release_runtime_test.go
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance commit -m "feat: add staged runtime update flow"
```

### Task 6: Add Structured Diagnostics and Export

**Files:**
- Create: `backend/internal/release/diagnostics.go`
- Create: `backend/internal/release/diagnostics_test.go`
- Modify: `backend/internal/logger/logger.go`
- Modify: `backend/app_release_runtime.go`
- Modify: `backend/app_release_runtime_test.go`

- [ ] **Step 1: Write the failing diagnostics tests**

```go
package release

import "testing"

func TestDiagnosticBundleRedactsSecrets(t *testing.T) {
	events := []DiagnosticEvent{{
		Stage: "update",
		Result: "failure",
		Fields: map[string]string{
			"accessToken": "secret-token",
			"proxyPassword": "top-secret",
			"summary": "hash mismatch",
		},
	}}
	bundle := BuildDiagnosticBundle(events)
	if bundle.Contains("secret-token") || bundle.Contains("top-secret") {
		t.Fatal("expected diagnostic bundle to redact sensitive values")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `rtk go test ./backend/internal/release -run 'TestDiagnosticBundleRedactsSecrets' -count=1`

Expected: FAIL with `undefined: DiagnosticEvent` and `undefined: BuildDiagnosticBundle`

- [ ] **Step 3: Implement diagnostic event model and bundle export**

```go
package release

type DiagnosticEvent struct {
	EventTime       string            `json:"eventTime"`
	Stage           string            `json:"stage"`
	Result          string            `json:"result"`
	ErrorCode       string            `json:"errorCode,omitempty"`
	AppVersion      string            `json:"appVersion"`
	ManifestVersion string            `json:"manifestVersion"`
	ResourceVersion string            `json:"resourceVersion"`
	Summary         string            `json:"summary"`
	Fields          map[string]string `json:"fields,omitempty"`
}

func sanitizeFields(fields map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range fields {
		switch strings.ToLower(k) {
		case "accesstoken", "password", "proxypassword", "cookie":
			out[k] = "[REDACTED]"
		default:
			out[k] = v
		}
	}
	return out
}
```

- [ ] **Step 4: Add backend bridge methods for export and detail retrieval**

```go
func (a *App) ExportDesktopEnvironmentDiagnostics() (string, error) {
	manager, err := a.releaseManager()
	if err != nil {
		return "", err
	}
	return manager.ExportDiagnostics(a.ctx)
}
```

- [ ] **Step 5: Run diagnostics tests**

Run: `rtk go test ./backend/internal/release ./backend -run 'TestDiagnosticBundleRedactsSecrets|TestExportDesktopEnvironmentDiagnostics' -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance add backend/internal/release/diagnostics.go backend/internal/release/diagnostics_test.go backend/internal/logger/logger.go backend/app_release_runtime.go backend/app_release_runtime_test.go
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance commit -m "feat: add release diagnostics export"
```

### Task 7: Add Frontend Environment Bootstrap Gate

**Files:**
- Create: `frontend/src/modules/runtime/types.ts`
- Create: `frontend/src/modules/runtime/api.ts`
- Create: `frontend/src/store/runtimeStore.ts`
- Create: `frontend/src/modules/runtime/pages/EnvironmentGatePage.tsx`
- Create: `frontend/src/modules/runtime/components/EnvironmentStatusCard.tsx`
- Create: `frontend/src/modules/runtime/components/UpdatePromptModal.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/modules/settings/api.ts`

- [ ] **Step 1: Write the failing compile-time integration check by referencing missing runtime modules**

```ts
import { useRuntimeStore } from './store/runtimeStore'
import { getDesktopEnvironmentStatus } from './modules/runtime/api'

void useRuntimeStore
void getDesktopEnvironmentStatus
```

- [ ] **Step 2: Run build to verify it fails**

Run: `rtk npm --prefix /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance/frontend run build`

Expected: FAIL with module resolution errors for `runtimeStore` or `modules/runtime/api`

- [ ] **Step 3: Add runtime store and Wails API wrappers**

```ts
export interface EnvironmentStatus {
  state: 'checking' | 'pass' | 'repairable' | 'blocked'
  items: Array<{
    code: string
    severity: 'info' | 'warning' | 'error'
    message: string
    repairable: boolean
  }>
}

export async function getDesktopEnvironmentStatus(): Promise<EnvironmentStatus> {
  const bindings: any = await getBindings()
  return (await bindings.GetDesktopEnvironmentStatus()) ?? { state: 'blocked', items: [] }
}
```

- [ ] **Step 4: Gate app startup on environment readiness**

```tsx
export default function App() {
  const environmentReady = useRuntimeStore((s) => s.environmentReady)
  const bootstrap = useRuntimeStore((s) => s.bootstrap)

  useEffect(() => {
    void bootstrap()
  }, [bootstrap])

  if (!environmentReady) {
    return <EnvironmentGatePage />
  }

  return <ProtectedAppShell />
}
```

- [ ] **Step 5: Run frontend build**

Run: `rtk npm --prefix /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance/frontend run build`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance add frontend/src/modules/runtime/types.ts frontend/src/modules/runtime/api.ts frontend/src/store/runtimeStore.ts frontend/src/modules/runtime/pages/EnvironmentGatePage.tsx frontend/src/modules/runtime/components/EnvironmentStatusCard.tsx frontend/src/modules/runtime/components/UpdatePromptModal.tsx frontend/src/App.tsx frontend/src/modules/settings/api.ts
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance commit -m "feat: add environment bootstrap gate"
```

### Task 8: Add Settings Repair + Diagnostics Entry

**Files:**
- Modify: `frontend/src/modules/settings/SettingsPage.tsx`
- Modify: `frontend/src/modules/settings/api.ts`
- Modify: `frontend/src/modules/settings/types.ts`
- Modify: `frontend/src/shared/layout/Topbar.tsx`

- [ ] **Step 1: Write the failing compile-time check for settings runtime actions**

```ts
import { exportEnvironmentDiagnostics, repairEnvironmentNow } from './api'

void exportEnvironmentDiagnostics
void repairEnvironmentNow
```

- [ ] **Step 2: Run build to verify it fails**

Run: `rtk npm --prefix /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance/frontend run build`

Expected: FAIL with missing exports from `frontend/src/modules/settings/api.ts`

- [ ] **Step 3: Add settings actions and UI card**

```ts
export async function repairEnvironmentNow(): Promise<EnvironmentStatus> {
  const bindings: any = await getBindings()
  return await bindings.RepairDesktopEnvironment()
}

export async function exportEnvironmentDiagnostics(): Promise<string> {
  const bindings: any = await getBindings()
  return await bindings.ExportDesktopEnvironmentDiagnostics()
}
```

```tsx
<Card title="环境检查与修复">
  <p className="text-sm text-gray-600">检查运行时环境、执行自动修复，并导出诊断包给支持团队。</p>
  <div className="flex gap-2">
    <Button onClick={() => void handleRunEnvironmentCheck()}>重新检查</Button>
    <Button variant="secondary" onClick={() => void handleRepairEnvironment()}>自动修复</Button>
    <Button variant="ghost" onClick={() => void handleExportDiagnostics()}>导出诊断</Button>
  </div>
</Card>
```

- [ ] **Step 4: Run frontend build**

Run: `rtk npm --prefix /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance/frontend run build`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance add frontend/src/modules/settings/SettingsPage.tsx frontend/src/modules/settings/api.ts frontend/src/modules/settings/types.ts frontend/src/shared/layout/Topbar.tsx
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance commit -m "feat: add runtime repair actions in settings"
```

### Task 9: Align Publish Assets With Runtime Contract

**Files:**
- Modify: `publish/installer.nsi`
- Modify: `publish/mac/publish-mac.sh`
- Modify: `bat/publish.ps1`
- Modify: `bat/publish.bat`
- Modify: `tools/public-release/README.md`

- [ ] **Step 1: Add a publish dry-run check script or command path and make it fail first**

```bash
rtk rg "runtime/current.json|minimumResourceVersion|packages" publish/installer.nsi publish/mac/publish-mac.sh bat/publish.ps1 bat/publish.bat
```

Expected: no matches for the new runtime contract yet

- [ ] **Step 2: Update Windows installer staging rules**

```nsi
!if /FileExists "${STAGINGDIR}\runtime\manifest.json"
  SetOutPath "$INSTDIR\runtime"
  File "${STAGINGDIR}\runtime\manifest.json"
!endif
!if /FileExists "${STAGINGDIR}\runtime\versions\*"
  SetOutPath "$INSTDIR\runtime\versions"
  File /r "${STAGINGDIR}\runtime\versions\*"
!endif
```

- [ ] **Step 3: Update macOS publish script to ship light package metadata only**

```bash
mkdir -p "$STAGING_DIR/runtime"
cp "$ROOT_DIR/publish/runtime-manifest.json" "$STAGING_DIR/runtime/manifest.json"
rm -rf "$STAGING_DIR/runtime/versions"
```

- [ ] **Step 4: Update public release docs**

```md
## Runtime contract

- Windows package includes required runtime versions under `runtime/versions/`
- macOS package ships only `runtime/manifest.json` and fetches required packages on first launch
- Both platforms activate resources through `runtime/current.json`
```

- [ ] **Step 5: Run publish-related verification**

Run: `rtk rg "runtime/current.json|runtime/manifest.json|minimumResourceVersion" publish/installer.nsi publish/mac/publish-mac.sh bat/publish.ps1 bat/publish.bat tools/public-release/README.md`

Expected: all files contain the new runtime contract markers

- [ ] **Step 6: Commit**

```bash
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance add publish/installer.nsi publish/mac/publish-mac.sh bat/publish.ps1 bat/publish.bat tools/public-release/README.md
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance commit -m "build: align publish assets with runtime contract"
```

### Task 10: Regenerate Bindings and Run Full Verification

**Files:**
- Modify: `frontend/src/wailsjs/go/main/App.d.ts`
- Modify: `frontend/src/wailsjs/go/main/App.js`
- Modify: `frontend/src/wailsjs/go/models.ts`
- Modify: `docs/superpowers/plans/2026-05-12-ant-browser-release-stability-implementation-plan.md`

- [ ] **Step 1: Regenerate Wails bindings after backend bridge changes**

Run: `rtk go run github.com/wailsapp/wails/v2/cmd/wails@v2.12.0 generate module`

Expected: updated `frontend/src/wailsjs/go/main/App.d.ts`, `frontend/src/wailsjs/go/main/App.js`, and `frontend/src/wailsjs/go/models.ts`

- [ ] **Step 2: Run focused backend release/runtime tests**

Run: `rtk go test ./backend/internal/release ./backend -run 'TestManifest|TestRuntimeLayout|TestChecker|TestRepair|TestClassifyUpdate|TestDiagnostic' -count=1`

Expected: PASS

- [ ] **Step 3: Run full backend regression**

Run: `rtk go test ./backend -count=1`

Expected: PASS

- [ ] **Step 4: Run frontend build**

Run: `rtk npm --prefix /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance/frontend run build`

Expected: PASS

- [ ] **Step 5: Record manual acceptance checklist in the plan before execution closes**

```md
- Windows clean machine self-install reaches login or repair gate
- macOS first-launch fetch completes without manual dependency hunting
- broken runtime enters repairable or blocked state with human-readable reason
- diagnostics export produces a redacted bundle
- startup soft update prompts for confirmation
- activation failure rolls back to the previous resource version
```

- [ ] **Step 6: Commit**

```bash
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance add frontend/src/wailsjs/go/main/App.d.ts frontend/src/wailsjs/go/main/App.js frontend/src/wailsjs/go/models.ts docs/superpowers/plans/2026-05-12-ant-browser-release-stability-implementation-plan.md
rtk git -C /Users/robbin/.config/superpowers/worktrees/ant-browser-fork/codex-workspace-native-instance commit -m "chore: verify release stability implementation"
```

## Self-Review

### Spec coverage

- Windows heavy package: covered by Task 9.
- macOS first-launch runtime fetch: covered by Tasks 4 and 9.
- separate `auth ready` and `environment ready`: covered by Task 7.
- bootstrap gate with pass/repairable/blocked states: covered by Tasks 3 and 7.
- deterministic auto-repair only: covered by Task 4.
- startup update check + user confirmation: covered by Tasks 5 and 7.
- staged runtime activation + rollback: covered by Task 5.
- diagnostics bundle + settings entry: covered by Tasks 6 and 8.
- rollout verification and bindings regeneration: covered by Task 10.

### Placeholder scan

- No `TBD`, `TODO`, or “implement later” markers remain.
- Every task includes exact file paths, concrete commands, and code snippets.

### Type consistency

- Backend bridge names are consistent across tasks:
  - `GetDesktopEnvironmentStatus`
  - `RepairDesktopEnvironment`
  - `CheckDesktopReleaseUpdate`
  - `ApplyDesktopReleaseUpdate`
  - `ExportDesktopEnvironmentDiagnostics`
- Frontend runtime API names are consistent across tasks:
  - `getDesktopEnvironmentStatus`
  - `repairEnvironmentNow`
  - `exportEnvironmentDiagnostics`
