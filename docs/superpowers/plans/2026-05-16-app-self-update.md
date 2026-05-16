# Application Self-Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build cross-platform-shaped application self-update with Windows user-mode install support, full-package payloads, rollback, diagnostics, frontend UX, and release tooling.

**Architecture:** Add a new `backend/internal/appupdate` shared core for manifest parsing, source resolution, staging, state, download, archive validation, and update classification. Implement Windows Phase 1 through a platform backend and `ant-chrome.exe --apply-update` / `--post-update-check` CLI modes while keeping macOS behind the same backend interface for future implementation.

**Tech Stack:** Go 1.22, Wails v2 bindings, React + TypeScript + Zustand, PowerShell publish scripts, NSIS packaging, standard-library zip/hash/filesystem APIs.

---

## File Structure

Create these backend shared-core files:

- `backend/internal/appupdate/manifest.go`: manifest types, loader, target package selection, version comparison.
- `backend/internal/appupdate/source.go`: manifest source resolution from runtime config, env, and `config.yaml`.
- `backend/internal/appupdate/layout.go`: `stateRoot/app-update` directory paths.
- `backend/internal/appupdate/state.go`: persistent state model and atomic read/write helpers.
- `backend/internal/appupdate/plan.go`: apply plan model and read/write helpers.
- `backend/internal/appupdate/download.go`: HTTP(S), `file://`, and local path payload download with size/hash verification.
- `backend/internal/appupdate/archive.go`: full zip extraction and staged payload validation.
- `backend/internal/appupdate/platform.go`: platform backend interface, shared errors, fake backend test helper.
- `backend/internal/appupdate/manager.go`: orchestration for check, download, stage, apply kickoff, startup recovery.
- `backend/internal/appupdate/windows_backend.go`: Windows install validation, runner preparation, backup, replacement, rollback, and post-check.

Create or modify these app integration files:

- `backend/app_update.go`: Wails-facing app update methods.
- `backend/app_release_runtime.go`: diagnostics additions only.
- `backend/internal/config/config.go`: `release.app_update_manifest_url` config field.
- `backend/internal/config/config_test.go`: config normalization tests.
- `main.go`: CLI dispatch before Wails startup.
- `frontend/src/modules/appUpdate/types.ts`: app-update frontend types.
- `frontend/src/modules/appUpdate/api.ts`: Wails binding wrapper.
- `frontend/src/store/appUpdateStore.ts`: app-update state and actions.
- `frontend/src/modules/appUpdate/components/AppUpdatePromptModal.tsx`: app update modal.
- `frontend/src/App.tsx`: bootstrap and modal integration.
- `frontend/src/modules/runtime/pages/EnvironmentGatePage.tsx`: required app update blocking state, if needed after frontend wiring.

Create or modify release tooling:

- `tools/app-update/verify-app-update-package.py`: verify app-update zip and manifest contract.
- `bat/publish.ps1`: generate full zip and app-update manifest after Windows staging.
- `publish/installer.nsi`: default install dir change to user-writable location for new installs.
- `docs/release/windows-packaging-and-update-runbook.md`: app self-update section.

---

### Task 1: Manifest Model And Version Classification

**Files:**
- Create: `backend/internal/appupdate/manifest.go`
- Create: `backend/internal/appupdate/manifest_test.go`

- [ ] **Step 1: Write failing manifest tests**

Create `backend/internal/appupdate/manifest_test.go`:

```go
package appupdate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeManifest(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app-update.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func TestLoadManifestAcceptsSchemaOne(t *testing.T) {
	path := writeManifest(t, `{
		"schemaVersion": 1,
		"channel": "stable",
		"version": "1.2.0",
		"minimumRuntimeResourceVersion": "2026.05.16",
		"minimumAppVersion": "1.1.0",
		"packages": [
			{"target":"windows-amd64","payloadType":"full","url":"file:///tmp/app.zip","sha256":"abc","size":12}
		]
	}`)

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest returned error: %v", err)
	}
	if manifest.Version != "1.2.0" {
		t.Fatalf("unexpected version: %q", manifest.Version)
	}
}

func TestLoadManifestRejectsUnsupportedSchema(t *testing.T) {
	path := writeManifest(t, `{"schemaVersion":2}`)
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected unsupported schema error")
	}
}

func TestPackageForTargetSelectsFullPayload(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: 1,
		Version:       "1.2.0",
		Packages: []Package{
			{Target: "darwin-arm64", PayloadType: PayloadTypeFull, URL: "file:///tmp/mac.zip", SHA256: "mac"},
			{Target: "windows-amd64", PayloadType: PayloadTypeFull, URL: "file:///tmp/win.zip", SHA256: "win"},
		},
	}

	pkg, err := manifest.PackageForTarget("windows-amd64")
	if err != nil {
		t.Fatalf("PackageForTarget returned error: %v", err)
	}
	if pkg.SHA256 != "win" {
		t.Fatalf("selected wrong package: %+v", pkg)
	}
}

func TestPackageForTargetRejectsDeltaInPhaseOne(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: 1,
		Version:       "1.2.0",
		Packages: []Package{
			{Target: "windows-amd64", PayloadType: "delta", URL: "file:///tmp/win.patch", SHA256: "patch"},
		},
	}

	if _, err := manifest.PackageForTarget("windows-amd64"); err == nil {
		t.Fatal("expected unsupported payload type error")
	}
}

func TestClassifyUsesRemoteAndMinimumAppVersions(t *testing.T) {
	state := Classify("1.1.0", Manifest{Version: "1.2.0", MinimumAppVersion: "1.1.0"})
	if state.Kind != KindSoft {
		t.Fatalf("expected soft update, got %s", state.Kind)
	}

	state = Classify("1.0.9", Manifest{Version: "1.2.0", MinimumAppVersion: "1.1.0"})
	if state.Kind != KindRequired {
		t.Fatalf("expected required update, got %s", state.Kind)
	}

	state = Classify("1.2.0", Manifest{Version: "1.2.0", MinimumAppVersion: "1.1.0"})
	if state.Kind != KindNone {
		t.Fatalf("expected no update, got %s", state.Kind)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestLoadManifest|TestPackageForTarget|TestClassify' -count=1
```

Expected: FAIL because `backend/internal/appupdate` does not exist.

- [ ] **Step 3: Implement manifest core**

Create `backend/internal/appupdate/manifest.go`:

```go
package appupdate

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const PayloadTypeFull = "full"

type UpdateKind string

const (
	KindNone               UpdateKind = "none"
	KindSoft               UpdateKind = "soft"
	KindRequired           UpdateKind = "required"
	KindUnsupportedInstall UpdateKind = "unsupported_install"
	KindFailed             UpdateKind = "failed"
)

type Manifest struct {
	SchemaVersion                 int       `json:"schemaVersion"`
	Channel                       string    `json:"channel"`
	Version                       string    `json:"version"`
	MinimumRuntimeResourceVersion string    `json:"minimumRuntimeResourceVersion"`
	MinimumAppVersion             string    `json:"minimumAppVersion"`
	PublishedAt                   string    `json:"publishedAt"`
	Notes                         []string  `json:"notes,omitempty"`
	Packages                      []Package `json:"packages"`
}

type Package struct {
	Target      string `json:"target"`
	PayloadType string `json:"payloadType"`
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size,omitempty"`
}

type State struct {
	Kind                          UpdateKind        `json:"kind"`
	Status                        PersistentStatus  `json:"status,omitempty"`
	LocalAppVersion               string            `json:"localAppVersion"`
	RemoteAppVersion              string            `json:"remoteAppVersion"`
	MinimumRuntimeResourceVersion string            `json:"minimumRuntimeResourceVersion,omitempty"`
	ManifestSource                string            `json:"manifestSource,omitempty"`
	ManifestURL                   string            `json:"manifestUrl,omitempty"`
	PayloadURL                    string            `json:"payloadUrl,omitempty"`
	Target                        string            `json:"target,omitempty"`
	Notes                         []string          `json:"notes,omitempty"`
	ErrorCode                     string            `json:"errorCode,omitempty"`
	ErrorMessage                  string            `json:"errorMessage,omitempty"`
	Details                       map[string]string `json:"details,omitempty"`
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != 1 {
		return Manifest{}, fmt.Errorf("unsupported app update manifest schema: %d", manifest.SchemaVersion)
	}
	manifest.Channel = strings.TrimSpace(manifest.Channel)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.MinimumRuntimeResourceVersion = strings.TrimSpace(manifest.MinimumRuntimeResourceVersion)
	manifest.MinimumAppVersion = strings.TrimSpace(manifest.MinimumAppVersion)
	return manifest, nil
}

func (m Manifest) PackageForTarget(target string) (Package, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return Package{}, fmt.Errorf("target is required")
	}
	for _, pkg := range m.Packages {
		if !strings.EqualFold(strings.TrimSpace(pkg.Target), target) {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(pkg.PayloadType), PayloadTypeFull) {
			return Package{}, fmt.Errorf("unsupported app update payload type for %s: %s", target, pkg.PayloadType)
		}
		pkg.Target = strings.TrimSpace(pkg.Target)
		pkg.PayloadType = PayloadTypeFull
		pkg.URL = strings.TrimSpace(pkg.URL)
		pkg.SHA256 = strings.ToLower(strings.TrimSpace(pkg.SHA256))
		return pkg, nil
	}
	return Package{}, fmt.Errorf("no app update package for target %s", target)
}

