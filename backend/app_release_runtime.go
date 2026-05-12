package backend

import (
	"ant-chrome/backend/internal/release"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type releaseRuntimeManager struct {
	app *App
}

type activeRuntimePointer struct {
	Version         string `json:"version"`
	ResourceVersion string `json:"resourceVersion"`
}

func (a *App) GetDesktopEnvironmentStatus() (release.CheckResult, error) {
	manager, err := a.releaseManager()
	if err != nil {
		return release.CheckResult{}, err
	}
	return manager.RunStartupCheck(a.ctx)
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
	manifestPath := filepath.Join(layout.InstallRoot, "publish", "runtime-manifest.json")
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
	resourceVersion, version := loadActiveRuntimeVersion(layout.ActivePointerPath())
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

func loadActiveRuntimeVersion(pointerPath string) (resourceVersion string, version string) {
	data, err := os.ReadFile(pointerPath)
	if err != nil {
		return "", ""
	}

	var pointer activeRuntimePointer
	if err := json.Unmarshal(data, &pointer); err != nil {
		return "", ""
	}

	resourceVersion = strings.TrimSpace(pointer.ResourceVersion)
	version = strings.TrimSpace(pointer.Version)
	if version == "" {
		version = resourceVersion
	}
	return resourceVersion, version
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
