package backend

import (
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/release"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type releaseRuntimeManager struct {
	app *App
}

type activeRuntimePointer struct {
	Version         string `json:"version"`
	ResourceVersion string `json:"resourceVersion"`
}

type activeRuntimePointerStatus string

const (
	activeRuntimePointerOK      activeRuntimePointerStatus = "ok"
	activeRuntimePointerMissing activeRuntimePointerStatus = "missing"
	activeRuntimePointerInvalid activeRuntimePointerStatus = "invalid"
)

func (a *App) GetDesktopEnvironmentStatus() (release.CheckResult, error) {
	manager, err := a.releaseManager()
	if err != nil {
		return release.CheckResult{}, err
	}
	return manager.RunStartupCheck(a.ctx)
}

func (a *App) RepairDesktopEnvironment() (release.CheckResult, error) {
	manager, err := a.releaseManager()
	if err != nil {
		return release.CheckResult{}, err
	}
	return manager.RepairAndRecheck(a.ctx)
}

func (a *App) releaseManager() (*releaseRuntimeManager, error) {
	if a.releaseManagerFn != nil {
		return a.releaseManagerFn()
	}
	return &releaseRuntimeManager{app: a}, nil
}

func (m *releaseRuntimeManager) RunStartupCheck(ctx context.Context) (release.CheckResult, error) {
	_ = ctx

	layout := m.app.runtimeLayout()
	manifestPath := m.manifestPath()
	if _, err := os.Stat(manifestPath); err != nil {
		return release.Checker{}.Run(release.CheckInput{
			ManifestPath: manifestPath,
			Target:       release.DefaultTarget(),
		}), nil
	}

	manifest, err := release.LoadManifest(manifestPath)
	if err != nil {
		return release.CheckResult{
			State: release.StateBlocked,
			Items: []release.FailureItem{{
				Code:       "ENV-MANIFEST-INVALID",
				Severity:   "error",
				Message:    "运行时 manifest 无法解析",
				Repairable: false,
			}},
		}, nil
	}

	target := release.DefaultTarget()
	resourceVersion, version, pointerStatus := loadActiveRuntimeVersion(layout.ActivePointerPath())
	if pointerStatus == activeRuntimePointerMissing {
		return release.CheckResult{
			State: release.StateRepairable,
			Items: []release.FailureItem{{
				Code:       "ENV-RUNTIME-POINTER-MISSING",
				Severity:   "error",
				Message:    "当前运行时指针缺失，需要修复",
				Repairable: true,
			}},
		}, nil
	}
	if pointerStatus == activeRuntimePointerInvalid {
		return release.CheckResult{
			State: release.StateRepairable,
			Items: []release.FailureItem{{
				Code:       "ENV-RUNTIME-POINTER-INVALID",
				Severity:   "error",
				Message:    "当前运行时指针损坏，需要修复",
				Repairable: true,
			}},
		}, nil
	}
	browserCorePath := ""
	if versionDir, err := layout.VersionDir(version); err == nil {
		browserCorePath = resolveBrowserCorePath(manifest, target, versionDir)
	}

	return release.Checker{Manifest: manifest}.Run(release.CheckInput{
		ManifestPath:    manifestPath,
		Target:          target,
		ResourceVersion: resourceVersion,
		BrowserCorePath: browserCorePath,
	}), nil
}

func (m *releaseRuntimeManager) RepairAndRecheck(ctx context.Context) (release.CheckResult, error) {
	result, err := m.RunStartupCheck(ctx)
	if err != nil {
		return release.CheckResult{}, err
	}
	return release.ExecuteRepairWithContext(ctx, m, result)
}

func (m *releaseRuntimeManager) ApplyRepairAction(ctx context.Context, action release.RepairAction) error {
	switch action.Kind {
	case "rewrite-active-pointer":
		return m.rewriteActivePointer()
	case "cleanup-temp":
		return m.cleanupStaging(ctx)
	case "fetch-package":
		return m.syncRuntimePackages(ctx)
	default:
		return fmt.Errorf("unsupported repair action: %s", action.Kind)
	}
}

func loadActiveRuntimeVersion(pointerPath string) (resourceVersion string, version string, status activeRuntimePointerStatus) {
	data, err := os.ReadFile(pointerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", activeRuntimePointerMissing
		}
		return "", "", activeRuntimePointerInvalid
	}

	var pointer activeRuntimePointer
	if err := json.Unmarshal(data, &pointer); err != nil {
		return "", "", activeRuntimePointerInvalid
	}

	resourceVersion = strings.TrimSpace(pointer.ResourceVersion)
	version = strings.TrimSpace(pointer.Version)
	if resourceVersion == "" || version == "" {
		return "", "", activeRuntimePointerInvalid
	}
	return resourceVersion, version, activeRuntimePointerOK
}