func Classify(localVersion string, manifest Manifest) State {
	localVersion = strings.TrimSpace(localVersion)
	remoteVersion := strings.TrimSpace(manifest.Version)
	state := State{
		Kind:                          KindNone,
		LocalAppVersion:               localVersion,
		RemoteAppVersion:              remoteVersion,
		MinimumRuntimeResourceVersion: strings.TrimSpace(manifest.MinimumRuntimeResourceVersion),
		Notes:                         append([]string{}, manifest.Notes...),
	}
	if remoteVersion == "" || localVersion == "" {
		return state
	}
	if compareVersion(localVersion, strings.TrimSpace(manifest.MinimumAppVersion)) < 0 {
		state.Kind = KindRequired
		return state
	}
	if compareVersion(localVersion, remoteVersion) < 0 {
		state.Kind = KindSoft
	}
	return state
}

func compareVersion(a, b string) int {
	av, okA := parseVersion(a)
	bv, okB := parseVersion(b)
	if !okA || !okB {
		return 0
	}
	maxLen := len(av)
	if len(bv) > maxLen {
		maxLen = len(bv)
	}
	for i := 0; i < maxLen; i++ {
		left := 0
		right := 0
		if i < len(av) {
			left = av[i]
		}
		if i < len(bv) {
			right = bv[i]
		}
		if left < right {
			return -1
		}
		if left > right {
			return 1
		}
	}
	return 0
}

func parseVersion(version string) ([]int, bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, false
	}
	version = strings.Split(version, "-")[0]
	version = strings.Split(version, "+")[0]
	parts := strings.Split(version, ".")
	out := make([]int, len(parts))
	for i, part := range parts {
		if part == "" {
			return nil, false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil, false
		}
		out[i] = n
	}
	return out, true
}
```

- [ ] **Step 4: Run tests and verify pass**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestLoadManifest|TestPackageForTarget|TestClassify' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add backend/internal/appupdate/manifest.go backend/internal/appupdate/manifest_test.go
rtk git commit -m "Add app update manifest model"
```

---

### Task 2: Source Resolution And Release Config Field

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Create: `backend/internal/appupdate/source.go`
- Create: `backend/internal/appupdate/source_test.go`
- Modify: `config.yaml`
- Modify: `publish/config.init.yaml`

- [ ] **Step 1: Write failing config and source tests**

Append to `backend/internal/config/config_test.go`:

```go
func TestReleaseAppUpdateManifestURLDefaultsEmpty(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Release.AppUpdateManifestURL != "" {
		t.Fatalf("Release.AppUpdateManifestURL default should be empty: got=%q", cfg.Release.AppUpdateManifestURL)
	}
}

func TestReleaseAppUpdateManifestURLIsLoaded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := []byte("release:\n  app_update_manifest_url: https://updates.example.com/app-update-stable.json\n")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Release.AppUpdateManifestURL != "https://updates.example.com/app-update-stable.json" {
		t.Fatalf("Release.AppUpdateManifestURL not loaded: got=%q", cfg.Release.AppUpdateManifestURL)
	}
}
```

Create `backend/internal/appupdate/source_test.go`:

```go
package appupdate

import (
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestResolveManifestSourcePrefersRuntimeConfig(t *testing.T) {
	runtimeDir := t.TempDir()
	configDir := filepath.Join(runtimeDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "app-update.json"), []byte(`{"manifestUrl":"file:///runtime/app-update.json"}`), 0o600); err != nil {
		t.Fatalf("write runtime config: %v", err)
	}
	t.Setenv("DESKTOP_APP_UPDATE_MANIFEST_URL", "file:///env/app-update.json")

	resolution := ResolveManifestSource(runtimeDir, &config.Config{Release: config.ReleaseConfig{AppUpdateManifestURL: "file:///config/app-update.json"}})
	if resolution.Source != "runtime-config" || resolution.URL != "file:///runtime/app-update.json" {
		t.Fatalf("unexpected resolution: %+v", resolution)
	}
}

func TestResolveManifestSourceUsesEnvBeforeConfig(t *testing.T) {
	t.Setenv("DESKTOP_APP_UPDATE_MANIFEST_URL", "file:///env/app-update.json")
	resolution := ResolveManifestSource(t.TempDir(), &config.Config{Release: config.ReleaseConfig{AppUpdateManifestURL: "file:///config/app-update.json"}})
	if resolution.Source != "env:DESKTOP_APP_UPDATE_MANIFEST_URL" || resolution.URL != "file:///env/app-update.json" {
		t.Fatalf("unexpected resolution: %+v", resolution)
	}
}

func TestResolveManifestSourceUsesConfig(t *testing.T) {
	resolution := ResolveManifestSource(t.TempDir(), &config.Config{Release: config.ReleaseConfig{AppUpdateManifestURL: "file:///config/app-update.json"}})
	if resolution.Source != "config.yaml" || resolution.URL != "file:///config/app-update.json" {
		t.Fatalf("unexpected resolution: %+v", resolution)
	}
}

func TestLoadManifestFromSourceSupportsHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"schemaVersion":1,"version":"1.2.0","packages":[]}`))
	}))
	defer server.Close()

	manifest, err := LoadManifestFromSource(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("LoadManifestFromSource returned error: %v", err)
	}
	if manifest.Version != "1.2.0" {
		t.Fatalf("unexpected version: %q", manifest.Version)
	}
}

func TestLoadManifestFromSourceSupportsFileURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app-update.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"version":"1.2.0","packages":[]}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	manifest, err := LoadManifestFromSource(context.Background(), "file://"+filepath.ToSlash(path))
	if err != nil {
		t.Fatalf("LoadManifestFromSource returned error: %v", err)
	}
	if manifest.Version != "1.2.0" {
		t.Fatalf("unexpected version: %q", manifest.Version)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
rtk go test ./backend/internal/config ./backend/internal/appupdate -run 'TestReleaseAppUpdate|TestResolveManifestSource|TestLoadManifestFromSource' -count=1
```

Expected: FAIL because `AppUpdateManifestURL`, `ResolveManifestSource`, and `LoadManifestFromSource` do not exist.

- [ ] **Step 3: Add config field and source resolver**

Modify `backend/internal/config/config.go`:

```go
type ReleaseConfig struct {
	UpdateManifestURL    string `yaml:"update_manifest_url"`
	AppUpdateManifestURL string `yaml:"app_update_manifest_url"`
}
```

Add near the end of `normalizeConfig`:

```go
config.Release.UpdateManifestURL = strings.TrimSpace(config.Release.UpdateManifestURL)
config.Release.AppUpdateManifestURL = strings.TrimSpace(config.Release.AppUpdateManifestURL)
```

Create `backend/internal/appupdate/source.go`:

```go
package appupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"ant-chrome/backend/internal/config"
)

type ManifestSourceResolution struct {
	URL        string
	Source     string
	ConfigPath string
}

type manifestSourceFile struct {
	ManifestURL string `json:"manifestUrl"`
}

func ResolveManifestSource(runtimeDir string, cfg *config.Config) ManifestSourceResolution {
	configPath := filepath.Join(strings.TrimSpace(runtimeDir), "config", "app-update.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var fileConfig manifestSourceFile
		if json.Unmarshal(data, &fileConfig) == nil {
			if url := strings.TrimSpace(fileConfig.ManifestURL); url != "" {
				return ManifestSourceResolution{URL: url, Source: "runtime-config", ConfigPath: configPath}
			}
		}
	}
	if value := strings.TrimSpace(os.Getenv("DESKTOP_APP_UPDATE_MANIFEST_URL")); value != "" {
		return ManifestSourceResolution{URL: value, Source: "env:DESKTOP_APP_UPDATE_MANIFEST_URL"}
	}
	if cfg != nil {
		if value := strings.TrimSpace(cfg.Release.AppUpdateManifestURL); value != "" {
			return ManifestSourceResolution{URL: value, Source: "config.yaml"}
		}
	}
	return ManifestSourceResolution{}
}

func LoadManifestFromSource(ctx context.Context, source string) (Manifest, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return Manifest{}, fmt.Errorf("app update manifest source is required")
	}
	parsed, err := url.Parse(source)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return Manifest{}, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return Manifest{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return Manifest{}, fmt.Errorf("load app update manifest failed with status %s", resp.Status)
		}
		return decodeManifest(resp.Body)
	}
	if err == nil && parsed.Scheme == "file" {
		return LoadManifest(parsed.Path)
	}
	return LoadManifest(source)
}

func decodeManifest(reader io.Reader) (Manifest, error) {
	var manifest Manifest
	if err := json.NewDecoder(reader).Decode(&manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != 1 {
		return Manifest{}, fmt.Errorf("unsupported app update manifest schema: %d", manifest.SchemaVersion)
	}
	return manifest, nil
}
```

Modify `config.yaml` and `publish/config.init.yaml` under `release:`:

```yaml
release:
  update_manifest_url: ""
  app_update_manifest_url: ""
```

- [ ] **Step 4: Run tests and verify pass**

Run:

```bash
rtk go test ./backend/internal/config ./backend/internal/appupdate -run 'TestReleaseAppUpdate|TestResolveManifestSource|TestLoadManifestFromSource' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add backend/internal/config/config.go backend/internal/config/config_test.go backend/internal/appupdate/source.go backend/internal/appupdate/source_test.go config.yaml publish/config.init.yaml
rtk git commit -m "Add app update manifest source resolution"
```

---

### Task 3: Layout, Persistent State, And Apply Plan

**Files:**
- Create: `backend/internal/appupdate/layout.go`
- Create: `backend/internal/appupdate/state.go`
- Create: `backend/internal/appupdate/plan.go`
- Create: `backend/internal/appupdate/state_test.go`

- [ ] **Step 1: Write failing state and plan tests**

Create `backend/internal/appupdate/state_test.go`:

```go
package appupdate

import (
	"path/filepath"
	"testing"
)

func TestLayoutPaths(t *testing.T) {
	layout := NewLayout("/install/root", "/state/root")
	if layout.Root() != filepath.Join("/state/root", "app-update") {
		t.Fatalf("unexpected root: %s", layout.Root())
	}
	if layout.StatePath() != filepath.Join("/state/root", "app-update", "state.json") {
		t.Fatalf("unexpected state path: %s", layout.StatePath())
	}
}

func TestWriteAndReadPersistentState(t *testing.T) {
	layout := NewLayout(filepath.Join(t.TempDir(), "install"), t.TempDir())
	state := PersistentState{
		Status:           StatusStaged,
		LocalAppVersion:  "1.1.0",
		RemoteAppVersion: "1.2.0",
		LastError:        ErrorInfo{Code: "APP-UPDATE-CHECKSUM-MISMATCH", Message: "hash mismatch"},
	}
	if err := WriteState(layout, state); err != nil {
		t.Fatalf("WriteState returned error: %v", err)
	}
	got, err := ReadState(layout)
	if err != nil {
		t.Fatalf("ReadState returned error: %v", err)
	}
	if got.Status != StatusStaged || got.LastError.Code != "APP-UPDATE-CHECKSUM-MISMATCH" {
		t.Fatalf("unexpected state: %+v", got)
	}
}

func TestWriteAndReadApplyPlan(t *testing.T) {
	layout := NewLayout(filepath.Join(t.TempDir(), "install"), t.TempDir())
	plan := ApplyPlan{
		InstallRoot:     layout.InstallRoot,
		StateRoot:       layout.StateRoot,
		Target:          "windows-amd64",
		OldAppVersion:   "1.1.0",
		NewAppVersion:   "1.2.0",
		StagedPath:      filepath.Join(layout.StagingRoot(), "1.2.0"),
		BackupPath:      filepath.Join(layout.BackupsRoot(), "1.1.0-20260516"),
		CurrentExePath:  filepath.Join(layout.InstallRoot, "ant-chrome.exe"),
		ExpectedSHA256:  "abc",
		ManifestSource:  "runtime-config",
		ManifestURL:     "file:///manifest.json",
		WaitForProcessID: 1234,
	}
	path, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}
	got, err := ReadPlan(path)
	if err != nil {
		t.Fatalf("ReadPlan returned error: %v", err)
	}
	if got.NewAppVersion != "1.2.0" || got.WaitForProcessID != 1234 {
		t.Fatalf("unexpected plan: %+v", got)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestLayoutPaths|TestWriteAndRead' -count=1
```

Expected: FAIL because layout, state, and plan helpers do not exist.

- [ ] **Step 3: Implement layout, state, and plan helpers**

Create `backend/internal/appupdate/layout.go`:

```go
package appupdate

import "path/filepath"

type Layout struct {
	InstallRoot string
	StateRoot   string
}

func NewLayout(installRoot, stateRoot string) Layout {
	return Layout{InstallRoot: filepath.Clean(installRoot), StateRoot: filepath.Clean(stateRoot)}
}

func (l Layout) Root() string {
	return filepath.Join(l.StateRoot, "app-update")
}

func (l Layout) StatePath() string {
	return filepath.Join(l.Root(), "state.json")
}

func (l Layout) PlanPath() string {
	return filepath.Join(l.Root(), "update-plan.json")
}

func (l Layout) DownloadsRoot() string {
	return filepath.Join(l.Root(), "downloads")
}

func (l Layout) StagingRoot() string {
	return filepath.Join(l.Root(), "staging")
}

func (l Layout) BackupsRoot() string {
	return filepath.Join(l.Root(), "backups")
}

func (l Layout) RunnerRoot() string {
	return filepath.Join(l.Root(), "runner")
}

func (l Layout) LogsRoot() string {
	return filepath.Join(l.Root(), "logs")
}
```

Create `backend/internal/appupdate/state.go`:

```go
package appupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type PersistentStatus string

const (
	StatusIdle               PersistentStatus = "idle"
	StatusAvailable          PersistentStatus = "available"
	StatusDownloading        PersistentStatus = "downloading"
	StatusStaged             PersistentStatus = "staged"
	StatusApplying           PersistentStatus = "applying"
	StatusVerifying          PersistentStatus = "verifying"
	StatusSucceeded          PersistentStatus = "succeeded"
	StatusRolledBack         PersistentStatus = "rolled_back"
	StatusFailedManualRepair PersistentStatus = "failed_manual_repair"
)

type ErrorInfo struct {
	Code    string            `json:"code,omitempty"`
	Message string            `json:"message,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

type PersistentState struct {
	Status           PersistentStatus `json:"status"`
	LocalAppVersion  string           `json:"localAppVersion,omitempty"`
	RemoteAppVersion string           `json:"remoteAppVersion,omitempty"`
	ManifestSource   string           `json:"manifestSource,omitempty"`
	ManifestURL      string           `json:"manifestUrl,omitempty"`
	PayloadURL       string           `json:"payloadUrl,omitempty"`
	Target           string           `json:"target,omitempty"`
	PlanPath         string           `json:"planPath,omitempty"`
	LogPath          string           `json:"logPath,omitempty"`
	BackupPath       string           `json:"backupPath,omitempty"`
	LastError        ErrorInfo        `json:"lastError,omitempty"`
	UpdatedAt        string           `json:"updatedAt"`
}

func WriteState(layout Layout, state PersistentState) error {
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(layout.StatePath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(layout.StatePath(), append(data, '\n'), 0o600)
}

func ReadState(layout Layout) (PersistentState, error) {
	data, err := os.ReadFile(layout.StatePath())
	if err != nil {
		return PersistentState{}, err
	}
	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return PersistentState{}, err
	}
	return state, nil
}
```

Create `backend/internal/appupdate/plan.go`:

```go
package appupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ApplyPlan struct {
	InstallRoot      string `json:"installRoot"`
	StateRoot        string `json:"stateRoot"`
	Target           string `json:"target"`
	OldAppVersion    string `json:"oldAppVersion"`
	NewAppVersion    string `json:"newAppVersion"`
	StagedPath       string `json:"stagedPath"`
	BackupPath       string `json:"backupPath"`
	CurrentExePath   string `json:"currentExePath"`
	ExpectedSHA256   string `json:"expectedSha256"`
	ManifestSource   string `json:"manifestSource"`
	ManifestURL      string `json:"manifestUrl"`
	PayloadURL       string `json:"payloadUrl"`
	WaitForProcessID int    `json:"waitForProcessId"`
}

func WritePlan(layout Layout, plan ApplyPlan) (string, error) {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return "", err
	}
	path := layout.PlanPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func ReadPlan(path string) (ApplyPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ApplyPlan{}, err
	}
	var plan ApplyPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return ApplyPlan{}, err
	}
	return plan, nil
}
```

- [ ] **Step 4: Run tests and verify pass**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestLayoutPaths|TestWriteAndRead' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add backend/internal/appupdate/layout.go backend/internal/appupdate/state.go backend/internal/appupdate/plan.go backend/internal/appupdate/state_test.go
rtk git commit -m "Add app update state persistence"
```

---

### Task 4: Download, Hash Verification, And Zip Staging

**Files:**
- Create: `backend/internal/appupdate/download.go`
- Create: `backend/internal/appupdate/archive.go`
- Create: `backend/internal/appupdate/download_test.go`
- Create: `backend/internal/appupdate/archive_test.go`

- [ ] **Step 1: Write failing download and archive tests**

Create `backend/internal/appupdate/download_test.go`:

```go
package appupdate

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadPayloadFromHTTPVerifiesHashAndSize(t *testing.T) {
	body := []byte("payload")
	sum := fmt.Sprintf("%x", sha256.Sum256(body))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	dst, err := DownloadPayload(t.Context(), server.URL, t.TempDir(), sum, int64(len(body)))
	if err != nil {
		t.Fatalf("DownloadPayload returned error: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read downloaded payload: %v", err)
	}
	if string(data) != "payload" {
		t.Fatalf("unexpected payload: %q", string(data))
	}
}

func TestDownloadPayloadRejectsHashMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.zip")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	if _, err := DownloadPayload(t.Context(), path, t.TempDir(), "bad-hash", 0); err == nil {
		t.Fatal("expected hash mismatch")
	}
}
```

Create `backend/internal/appupdate/archive_test.go`:

```go
package appupdate

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(file)
	for name, body := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
	return path
}

func TestExtractFullPayloadRejectsZipSlip(t *testing.T) {
	zipPath := writeZip(t, map[string]string{"../escape.txt": "bad"})
	if err := ExtractFullPayload(zipPath, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("expected zip slip rejection")
	}
}