func resolveBrowserCorePath(manifest release.Manifest, target, versionDir string) string {
	packages, err := manifest.RequiredPackages(target)
	if err != nil {
		return ""
	}
	for _, pkg := range packages {
		if strings.EqualFold(strings.TrimSpace(pkg.Kind), "browser-core") {
			return release.ResolvePackagePath(versionDir, pkg)
		}
	}
	return ""
}

func (m *releaseRuntimeManager) manifestPath() string {
	return filepath.Join(m.app.runtimeLayout().InstallRoot, "publish", "runtime-manifest.json")
}

func (m *releaseRuntimeManager) rewriteActivePointer() error {
	layout := m.app.runtimeLayout()
	manifest, err := release.LoadManifest(m.manifestPath())
	if err != nil {
		return err
	}

	version, err := m.inferRepairVersion(layout, manifest)
	if err != nil {
		return err
	}
	return writeActiveRuntimePointer(layout.ActivePointerPath(), activeRuntimePointer{
		Version:         version,
		ResourceVersion: version,
	})
}

func (m *releaseRuntimeManager) cleanupStaging(ctx context.Context) error {
	_ = ctx
	layout := m.app.runtimeLayout()
	if err := os.RemoveAll(layout.StagingRoot()); err != nil {
		return err
	}
	return os.MkdirAll(layout.StagingRoot(), 0o755)
}

func (m *releaseRuntimeManager) syncRuntimePackages(ctx context.Context) error {
	_ = ctx
	layout := m.app.runtimeLayout()
	manifest, err := release.LoadManifest(m.manifestPath())
	if err != nil {
		return err
	}

	target := release.DefaultTarget()
	packages, err := manifest.RequiredPackages(target)
	if err != nil {
		return err
	}

	version := strings.TrimSpace(manifest.MinimumResourceVersion)
	if version == "" {
		return fmt.Errorf("manifest minimumResourceVersion is required")
	}
	versionDir, err := layout.VersionDir(version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		return err
	}

	for _, file := range manifest.Files {
		if !slices.Contains(file.Targets, target) {
			continue
		}
		if err := copyManifestFile(filepath.Join(layout.InstallRoot, "publish"), versionDir, file); err != nil {
			return err
		}
	}

	for _, pkg := range packages {
		if strings.EqualFold(strings.TrimSpace(pkg.Kind), "browser-core") {
			continue
		}
		if pkg.Path == "" {
			continue
		}
		dst := release.ResolvePackagePath(versionDir, pkg)
		if dst == "" {
			return fmt.Errorf("invalid package path for %s", pkg.ID)
		}
		if _, err := os.Stat(dst); err != nil {
			return fmt.Errorf("required runtime package missing after sync: %s", pkg.ID)
		}
		if err := fsutil.EnsureExecutable(dst); err != nil {
			return err
		}
	}

	return writeActiveRuntimePointer(layout.ActivePointerPath(), activeRuntimePointer{
		Version:         version,
		ResourceVersion: version,
	})
}

func (m *releaseRuntimeManager) inferRepairVersion(layout release.RuntimeLayout, manifest release.Manifest) (string, error) {
	minimum := strings.TrimSpace(manifest.MinimumResourceVersion)
	if minimum != "" {
		if versionDir, err := layout.VersionDir(minimum); err == nil {
			if info, statErr := os.Stat(versionDir); statErr == nil && info.IsDir() {
				return minimum, nil
			}
		}
	}

	entries, err := os.ReadDir(layout.VersionsRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no runtime versions available to repair pointer")
		}
		return "", err
	}

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, strings.TrimSpace(entry.Name()))
		}
	}
	if len(versions) == 1 {
		return versions[0], nil
	}
	if minimum != "" && len(versions) > 1 {
		for _, version := range versions {
			if version == minimum {
				return version, nil
			}
		}
	}
	return "", fmt.Errorf("unable to infer active runtime version deterministically")
}

func writeActiveRuntimePointer(pointerPath string, pointer activeRuntimePointer) error {
	data, err := json.Marshal(pointer)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(pointerPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(pointerPath, data, 0o600)
}

func copyManifestFile(publishRoot, versionDir string, file release.RuntimeFile) error {
	relPath := filepath.FromSlash(strings.TrimSpace(file.Path))
	if relPath == "" || relPath == "." {
		return fmt.Errorf("invalid runtime file path")
	}

	src := filepath.Join(publishRoot, relPath)
	dst := filepath.Join(versionDir, relPath)
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("runtime file source must be a file: %s", src)
	}
	if err := copyFile(src, dst, info.Mode().Perm()); err != nil {
		return err
	}
	if err := verifySHA256(dst, file.SHA256); err != nil {
		return err
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func verifySHA256(path, expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := fmt.Sprintf("%x", hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("sha256 mismatch for %s", path)
	}
	return nil
}