func TestValidateStagedWindowsPayloadRequiresCoreFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "publish"), 0o755); err != nil {
		t.Fatalf("mkdir publish: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ant-chrome.exe"), []byte("MZ"), 0o600); err != nil {
		t.Fatalf("write exe: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "publish", "runtime-manifest.json"), []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := ValidateStagedPayload("windows-amd64", dir); err != nil {
		t.Fatalf("ValidateStagedPayload returned error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestDownloadPayload|TestExtractFullPayload|TestValidateStaged' -count=1
```

Expected: FAIL because download and archive helpers do not exist.

- [ ] **Step 3: Implement download and archive helpers**

Create `backend/internal/appupdate/download.go`:

```go
package appupdate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func DownloadPayload(ctx context.Context, source, downloadsDir, expectedSHA256 string, expectedSize int64) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("payload source is required")
	}
	if err := os.MkdirAll(downloadsDir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(downloadsDir, "payload.zip")
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()

	in, err := openPayloadSource(ctx, source)
	if err != nil {
		return "", err
	}
	defer in.Close()

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hash), in)
	if err != nil {
		return "", err
	}
	if expectedSize > 0 && written != expectedSize {
		return "", fmt.Errorf("payload size mismatch: expected %d, got %d", expectedSize, written)
	}
	actual := fmt.Sprintf("%x", hash.Sum(nil))
	if expected := strings.ToLower(strings.TrimSpace(expectedSHA256)); expected != "" && actual != expected {
		return "", fmt.Errorf("payload sha256 mismatch: expected %s, got %s", expected, actual)
	}
	return dst, nil
}

func openPayloadSource(ctx context.Context, source string) (io.ReadCloser, error) {
	parsed, err := url.Parse(source)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("download failed with status %s", resp.Status)
		}
		return resp.Body, nil
	}
	if err == nil && parsed.Scheme == "file" {
		return os.Open(parsed.Path)
	}
	return os.Open(source)
}
```

Create `backend/internal/appupdate/archive.go`:

```go
package appupdate

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ExtractFullPayload(zipPath, destination string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}

	for _, file := range reader.File {
		target := filepath.Join(destination, filepath.FromSlash(file.Name))
		rel, err := filepath.Rel(destination, target)
		if err != nil {
			return err
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("zip entry escapes destination: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		_ = in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func ValidateStagedPayload(target, stagedRoot string) error {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "windows-amd64":
		required := []string{
			"ant-chrome.exe",
			filepath.Join("publish", "runtime-manifest.json"),
		}
		for _, rel := range required {
			if info, err := os.Stat(filepath.Join(stagedRoot, rel)); err != nil || info.IsDir() {
				return fmt.Errorf("staged payload missing required file: %s", rel)
			}
		}
		return nil
	case "darwin-arm64", "darwin-amd64":
		return fmt.Errorf("macOS app update backend is not implemented in Phase 1")
	default:
		return fmt.Errorf("unsupported app update target: %s", target)
	}
}
```

- [ ] **Step 4: Run tests and verify pass**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestDownloadPayload|TestExtractFullPayload|TestValidateStaged' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add backend/internal/appupdate/download.go backend/internal/appupdate/archive.go backend/internal/appupdate/download_test.go backend/internal/appupdate/archive_test.go
rtk git commit -m "Add app update payload staging"
```

---

### Task 5: Shared Manager With Fake Platform Backend

**Files:**
- Create: `backend/internal/appupdate/platform.go`
- Create: `backend/internal/appupdate/manager.go`
- Create: `backend/internal/appupdate/manager_test.go`

- [ ] **Step 1: Write failing manager tests**

Create `backend/internal/appupdate/manager_test.go`:

```go
package appupdate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type fakePlatform struct {
	target         string
	validateError error
	spawned       bool
}

func (f *fakePlatform) Target() string { return f.target }
func (f *fakePlatform) ValidateInstallMode(Layout) error { return f.validateError }
func (f *fakePlatform) PrepareApply(ApplyPlan) error { return nil }
func (f *fakePlatform) SpawnApplyRunner(string) error { f.spawned = true; return nil }
func (f *fakePlatform) RunApply(string) error { return nil }
func (f *fakePlatform) PostUpdateCheck(string) error { return nil }

func TestManagerCheckClassifiesUnsupportedInstall(t *testing.T) {
	manager := Manager{
		LocalAppVersion: "1.1.0",
		Layout:          NewLayout(filepath.Join(t.TempDir(), "install"), t.TempDir()),
		Platform:        &fakePlatform{target: "windows-amd64", validateError: ErrUnsupportedInstall},
		ManifestProvider: func(context.Context) (Manifest, ManifestSourceResolution, error) {
			return Manifest{SchemaVersion: 1, Version: "1.2.0", Packages: []Package{{Target: "windows-amd64", PayloadType: PayloadTypeFull, URL: "file:///tmp/app.zip", SHA256: "abc"}}}, ManifestSourceResolution{Source: "override", URL: "file:///manifest.json"}, nil
		},
	}

	state, err := manager.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if state.Kind != KindUnsupportedInstall || state.ErrorCode != "APP-UPDATE-INSTALL-UNSUPPORTED" {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestManagerApplySpawnsRunnerForStagedUpdate(t *testing.T) {
	stateRoot := t.TempDir()
	installRoot := filepath.Join(t.TempDir(), "install")
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatalf("mkdir install: %v", err)
	}
	layout := NewLayout(installRoot, stateRoot)
	staged := filepath.Join(layout.StagingRoot(), "1.2.0")
	if err := os.MkdirAll(filepath.Join(staged, "publish"), 0o755); err != nil {
		t.Fatalf("mkdir staged: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staged, "ant-chrome.exe"), []byte("MZ"), 0o600); err != nil {
		t.Fatalf("write exe: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staged, "publish", "runtime-manifest.json"), []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write runtime manifest: %v", err)
	}
	platform := &fakePlatform{target: "windows-amd64"}
	manager := Manager{
		LocalAppVersion: "1.1.0",
		Layout:          layout,
		Platform:        platform,
	}
	if err := WriteState(layout, PersistentState{Status: StatusStaged, RemoteAppVersion: "1.2.0", Target: "windows-amd64"}); err != nil {
		t.Fatalf("write state: %v", err)
	}

	state, err := manager.Apply(context.Background())
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !platform.spawned || state.Status != StatusApplying {
		t.Fatalf("expected runner spawn and applying state, got spawned=%v state=%+v", platform.spawned, state)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestManager' -count=1
```

Expected: FAIL because `Manager`, `PlatformUpdater`, and errors do not exist.

- [ ] **Step 3: Implement platform interface and manager skeleton**

Create `backend/internal/appupdate/platform.go`:

```go
package appupdate

import "errors"

var ErrUnsupportedInstall = errors.New("unsupported app update install location")

type PlatformUpdater interface {
	Target() string
	ValidateInstallMode(Layout) error
	PrepareApply(ApplyPlan) error
	SpawnApplyRunner(planPath string) error
	RunApply(planPath string) error
	PostUpdateCheck(planPath string) error
}
```

Create `backend/internal/appupdate/manager.go`:

```go
package appupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type ManifestProvider func(context.Context) (Manifest, ManifestSourceResolution, error)

type Manager struct {
	LocalAppVersion string
	Layout          Layout
	Platform        PlatformUpdater
	ManifestProvider ManifestProvider
}

func (m Manager) Check(ctx context.Context) (State, error) {
	manifest, resolution, err := m.ManifestProvider(ctx)
	if err != nil {
		return State{Kind: KindFailed, ErrorCode: "APP-UPDATE-MANIFEST-LOAD-FAILED", ErrorMessage: err.Error()}, nil
	}
	state := Classify(m.LocalAppVersion, manifest)
	state.ManifestSource = resolution.Source
	state.ManifestURL = resolution.URL
	state.Target = m.Platform.Target()
	pkg, pkgErr := manifest.PackageForTarget(m.Platform.Target())
	if pkgErr != nil {
		state.Kind = KindFailed
		state.ErrorCode = "APP-UPDATE-TARGET-MISSING"
		state.ErrorMessage = pkgErr.Error()
		return state, nil
	}
	state.PayloadURL = pkg.URL
	if err := m.Platform.ValidateInstallMode(m.Layout); err != nil {
		state.Kind = KindUnsupportedInstall
		state.ErrorCode = "APP-UPDATE-INSTALL-UNSUPPORTED"
		state.ErrorMessage = err.Error()
		return state, nil
	}
	return state, nil
}

func (m Manager) Apply(ctx context.Context) (State, error) {
	_ = ctx
	persistent, err := ReadState(m.Layout)
	if err != nil {
		return State{}, err
	}
	if persistent.Status != StatusStaged {
		return State{}, fmt.Errorf("app update is not staged: %s", persistent.Status)
	}
	stagedPath := filepath.Join(m.Layout.StagingRoot(), persistent.RemoteAppVersion)
	backupPath := filepath.Join(m.Layout.BackupsRoot(), persistent.LocalAppVersion+"-backup")
	plan := ApplyPlan{
		InstallRoot:    m.Layout.InstallRoot,
		StateRoot:      m.Layout.StateRoot,
		Target:         persistent.Target,
		OldAppVersion:  persistent.LocalAppVersion,
		NewAppVersion:  persistent.RemoteAppVersion,
		StagedPath:     stagedPath,
		BackupPath:     backupPath,
		ManifestSource: persistent.ManifestSource,
		ManifestURL:    persistent.ManifestURL,
		PayloadURL:     persistent.PayloadURL,
	}
	if err := m.Platform.PrepareApply(plan); err != nil {
		return State{}, err
	}
	planPath, err := WritePlan(m.Layout, plan)
	if err != nil {
		return State{}, err
	}
	if err := m.Platform.SpawnApplyRunner(planPath); err != nil {
		return State{}, err
	}
	next := PersistentState{
		Status:           StatusApplying,
		LocalAppVersion:  persistent.LocalAppVersion,
		RemoteAppVersion: persistent.RemoteAppVersion,
		ManifestSource:   persistent.ManifestSource,
		ManifestURL:      persistent.ManifestURL,
		PayloadURL:       persistent.PayloadURL,
		Target:           persistent.Target,
		PlanPath:         planPath,
		BackupPath:       backupPath,
	}
	if err := WriteState(m.Layout, next); err != nil {
		return State{}, err
	}
	return State{Kind: KindSoft, Status: StatusApplying, LocalAppVersion: next.LocalAppVersion, RemoteAppVersion: next.RemoteAppVersion}, nil
}

func DefaultManifestProvider(runtimeDir string, cfgSource func() ManifestSourceResolution) ManifestProvider {
	return func(ctx context.Context) (Manifest, ManifestSourceResolution, error) {
		resolution := cfgSource()
		if resolution.URL == "" {
			return Manifest{}, resolution, os.ErrNotExist
		}
		manifest, err := LoadManifestFromSource(ctx, resolution.URL)
		if err == nil {
			return manifest, resolution, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{}, resolution, err
		}
		return Manifest{}, resolution, err
	}
}
```

- [ ] **Step 4: Run tests and verify pass**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestManager' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add backend/internal/appupdate/platform.go backend/internal/appupdate/manager.go backend/internal/appupdate/manager_test.go
rtk git commit -m "Add app update manager skeleton"
```

---

### Task 6: Windows Backend Validation, Backup, Apply, And Rollback

**Files:**
- Create: `backend/internal/appupdate/windows_backend.go`
- Create: `backend/internal/appupdate/windows_backend_test.go`

- [ ] **Step 1: Write failing Windows backend tests**

Create `backend/internal/appupdate/windows_backend_test.go`:

```go
package appupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWindowsBackendRejectsProgramFiles(t *testing.T) {
	backend := WindowsBackend{}
	layout := NewLayout(`C:\Program Files\Ant Browser`, t.TempDir())
	if runtime.GOOS != "windows" {
		layout = NewLayout(`/Program Files/Ant Browser`, t.TempDir())
	}
	if err := backend.ValidateInstallMode(layout); err == nil {
		t.Fatal("expected Program Files install to be rejected")
	}
}

func TestWindowsBackendBackupReplaceAndRollback(t *testing.T) {
	install := t.TempDir()
	state := t.TempDir()
	layout := NewLayout(install, state)
	if err := os.WriteFile(filepath.Join(install, "ant-chrome.exe"), []byte("old"), 0o600); err != nil {
		t.Fatalf("write old exe: %v", err)
	}
	staged := filepath.Join(t.TempDir(), "staged")
	if err := os.MkdirAll(staged, 0o755); err != nil {
		t.Fatalf("mkdir staged: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staged, "ant-chrome.exe"), []byte("new"), 0o600); err != nil {
		t.Fatalf("write new exe: %v", err)
	}
	plan := ApplyPlan{
		InstallRoot:   install,
		StateRoot:     state,
		Target:        "windows-amd64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
		StagedPath:    staged,
		BackupPath:    filepath.Join(layout.BackupsRoot(), "1.1.0-test"),
	}
	backend := WindowsBackend{}
	if err := backend.backupInstall(plan); err != nil {
		t.Fatalf("backupInstall returned error: %v", err)
	}
	if err := backend.replaceInstall(plan); err != nil {
		t.Fatalf("replaceInstall returned error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(install, "ant-chrome.exe"))
	if string(data) != "new" {
		t.Fatalf("expected new exe, got %q", string(data))
	}
	if err := backend.rollbackInstall(plan); err != nil {
		t.Fatalf("rollbackInstall returned error: %v", err)
	}
	data, _ = os.ReadFile(filepath.Join(install, "ant-chrome.exe"))
	if string(data) != "old" {
		t.Fatalf("expected old exe after rollback, got %q", string(data))
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestWindowsBackend' -count=1
```

Expected: FAIL because `WindowsBackend` does not exist.

- [ ] **Step 3: Implement Windows backend file operations**

Create `backend/internal/appupdate/windows_backend.go`:

```go
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
	lower := strings.ToLower(filepath.ToSlash(install))
	if strings.Contains(lower, "/program files/") || strings.HasSuffix(lower, "/program files") || strings.Contains(lower, "/program files (x86)/") {
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
	return nil
}

func (b WindowsBackend) SpawnApplyRunner(planPath string) error {
	exe := strings.TrimSpace(b.CurrentExePath)
	if exe == "" {
		var err error
		exe, err = os.Executable()
		if err != nil {
			return err
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
	if err := WriteState(layout, PersistentState{Status: StatusApplying, LocalAppVersion: plan.OldAppVersion, RemoteAppVersion: plan.NewAppVersion, PlanPath: planPath, BackupPath: plan.BackupPath}); err != nil {
		return err
	}
	if err := b.backupInstall(plan); err != nil {
		_ = WriteState(layout, PersistentState{Status: StatusFailedManualRepair, LastError: ErrorInfo{Code: "APP-UPDATE-ROLLBACK-FAILED-MANUAL-REPAIR", Message: err.Error()}})
		return err
	}
	if err := b.replaceInstall(plan); err != nil {
		_ = b.rollbackInstall(plan)
		_ = WriteState(layout, PersistentState{Status: StatusRolledBack, LastError: ErrorInfo{Code: "APP-UPDATE-APPLY-FAILED-ROLLED-BACK", Message: err.Error()}, BackupPath: plan.BackupPath})
		return err
	}
	return WriteState(layout, PersistentState{Status: StatusVerifying, LocalAppVersion: plan.OldAppVersion, RemoteAppVersion: plan.NewAppVersion, PlanPath: planPath, BackupPath: plan.BackupPath})
}

func (b WindowsBackend) PostUpdateCheck(planPath string) error {
	plan, err := ReadPlan(planPath)
	if err != nil {
		return err
	}
	layout := NewLayout(plan.InstallRoot, plan.StateRoot)
	return WriteState(layout, PersistentState{Status: StatusSucceeded, LocalAppVersion: plan.OldAppVersion, RemoteAppVersion: plan.NewAppVersion})
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
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		}
		return copyFileMode(path, target, info.Mode().Perm())
	})
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
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
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func waitForProcessExit(pid int, timeout time.Duration) {
	if runtime.GOOS != "windows" || pid <= 0 {
		return
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		process, err := os.FindProcess(pid)
		if err != nil || process.Signal(os.Signal(nil)) != nil {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}
```

- [ ] **Step 4: Run tests and verify pass**

Run:

```bash
rtk go test ./backend/internal/appupdate -run 'TestWindowsBackend' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add backend/internal/appupdate/windows_backend.go backend/internal/appupdate/windows_backend_test.go
rtk git commit -m "Add Windows app update backend"
```

---

### Task 7: CLI Entrypoints Before Wails Startup

**Files:**
- Modify: `main.go`
- Create: `main_update_cli_test.go`
- Create: `backend/app_update_cli.go`

- [ ] **Step 1: Write failing CLI argument tests**

Create `main_update_cli_test.go`:

```go
package main

import "testing"

func TestParseUpdateCLIApply(t *testing.T) {
	mode, planPath := parseUpdateCLI([]string{"ant-chrome.exe", "--apply-update", "C:/tmp/update-plan.json"})
	if mode != "apply" || planPath != "C:/tmp/update-plan.json" {
		t.Fatalf("unexpected parse result: mode=%q plan=%q", mode, planPath)
	}
}

func TestParseUpdateCLIPostCheck(t *testing.T) {
	mode, planPath := parseUpdateCLI([]string{"ant-chrome.exe", "--post-update-check", "C:/tmp/update-plan.json"})
	if mode != "post-check" || planPath != "C:/tmp/update-plan.json" {
		t.Fatalf("unexpected parse result: mode=%q plan=%q", mode, planPath)
	}
}

func TestParseUpdateCLINone(t *testing.T) {
	mode, planPath := parseUpdateCLI([]string{"ant-chrome.exe"})
	if mode != "" || planPath != "" {
		t.Fatalf("unexpected parse result: mode=%q plan=%q", mode, planPath)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
rtk go test . -run 'TestParseUpdateCLI' -count=1
```

Expected: FAIL because `parseUpdateCLI` does not exist.

- [ ] **Step 3: Implement CLI dispatch and backend entrypoints**

In `main.go`, add:

```go
func parseUpdateCLI(args []string) (mode string, planPath string) {
	if len(args) < 3 {
		return "", ""
	}
	switch strings.TrimSpace(args[1]) {
	case "--apply-update":
		return "apply", strings.TrimSpace(args[2])
	case "--post-update-check":
		return "post-check", strings.TrimSpace(args[2])
	default:
		return "", ""
	}
}
```

At the top of `main()`, before app root detection, add:

```go
if mode, planPath := parseUpdateCLI(os.Args); mode != "" {
	if err := backend.RunAppUpdateCLI(mode, planPath); err != nil {
		log.Printf("app update %s failed: %v", mode, err)
		os.Exit(1)
	}
	return
}
```

Create `backend/app_update_cli.go`:

```go
package backend

import (
	"fmt"

	"ant-chrome/backend/internal/appupdate"
)

func RunAppUpdateCLI(mode, planPath string) error {
	backend := appupdate.WindowsBackend{}
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

- [ ] **Step 4: Run tests and verify pass**

Run:

```bash
rtk go test . -run 'TestParseUpdateCLI' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add main.go main_update_cli_test.go backend/app_update_cli.go
rtk git commit -m "Add app update CLI modes"
```

---

### Task 8: Wails Backend API And Diagnostics

**Files:**
- Create: `backend/app_update.go`
- Create: `backend/app_update_test.go`
- Modify: `backend/app_release_runtime.go`
- Modify: `frontend/src/wailsjs/go/main/App.d.ts` and generated Wails files after running bindings, if the project requires checked-in bindings.

- [ ] **Step 1: Write failing backend API tests**

Create `backend/app_update_test.go`:

```go
package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDesktopAppUpdateStateReturnsIdleWhenMissing(t *testing.T) {
	app := NewApp(t.TempDir(), "1.1.0")
	state, err := app.GetDesktopAppUpdateState()
	if err != nil {
		t.Fatalf("GetDesktopAppUpdateState returned error: %v", err)
	}
	if state.Kind != "none" {
		t.Fatalf("expected none, got %+v", state)
	}
}

func TestClearDesktopAppUpdateFailureRemovesState(t *testing.T) {
	root := t.TempDir()
	app := NewApp(root, "1.1.0")
	layout := app.appUpdateLayout()
	if err := os.MkdirAll(filepath.Dir(layout.StatePath()), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(layout.StatePath(), []byte(`{"status":"failed_manual_repair"}`), 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}
	if err := app.ClearDesktopAppUpdateFailure(); err != nil {
		t.Fatalf("ClearDesktopAppUpdateFailure returned error: %v", err)
	}
	if _, err := os.Stat(layout.StatePath()); !os.IsNotExist(err) {
		t.Fatalf("expected state file removed, err=%v", err)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
rtk go test ./backend -run 'TestGetDesktopAppUpdateState|TestClearDesktopAppUpdateFailure' -count=1
```

Expected: FAIL because app update methods do not exist.

- [ ] **Step 3: Implement Wails-facing API**

Create `backend/app_update.go`:

```go
package backend

import (
	"context"
	"os"

	"ant-chrome/backend/internal/appupdate"
)

func (a *App) appUpdateLayout() appupdate.Layout {
	layout := a.runtimeLayout()
	return appupdate.NewLayout(layout.InstallRoot, layout.StateRoot)
}

func (a *App) appUpdateManager() appupdate.Manager {
	layout := a.appUpdateLayout()
	return appupdate.Manager{
		LocalAppVersion: a.appVersion(),
		Layout:          layout,
		Platform:        appupdate.WindowsBackend{ProcessID: os.Getpid()},
		ManifestProvider: func(ctx context.Context) (appupdate.Manifest, appupdate.ManifestSourceResolution, error) {
			resolution := appupdate.ResolveManifestSource(resolveWorkspaceRuntimeDirWithConfig(a.config), a.config)
			manifest, err := appupdate.LoadManifestFromSource(ctx, resolution.URL)
			return manifest, resolution, err
		},
	}
}

func (a *App) CheckDesktopAppUpdate() (appupdate.State, error) {
	return a.appUpdateManager().Check(a.ctx)
}

func (a *App) ApplyDesktopAppUpdate() (appupdate.State, error) {
	return a.appUpdateManager().Apply(a.ctx)
}

func (a *App) GetDesktopAppUpdateState() (appupdate.State, error) {
	persistent, err := appupdate.ReadState(a.appUpdateLayout())
	if os.IsNotExist(err) {
		return appupdate.State{Kind: appupdate.KindNone, LocalAppVersion: a.appVersion()}, nil
	}
	if err != nil {
		return appupdate.State{}, err
	}
	return appupdate.State{
		Kind:             appupdate.KindNone,
		Status:           persistent.Status,
		LocalAppVersion:  persistent.LocalAppVersion,
		RemoteAppVersion: persistent.RemoteAppVersion,
		ManifestSource:   persistent.ManifestSource,
		ManifestURL:      persistent.ManifestURL,
		PayloadURL:       persistent.PayloadURL,
		Target:           persistent.Target,
		ErrorCode:        persistent.LastError.Code,
		ErrorMessage:     persistent.LastError.Message,
		Details:          persistent.LastError.Details,
	}, nil
}

func (a *App) ClearDesktopAppUpdateFailure() error {
	err := os.Remove(a.appUpdateLayout().StatePath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
```

Modify `backend/app_release_runtime.go` diagnostics map to include app-update paths:

```go
"appUpdateStatePath": layoutForAppUpdate.StatePath(),
"appUpdatePlanPath":  layoutForAppUpdate.PlanPath(),
"appUpdateRoot":      layoutForAppUpdate.Root(),
```

Use a local variable before building the bundle:

```go
layoutForAppUpdate := appupdate.NewLayout(layout.InstallRoot, layout.StateRoot)
```

Add the import:

```go
"ant-chrome/backend/internal/appupdate"
```

- [ ] **Step 4: Run tests and generate Wails bindings**

Run:

```bash
rtk go test ./backend -run 'TestGetDesktopAppUpdateState|TestClearDesktopAppUpdateFailure' -count=1
rtk go test ./backend ./backend/internal/appupdate -count=1
rtk ./bat/generate-bindings.bat
```

Expected: Go tests PASS. Binding generation exits 0 on Windows; on macOS/Linux, if `.bat` cannot run, note the binding regeneration as a Windows verification step.

- [ ] **Step 5: Commit**

```bash
rtk git add backend/app_update.go backend/app_update_test.go backend/app_release_runtime.go frontend/src/wailsjs
rtk git commit -m "Expose desktop app update APIs"
```

---

### Task 9: Frontend App Update Store And Modal

**Files:**
- Create: `frontend/src/modules/appUpdate/types.ts`
- Create: `frontend/src/modules/appUpdate/api.ts`
- Create: `frontend/src/store/appUpdateStore.ts`
- Create: `frontend/src/modules/appUpdate/components/AppUpdatePromptModal.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/modules/runtime/pages/EnvironmentGatePage.tsx`

- [ ] **Step 1: Add frontend types and API wrapper**

Create `frontend/src/modules/appUpdate/types.ts`:

```ts
export type AppUpdateKind = 'none' | 'soft' | 'required' | 'unsupported_install' | 'failed'
export type AppUpdateStatus =
  | ''
  | 'idle'
  | 'available'
  | 'downloading'
  | 'staged'
  | 'applying'
  | 'verifying'
  | 'succeeded'
  | 'rolled_back'
  | 'failed_manual_repair'

export interface AppUpdateState {
  kind: AppUpdateKind
  status: AppUpdateStatus
  localAppVersion: string
  remoteAppVersion: string
  minimumRuntimeResourceVersion: string
  manifestSource: string
  manifestUrl: string
  payloadUrl: string
  target: string
  notes: string[]
  errorCode: string
  errorMessage: string
  details: Record<string, string>
}
```

Create `frontend/src/modules/appUpdate/api.ts`:

```ts
import type { AppUpdateState } from './types'

type AppUpdateBindings = {
  CheckDesktopAppUpdate?: () => Promise<any>
  ApplyDesktopAppUpdate?: () => Promise<any>
  GetDesktopAppUpdateState?: () => Promise<any>
  ClearDesktopAppUpdateFailure?: () => Promise<void>
}

async function getBindings(): Promise<AppUpdateBindings | null> {
  const fallback = ((window as any)?.go?.main?.App as AppUpdateBindings | undefined) ?? null
  try {
    const module: any = await import('../../wailsjs/go/main/App')
    return { ...(fallback || {}), ...(module as AppUpdateBindings) }
  } catch {
    return fallback
  }
}

function normalize(input: any): AppUpdateState {
  const kind = String(input?.kind || 'none') as AppUpdateState['kind']
  const status = String(input?.status || '') as AppUpdateState['status']
  return {
    kind: kind === 'soft' || kind === 'required' || kind === 'unsupported_install' || kind === 'failed' || kind === 'none' ? kind : 'none',
    status:
      status === 'idle' ||
      status === 'available' ||
      status === 'downloading' ||
      status === 'staged' ||
      status === 'applying' ||
      status === 'verifying' ||
      status === 'succeeded' ||
      status === 'rolled_back' ||
      status === 'failed_manual_repair'
        ? status
        : '',
    localAppVersion: String(input?.localAppVersion || ''),
    remoteAppVersion: String(input?.remoteAppVersion || ''),
    minimumRuntimeResourceVersion: String(input?.minimumRuntimeResourceVersion || ''),
    manifestSource: String(input?.manifestSource || ''),
    manifestUrl: String(input?.manifestUrl || ''),
    payloadUrl: String(input?.payloadUrl || ''),
    target: String(input?.target || ''),
    notes: Array.isArray(input?.notes) ? input.notes.map((item: any) => String(item)) : [],
    errorCode: String(input?.errorCode || ''),
    errorMessage: String(input?.errorMessage || ''),
    details:
      input?.details && typeof input.details === 'object'
        ? Object.fromEntries(Object.entries(input.details).map(([key, value]) => [String(key), String(value ?? '')]))
        : {},
  }
}

async function requireBinding<K extends keyof AppUpdateBindings>(name: K): Promise<NonNullable<AppUpdateBindings[K]>> {
  const bindings = await getBindings()
  const fn = bindings?.[name]
  if (!fn) {
    throw new Error(`当前环境缺少 ${String(name)} 绑定`)
  }
  return fn as NonNullable<AppUpdateBindings[K]>
}

export async function checkDesktopAppUpdate(): Promise<AppUpdateState> {
  const fn = await requireBinding('CheckDesktopAppUpdate')
  return normalize(await fn())
}

export async function applyDesktopAppUpdate(): Promise<AppUpdateState> {
  const fn = await requireBinding('ApplyDesktopAppUpdate')
  return normalize(await fn())
}

export async function getDesktopAppUpdateState(): Promise<AppUpdateState> {
  const fn = await requireBinding('GetDesktopAppUpdateState')
  return normalize(await fn())
}

export async function clearDesktopAppUpdateFailure(): Promise<void> {
  const fn = await requireBinding('ClearDesktopAppUpdateFailure')
  await fn()
}
```

- [ ] **Step 2: Add Zustand store**

Create `frontend/src/store/appUpdateStore.ts`:

```ts
import { create } from 'zustand'
import {
  applyDesktopAppUpdate,
  checkDesktopAppUpdate,
  clearDesktopAppUpdateFailure,
  getDesktopAppUpdateState,
} from '../modules/appUpdate/api'
import type { AppUpdateState } from '../modules/appUpdate/types'

const noneState: AppUpdateState = {
  kind: 'none',
  status: '',
  localAppVersion: '',
  remoteAppVersion: '',
  minimumRuntimeResourceVersion: '',
  manifestSource: '',
  manifestUrl: '',
  payloadUrl: '',
  target: '',
  notes: [],
  errorCode: '',
  errorMessage: '',
  details: {},
}

interface AppUpdateStoreState {
  state: AppUpdateState
  promptOpen: boolean
  checking: boolean
  applying: boolean
  error: string
  bootstrap: () => Promise<void>
  applyNow: () => Promise<void>
  dismiss: () => void
  clearFailure: () => Promise<void>
}

function message(error: unknown, fallback: string) {
  if (typeof error === 'string') return error.trim() || fallback
  if (error instanceof Error) return error.message.trim() || fallback
  return fallback
}

export const useAppUpdateStore = create<AppUpdateStoreState>((set, get) => ({
  state: noneState,
  promptOpen: false,
  checking: false,
  applying: false,
  error: '',
  bootstrap: async () => {
    if (get().checking || get().applying) return
    set({ checking: true, error: '' })
    try {
      const persisted = await getDesktopAppUpdateState()
      if (persisted.status === 'rolled_back' || persisted.status === 'failed_manual_repair') {
        set({ state: persisted, promptOpen: true, checking: false })
        return
      }
      const state = await checkDesktopAppUpdate()
      set({
        state,
        promptOpen: state.kind === 'soft' || state.kind === 'required' || state.kind === 'unsupported_install' || state.kind === 'failed',
        checking: false,
      })
    } catch (error) {
      set({ state: noneState, promptOpen: false, checking: false, error: message(error, '应用更新检查失败') })
    }
  },
  applyNow: async () => {
    if (get().applying) return
    set({ applying: true, error: '' })
    try {
      const state = await applyDesktopAppUpdate()
      set({ state, promptOpen: true, applying: false })
    } catch (error) {
      set({ applying: false, error: message(error, '应用更新启动失败') })
      throw error
    }
  },
  dismiss: () =>
    set((current) => {
      if (current.state.kind === 'required') return current
      return { promptOpen: false, error: '' }
    }),
  clearFailure: async () => {
    await clearDesktopAppUpdateFailure()
    set({ state: noneState, promptOpen: false, error: '' })
  },
}))
```

- [ ] **Step 3: Add modal component and app integration**

Create `frontend/src/modules/appUpdate/components/AppUpdatePromptModal.tsx`:

```tsx
import { AlertCircle, Download, RotateCcw } from 'lucide-react'
import { Button, Modal, Alert } from '../../../shared/components'
import { useAppUpdateStore } from '../../../store/appUpdateStore'

export function AppUpdatePromptModal() {
  const open = useAppUpdateStore((state) => state.promptOpen)
  const update = useAppUpdateStore((state) => state.state)
  const applying = useAppUpdateStore((state) => state.applying)
  const error = useAppUpdateStore((state) => state.error)
  const applyNow = useAppUpdateStore((state) => state.applyNow)
  const dismiss = useAppUpdateStore((state) => state.dismiss)
  const clearFailure = useAppUpdateStore((state) => state.clearFailure)

  const required = update.kind === 'required'
  const unsupported = update.kind === 'unsupported_install'
  const failed = update.status === 'rolled_back' || update.status === 'failed_manual_repair' || update.kind === 'failed'
  const title = unsupported
    ? '当前安装位置不支持自动更新'
    : failed
      ? '应用更新未完成'
      : required
        ? '需要先升级客户端'
        : '检测到客户端更新'

  return (
    <Modal open={open} onClose={required ? undefined : dismiss} title={title} width="520px" closable={!required && !applying}>
      <div className="space-y-4">
        <div className="flex gap-3">
          <div className="mt-1 flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-amber-50 text-amber-600">
            {failed ? <RotateCcw className="h-5 w-5" /> : <Download className="h-5 w-5" />}
          </div>
          <div className="space-y-2 text-sm text-[var(--color-text-secondary)]">
            {unsupported ? (
              <p>当前安装目录不可写或位于 Program Files。请迁移到用户目录安装版本后再使用自动更新。</p>
            ) : failed ? (
              <p>{update.errorMessage || '更新失败。若已回滚，当前已恢复旧版本。'}</p>
            ) : (
              <p>
                当前版本 {update.localAppVersion || '-'}，可更新到 {update.remoteAppVersion || '-'}。更新会关闭并重启应用。
              </p>
            )}
            {update.manifestSource && (
              <p className="text-xs">来源：{update.manifestSource}</p>
            )}
            {update.notes.length > 0 && (
              <ul className="list-disc pl-5 text-xs">
                {update.notes.map((note) => (
                  <li key={note}>{note}</li>
                ))}
              </ul>
            )}
          </div>
        </div>

        {error && <Alert type="error" title="更新操作失败" message={error} />}
        {update.errorCode && <Alert type="warning" title={update.errorCode} message={update.errorMessage || '请导出诊断信息。'} />}

        <div className="flex justify-end gap-2">
          {failed && (
            <Button variant="secondary" onClick={clearFailure}>
              清除失败状态
            </Button>
          )}
          {!required && !applying && (
            <Button variant="secondary" onClick={dismiss}>
              稍后再说
            </Button>
          )}
          {!unsupported && !failed && (
            <Button onClick={applyNow} loading={applying}>
              {required ? '立即更新并重启' : '下载并重启更新'}
            </Button>
          )}
        </div>
      </div>
    </Modal>
  )
}
```

Modify `frontend/src/App.tsx`:

```tsx
import { AppUpdatePromptModal } from './modules/appUpdate/components/AppUpdatePromptModal'
import { useAppUpdateStore } from './store/appUpdateStore'
```

Inside the top-level app component that already bootstraps runtime/auth, call:

```tsx
const bootstrapAppUpdate = useAppUpdateStore((state) => state.bootstrap)

useEffect(() => {
  bootstrapAppUpdate()
}, [bootstrapAppUpdate])
```

Render near existing `<UpdatePromptModal />`:

```tsx
<AppUpdatePromptModal />
```

- [ ] **Step 4: Run frontend build**

Run:

```bash
cd frontend && rtk npm run build
```

Expected: TypeScript and Vite build PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add frontend/src/modules/appUpdate frontend/src/store/appUpdateStore.ts frontend/src/App.tsx frontend/src/modules/runtime/pages/EnvironmentGatePage.tsx
rtk git commit -m "Add app update frontend prompt"
```

---

### Task 10: Publish Zip, App Manifest, And Contract Verification

**Files:**
- Create: `tools/app-update/verify-app-update-package.py`
- Modify: `bat/publish.ps1`
- Modify: `publish/installer.nsi`
- Modify: `tools/runtime/verify-publish-contract.py`

- [ ] **Step 1: Write package verifier script**

Create `tools/app-update/verify-app-update-package.py`:

```python
#!/usr/bin/env python3

from __future__ import annotations

import hashlib
import json
import sys
import zipfile
from pathlib import Path


def fail(message: str) -> None:
    print(f"[ERROR] {message}", file=sys.stderr)
    raise SystemExit(1)


def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def main() -> None:
    if len(sys.argv) != 4:
        fail("usage: verify-app-update-package.py <manifest> <zip> <target>")
    manifest_path = Path(sys.argv[1])
    zip_path = Path(sys.argv[2])
    target = sys.argv[3].strip().lower()

    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    if manifest.get("schemaVersion") != 1:
        fail("app-update manifest schemaVersion must be 1")
    packages = manifest.get("packages") or []
    package = next((p for p in packages if str(p.get("target", "")).strip().lower() == target), None)
    if package is None:
        fail(f"manifest missing package for {target}")
    if package.get("payloadType") != "full":
        fail("Phase 1 package payloadType must be full")
    expected_hash = str(package.get("sha256") or "").lower()
    actual_hash = sha256(zip_path)
    if expected_hash != actual_hash:
        fail(f"sha256 mismatch: expected {expected_hash}, got {actual_hash}")

    with zipfile.ZipFile(zip_path) as zf:
        names = set(zf.namelist())
        required = {"ant-chrome.exe", "publish/runtime-manifest.json", "bin/xray.exe", "bin/sing-box.exe"}
        missing = sorted(required - names)
        if missing:
            fail("zip missing required files: " + ", ".join(missing))
        forbidden = [name for name in names if name.startswith("data/") or name.endswith(".db") or name.endswith(".sqlite")]
        if forbidden:
            fail("zip contains mutable user data: " + ", ".join(sorted(forbidden)[:10]))
    print("[OK] app update package verified")


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Modify Windows publish script**

In `bat/publish.ps1`, add functions after `Invoke-WindowsPackaging`:

```powershell
function New-AppUpdateZip {
    param(
        [Parameter(Mandatory = $true)]
        [string]$StagingDir
    )

    $zipPath = Join-Path $repoRoot "publish/output/AntBrowser-$script:Version-windows-amd64.zip"
    if (Test-Path -LiteralPath $zipPath) {
        Remove-Item -LiteralPath $zipPath -Force
    }
    Compress-Archive -Path (Join-Path $StagingDir "*") -DestinationPath $zipPath -Force
    return $zipPath
}

function New-AppUpdateManifest {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ZipPath
    )

    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $ZipPath).Hash.ToLowerInvariant()
    $size = (Get-Item -LiteralPath $ZipPath).Length
    $manifestPath = Join-Path $repoRoot "publish/output/app-update-stable.json"
    $manifest = [ordered]@{
        schemaVersion = 1
        channel = "stable"
        version = $script:Version
        minimumRuntimeResourceVersion = $script:Version
        minimumAppVersion = $script:Version
        publishedAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
        notes = @("Ant Browser $script:Version")
        packages = @(@{
            target = "windows-amd64"
            payloadType = "full"
            url = "AntBrowser-$script:Version-windows-amd64.zip"
            sha256 = $hash
            size = $size
        })
    }
    $json = $manifest | ConvertTo-Json -Depth 20
    [System.IO.File]::WriteAllText($manifestPath, $json + "`n", [System.Text.UTF8Encoding]::new($false))
    [System.IO.File]::WriteAllText("$manifestPath.sha256", $hash + "  app-update-stable.json`n", [System.Text.UTF8Encoding]::new($false))
    return $manifestPath
}
```

In `Publish-Windows`, after `Invoke-WindowsPackaging` and before staging cleanup, add:

```powershell
$appUpdateZip = New-AppUpdateZip -StagingDir $stagingDir
$appUpdateManifest = New-AppUpdateManifest -ZipPath $appUpdateZip
Invoke-NativeCommand -FilePath "python3" -Arguments @("tools/app-update/verify-app-update-package.py", $appUpdateManifest, $appUpdateZip, "windows-amd64")
Write-Host "✓ 应用本体更新包生成成功"
```

- [ ] **Step 3: Change NSIS default install directory for new installs**

In `publish/installer.nsi`, change:

```nsi
!define INSTALL_DIR     "$PROGRAMFILES64\Ant Browser"
RequestExecutionLevel admin
```

to:

```nsi
!define INSTALL_DIR     "$LOCALAPPDATA\Programs\Ant Browser"
RequestExecutionLevel user
```

Keep `InstallDirRegKey` so existing installs preserve their recorded location.

- [ ] **Step 4: Run verifier help and publish contract**

Run:

```bash
rtk python3 tools/app-update/verify-app-update-package.py
rtk python3 tools/runtime/verify-publish-contract.py
```

Expected: first command exits non-zero with usage text; second command PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add tools/app-update/verify-app-update-package.py bat/publish.ps1 publish/installer.nsi tools/runtime/verify-publish-contract.py
rtk git commit -m "Add app update publish artifacts"
```

---

### Task 11: Runbook And Regression Documentation

**Files:**
- Modify: `docs/release/windows-packaging-and-update-runbook.md`
- Modify: `docs/reports/2026-05-16-windows-release-stability-phase-report.md`

- [ ] **Step 1: Update runbook application self-update section**

Add this section to `docs/release/windows-packaging-and-update-runbook.md` after runtime update scenarios:

```markdown
## 应用本体自更新回归场景

应用本体更新与 runtime 更新分开验证。runtime 更新只切换 `runtime/current.json`；应用本体更新会替换用户态安装目录中的 `ant-chrome.exe` 与随包 payload。

### 前置条件

1. 新安装默认目录为 `%LOCALAPPDATA%\Programs\Ant Browser`
2. `publish/output/app-update-stable.json` 存在
3. `publish/output/AntBrowser-<version>-windows-amd64.zip` 存在
4. manifest 中的 `sha256` 与 zip 文件一致

### A：soft app update success

构造规则：

- 本地 app version 小于 manifest `version`
- 本地 app version 大于等于 manifest `minimumAppVersion`
- payload 为 `payloadType: full`

预期：

1. 弹窗显示客户端更新
2. 用户可稍后处理
3. 点击更新后应用退出
4. updater 替换用户态安装目录
5. 应用自动重启
6. 新版本号等于 manifest `version`

### B：required app update success

构造规则：

- 本地 app version 小于 manifest `minimumAppVersion`

预期：

1. 弹窗阻断进入主应用
2. 没有“稍后再说”
3. 更新成功后自动重启并放行

### C：unsupported install

构造规则：

- 应用安装在 `Program Files`
- 或安装目录当前用户不可写

预期：

1. 不下载 payload
2. 不尝试替换文件
3. 弹窗提示迁移到用户态安装目录

### D：checksum mismatch

构造规则：

- manifest `sha256` 与 zip 不一致

预期：

1. 下载后校验失败
2. 应用不退出
3. 错误显示 `APP-UPDATE-CHECKSUM-MISMATCH`

### E：apply failure rollback

构造规则：

- staged payload 有效
- 替换过程中制造文件占用或删除失败

预期：

1. updater 记录失败
2. 恢复旧版本
3. 旧版本重启后显示“更新失败，已恢复旧版本”

### F：post-check failure rollback

构造规则：

- payload 可替换
- payload 缺少 `publish/runtime-manifest.json` 或 workspace agent payload

预期：

1. 新版本 post-update check 失败
2. updater 恢复旧版本
3. 旧版本重启后显示失败原因和诊断路径
```

- [ ] **Step 2: Update phase report**

Add a short “Next phase design completed” entry to `docs/reports/2026-05-16-windows-release-stability-phase-report.md`:

```markdown
## 下一阶段设计补充

已新增应用本体自更新设计与实施计划：

- `docs/superpowers/specs/2026-05-16-app-self-update-design.md`
- `docs/superpowers/plans/2026-05-16-app-self-update.md`

核心方向：

- Windows Phase 1 采用用户态安装目录
- 应用本体更新使用独立 app-update manifest
- Phase 1 使用 full zip payload
- 替换动作由短生命周期 `ant-chrome.exe --apply-update` 执行
- 共享 app-update core 保留 macOS backend 扩展点
```

- [ ] **Step 3: Commit**

```bash
rtk git add docs/release/windows-packaging-and-update-runbook.md docs/reports/2026-05-16-windows-release-stability-phase-report.md
rtk git commit -m "Document app self-update regression scenarios"
```

---

### Task 12: Final Verification And Windows Handoff

**Files:**
- No new files.
- Verify all modified files from Tasks 1-11.

- [ ] **Step 1: Run backend tests**

Run:

```bash
rtk go test ./backend ./backend/internal/appupdate ./backend/internal/config -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full Go test suite**

Run:

```bash
rtk go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 3: Run frontend build**

Run:

```bash
cd frontend && rtk npm run build
```

Expected: PASS.

- [ ] **Step 4: Run release contract checks**

Run:

```bash
rtk python3 tools/runtime/verify-publish-contract.py
```

Expected: `[OK] publish contract verified`.

- [ ] **Step 5: Windows packaging verification**

On Windows:

```powershell
git fetch --all
git checkout codex/windows-phase1-stability
git pull --ff-only
bat\publish.bat W -Version 1.2.0
```

Expected:

- `publish/output/AntBrowser-Setup-1.2.0.exe`
- `publish/output/AntBrowser-1.2.0-windows-amd64.zip`
- `publish/output/app-update-stable.json`
- `publish/output/app-update-stable.json.sha256`
- console includes app update package verification success

- [ ] **Step 6: True install regression**

On Windows:

```powershell
$env:DESKTOP_APP_UPDATE_MANIFEST_URL="file:///C:/path/to/app-update-stable.json"
```

Run these scenarios from the runbook:

- soft app update success
- required app update success
- manifest load fail
- unsupported `Program Files` install
- checksum mismatch
- invalid payload
- apply failure rollback
- post-check failure rollback

Expected: each scenario matches the runbook.

- [ ] **Step 7: Final commit if verification fixes were required**

If verification required code or documentation fixes:

```bash
rtk git add backend frontend tools bat publish docs config.yaml main.go wails.json
rtk git commit -m "Stabilize app self-update verification"
```

If no fixes were required, do not create an empty commit.

---

## Self-Review Checklist

- Spec coverage: Tasks cover manifest, source resolution, state machine, download, staging, Windows backend, CLI modes, Wails API, frontend UX, publish artifacts, runbook, and final verification.
- Cross-platform boundary: shared core and platform backend are separate; macOS remains a backend addition.
- Runtime update separation: existing runtime update APIs remain untouched except diagnostics additions.
- Phase 1 scope: full zip payload only; delta payloads are rejected by tests.
- Rollback: Windows backend tests and runbook cover apply failure and post-check rollback.
- No long-running helper: runner is copied from current executable and short-lived.
- No unsupported silent `Program Files` update: validation rejects it and frontend displays unsupported install state.
