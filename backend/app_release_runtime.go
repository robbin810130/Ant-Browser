package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/release"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

type releaseRuntimeManager struct {
	app                    *App
	remoteManifestProvider func(context.Context) (release.Manifest, error)
	activationProbe        func(string) error
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

var errNoRuntimeVersionsAvailable = errors.New("no runtime versions available to repair pointer")

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

func (a *App) ExportDesktopEnvironmentDiagnostics() (string, error) {
	manager, err := a.releaseManager()
	if err != nil {
		return "", err
	}
	return manager.ExportDiagnostics(a.ctx)
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
	if pathResult := m.checkLocalPathStatus(layout); pathResult.State != release.StatePass {
		return pathResult, nil
	}
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
				Code:              "ENV-MANIFEST-INVALID",
				Severity:          "error",
				Message:           "运行时 manifest 无法解析",
				Repairable:        false,
				RecommendedAction: "请确认安装包内容完整，并导出诊断包给支持团队检查 manifest 与运行时目录。",
			}},
		}, nil
	}

	target := release.DefaultTarget()
	resourceVersion, version, pointerStatus := loadActiveRuntimeVersion(layout.ActivePointerPath())
	if pointerStatus == activeRuntimePointerMissing {
		return release.CheckResult{
			State: release.StateRepairable,
			Items: []release.FailureItem{{
				Code:              "ENV-RUNTIME-POINTER-MISSING",
				Severity:          "error",
				Message:           "当前运行时指针缺失，需要修复",
				Repairable:        true,
				RecommendedAction: "先尝试自动修复；若修复后仍失败，请导出诊断包并检查 runtime/current.json 是否可创建。",
			}},
		}, nil
	}
	if pointerStatus == activeRuntimePointerInvalid {
		return release.CheckResult{
			State: release.StateRepairable,
			Items: []release.FailureItem{{
				Code:              "ENV-RUNTIME-POINTER-INVALID",
				Severity:          "error",
				Message:           "当前运行时指针损坏，需要修复",
				Repairable:        true,
				RecommendedAction: "先尝试自动修复；若仍失败，请删除损坏的 current.json 后重新检查，或导出诊断包给支持团队。",
			}},
		}, nil
	}
	browserCorePath := ""
	if versionDir, err := layout.VersionDir(version); err == nil {
		browserCorePath = resolveBrowserCorePath(manifest, target, versionDir)
	}

	result := release.Checker{Manifest: manifest}.Run(release.CheckInput{
		ManifestPath:    manifestPath,
		Target:          target,
		ResourceVersion: resourceVersion,
		BrowserCorePath: browserCorePath,
	})
	if result.State != release.StatePass {
		return result, nil
	}

	if workspaceResult := m.checkWorkspaceHostStatus(); workspaceResult.State != release.StatePass {
		return workspaceResult, nil
	}

	return result, nil
}

func (m *releaseRuntimeManager) checkLocalPathStatus(layout release.RuntimeLayout) release.CheckResult {
	if result := ensureExistingDirectory(
		layout.InstallRoot,
		"ENV-INSTALL-ROOT-INVALID",
		"应用安装目录无效，无法继续完成桌面环境初始化",
		"请确认当前应用目录存在且完整；若是从压缩包运行，请先完整解压后再启动。",
	); result.State != release.StatePass {
		return result
	}

	if result := ensureWritableDirectory(
		layout.StateRoot,
		"ENV-STATE-ROOT-UNWRITABLE",
		"应用状态目录不可写，无法保存运行时配置与登录态",
		"请检查当前用户对状态目录的写权限，必要时改到可写目录后重新启动，并导出诊断包。",
	); result.State != release.StatePass {
		return result
	}

	if result := ensureWritableDirectory(
		layout.RuntimeRoot(),
		"ENV-RUNTIME-ROOT-UNWRITABLE",
		"运行时目录不可写，无法生成 active pointer 或运行时版本目录",
		"请检查 runtime 目录是否被文件占用、杀软拦截或权限限制，修复后重新检查。",
	); result.State != release.StatePass {
		return result
	}

	if result := ensureWritableDirectory(
		layout.DiagnosticsRoot(),
		"ENV-DIAGNOSTICS-ROOT-UNWRITABLE",
		"诊断目录不可写，环境失败时将无法导出诊断包",
		"请检查 diagnostics 目录是否可创建和写入，修复后重新检查；如仍失败，请手工收集日志给支持团队。",
	); result.State != release.StatePass {
		return result
	}

	return release.CheckResult{State: release.StatePass}
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

func (m *releaseRuntimeManager) CheckForUpdate(ctx context.Context) (release.UpdateState, error) {
	manager, err := m.updateManager(ctx)
	if err != nil {
		return release.UpdateState{}, err
	}

	localResourceVersion := manager.CurrentResourceVersion()
	if strings.TrimSpace(localResourceVersion) == "" {
		localResourceVersion = strings.TrimSpace(manager.LocalManifest.MinimumResourceVersion)
	}
	return manager.ClassifyUpdate(localResourceVersion), nil
}

func (m *releaseRuntimeManager) ApplyConfirmedUpdate(ctx context.Context) (release.UpdateState, error) {
	manager, err := m.updateManager(ctx)
	if err != nil {
		return release.UpdateState{}, err
	}

	localResourceVersion := manager.CurrentResourceVersion()
	if strings.TrimSpace(localResourceVersion) == "" {
		localResourceVersion = strings.TrimSpace(manager.LocalManifest.MinimumResourceVersion)
	}
	state := manager.ClassifyUpdate(localResourceVersion)
	if state.Kind == "none" {
		return state, nil
	}

	targetVersion := strings.TrimSpace(state.ResourceVersion)
	if targetVersion == "" {
		return state, fmt.Errorf("remote resource version is required")
	}
	if err := manager.ActivateVersion(targetVersion, m.runtimeActivationProbeForManifest(manager.RemoteManifest)); err != nil {
		return state, err
	}
	return state, nil
}

func (m *releaseRuntimeManager) ExportDiagnostics(ctx context.Context) (string, error) {
	result, err := m.RunStartupCheck(ctx)
	if err != nil {
		return "", err
	}

	layout := m.app.runtimeLayout()
	manifest, _ := release.LoadManifest(m.manifestPath())
	resourceVersion, version, _ := loadActiveRuntimeVersion(layout.ActivePointerPath())

	errorCodes := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		if code := strings.TrimSpace(item.Code); code != "" {
			errorCodes = append(errorCodes, code)
		}
	}

	events := []release.DiagnosticEvent{{
		EventTime:       time.Now().UTC().Format(time.RFC3339),
		Stage:           "selfcheck",
		Result:          diagnosticResultFromState(result.State),
		ErrorCode:       strings.Join(errorCodes, ","),
		AppVersion:      strings.TrimSpace(manifest.AppVersion),
		ManifestVersion: strings.TrimSpace(manifest.MinimumResourceVersion),
		ResourceVersion: strings.TrimSpace(resourceVersion),
		Summary:         diagnosticSummary(result),
		Fields: map[string]string{
			"machineScope":       release.DefaultTarget(),
			"environmentState":   string(result.State),
			"activeRuntime":      strings.TrimSpace(version),
			"runtimePointerPath": layout.ActivePointerPath(),
		},
	}}

	logs := make([]release.DiagnosticLogEntry, 0, len(logger.GetBufferedEntries()))
	for _, entry := range logger.GetBufferedEntries() {
		logs = append(logs, release.DiagnosticLogEntry{
			Time:      entry.Time,
			Level:     entry.Level,
			Component: entry.Component,
			Message:   entry.Message,
			Fields:    release.SanitizeLogFields(entry.Fields),
		})
	}

	workspaceServerOriginDetails := resolveWorkspaceServerOriginDetails(resolveWorkspaceRuntimeDirWithConfig(m.app.config), m.app.config)

	return release.WriteDiagnosticBundle(layout.DiagnosticsRoot(), release.DiagnosticBundle{
		Platform:         fmt.Sprintf("%s-%s", goruntime.GOOS, goruntime.GOARCH),
		AppVersion:       strings.TrimSpace(manifest.AppVersion),
		ManifestVersion:  strings.TrimSpace(manifest.MinimumResourceVersion),
		ResourceVersion:  strings.TrimSpace(resourceVersion),
		EnvironmentState: string(result.State),
		ErrorCodes:       errorCodes,
		Summary:          diagnosticSummary(result),
		Paths: map[string]string{
			"installRoot":                layout.InstallRoot,
			"stateRoot":                  layout.StateRoot,
			"manifestPath":               m.manifestPath(),
			"runtimeRoot":                layout.RuntimeRoot(),
			"activePointer":              layout.ActivePointerPath(),
			"diagnosticsRoot":            layout.DiagnosticsRoot(),
			"workspaceRuntimeDir":        resolveWorkspaceRuntimeDirWithConfig(m.app.config),
			"workspaceServerOrigin":      workspaceServerOriginDetails.Origin,
			"workspaceServerOriginSource": workspaceServerOriginDetails.Source,
			"workspaceServerConfigPath":  workspaceServerOriginDetails.ConfigPath,
		},
		Events: events,
		Logs:   logs,
	})
}

func (m *releaseRuntimeManager) checkWorkspaceHostStatus() release.CheckResult {
	resolution := resolveWorkspaceServerOriginDetails(resolveWorkspaceRuntimeDirWithConfig(m.app.config), m.app.config)
	serverOrigin := strings.TrimRight(strings.TrimSpace(resolution.Origin), "/")
	if serverOrigin == "" {
		return release.CheckResult{
			State: release.StateBlocked,
			Items: []release.FailureItem{{
				Code:              workspaceHostFailureCode(resolution),
				Severity:          "error",
				Message:           "workspace host 地址为空，无法继续登录",
				Repairable:        false,
				RecommendedAction: workspaceServerOriginAction(resolution),
			}},
		}
	}

	for _, path := range []string{"/api/client/health", "/api/health"} {
		if err := getWorkspaceJSON(serverOrigin+path, nil); err == nil {
			return release.CheckResult{State: release.StatePass}
		}
	}

	return release.CheckResult{
		State: release.StateBlocked,
		Items: []release.FailureItem{{
			Code:              workspaceHostFailureCode(resolution),
			Severity:          "error",
			Message:           fmt.Sprintf("workspace host 不可达，请确认服务端已启动并检查连接配置 (%s)", serverOrigin),
			Repairable:        false,
			RecommendedAction: workspaceServerOriginAction(resolution),
		}},
	}
}

func workspaceHostFailureCode(resolution workspaceServerOriginResolution) string {
	switch resolution.Source {
	case "runtime-config":
		return "ENV-WORKSPACE-HOST-RUNTIME-CONFIG-UNREACHABLE"
	case "env:DESKTOP_SERVER_BASE_URL":
		return "ENV-WORKSPACE-HOST-ENV-UNREACHABLE"
	case "config.yaml":
		return "ENV-WORKSPACE-HOST-APP-CONFIG-UNREACHABLE"
	default:
		return "ENV-WORKSPACE-HOST-DEFAULT-UNREACHABLE"
	}
}

func workspaceServerOriginAction(resolution workspaceServerOriginResolution) string {
	origin := strings.TrimSpace(resolution.Origin)
	switch resolution.Source {
	case "runtime-config":
		return fmt.Sprintf("当前 workspace host 来自 server-connection.json：%s。请检查该文件中的地址是否正确，并确认对应服务已启动。", strings.TrimSpace(resolution.ConfigPath))
	case "env:DESKTOP_SERVER_BASE_URL":
		return fmt.Sprintf("当前 workspace host 来自环境变量 DESKTOP_SERVER_BASE_URL (%s)。请检查该变量值是否正确，并确认对应服务已启动。", origin)
	case "config.yaml":
		return fmt.Sprintf("当前 workspace host 来自 config.yaml 的 workspace.server_origin (%s)。请检查配置值是否正确，并确认对应服务已启动。", origin)
	default:
		return fmt.Sprintf("当前 workspace host 使用默认地址 (%s)。请确认本机 workspace server 已启动；若仍失败，请导出诊断包。", origin)
	}
}

func ensureExistingDirectory(path, code, message, action string) release.CheckResult {
	path = strings.TrimSpace(path)
	if path == "" {
		return blockedFailure(code, message, action)
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return blockedFailure(code, message, action)
	}

	return release.CheckResult{State: release.StatePass}
}

func ensureWritableDirectory(path, code, message, action string) release.CheckResult {
	path = strings.TrimSpace(path)
	if path == "" {
		return blockedFailure(code, message, action)
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return blockedFailure(code, message, action)
	}

	file, err := os.CreateTemp(path, ".write-check-*")
	if err != nil {
		return blockedFailure(code, message, action)
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)

	return release.CheckResult{State: release.StatePass}
}

func blockedFailure(code, message, action string) release.CheckResult {
	return release.CheckResult{
		State: release.StateBlocked,
		Items: []release.FailureItem{{
			Code:              code,
			Severity:          "error",
			Message:           message,
			Repairable:        false,
			RecommendedAction: action,
		}},
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

func (m *releaseRuntimeManager) updateManager(ctx context.Context) (release.Manager, error) {
	_ = ctx
	localManifest, err := release.LoadManifest(m.manifestPath())
	if err != nil {
		return release.Manager{}, err
	}

	remoteManifest := localManifest
	if m.remoteManifestProvider != nil {
		remoteManifest, err = m.remoteManifestProvider(ctx)
		if err != nil {
			return release.Manager{}, err
		}
	}

	return release.Manager{
		LocalManifest:  localManifest,
		RemoteManifest: remoteManifest,
		Layout:         m.app.runtimeLayout(),
	}, nil
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

func diagnosticResultFromState(state release.CheckState) string {
	switch state {
	case release.StatePass:
		return "success"
	case release.StateRepairable:
		return "warning"
	default:
		return "failure"
	}
}

func diagnosticSummary(result release.CheckResult) string {
	if len(result.Items) == 0 {
		return fmt.Sprintf("environment state: %s", result.State)
	}
	return strings.TrimSpace(result.Items[0].Message)
}

func (m *releaseRuntimeManager) runtimeActivationProbeForManifest(manifest release.Manifest) func(string) error {
	if m.activationProbe != nil {
		return m.activationProbe
	}
	return func(versionDir string) error {
		return m.probeRuntimePackages(manifest, versionDir)
	}
}

func (m *releaseRuntimeManager) probeRuntimePackages(manifest release.Manifest, versionDir string) error {
	packages, err := manifest.RequiredPackages(release.DefaultTarget())
	if err != nil {
		return err
	}
	for _, pkg := range packages {
		if strings.EqualFold(strings.TrimSpace(pkg.Kind), "browser-core") {
			corePath := release.ResolvePackagePath(versionDir, pkg)
			if result := browser.ValidateCoreDirectory(corePath); !result.Valid {
				return fmt.Errorf(result.Message)
			}
			continue
		}
		path := release.ResolvePackagePath(versionDir, pkg)
		if path == "" {
			return fmt.Errorf("invalid package path for %s", pkg.ID)
		}
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("required runtime package missing after activation: %s", pkg.ID)
		}
	}
	return nil
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
		if errors.Is(err, errNoRuntimeVersionsAvailable) {
			return m.syncRuntimePackages(context.Background())
		}
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

	for _, pkg := range packages {
		if strings.EqualFold(strings.TrimSpace(pkg.Kind), "browser-core") {
			continue
		}
		if pkg.Path == "" {
			continue
		}
		src, err := resolveRuntimePackageSource(layout, pkg)
		if err != nil {
			return err
		}
		dst := release.ResolvePackagePath(versionDir, pkg)
		if dst == "" {
			return fmt.Errorf("invalid package path for %s", pkg.ID)
		}
		info, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("runtime package source missing for %s: %w", pkg.ID, err)
		}
		if info.IsDir() {
			return fmt.Errorf("runtime package source must be a file for %s", pkg.ID)
		}
		if err := copyFile(src, dst, info.Mode().Perm()); err != nil {
			return err
		}
		if err := verifySHA256(dst, pkg.SHA256); err != nil {
			return err
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
			return "", errNoRuntimeVersionsAvailable
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

func resolveRuntimePackageSource(layout release.RuntimeLayout, pkg release.RuntimePackage) (string, error) {
	packagePath := filepath.FromSlash(strings.TrimSpace(pkg.Path))
	if packagePath == "" {
		return "", fmt.Errorf("runtime package path is required for %s", pkg.ID)
	}

	candidates := []string{
		filepath.Join(layout.InstallRoot, "publish", packagePath),
		filepath.Join(layout.InstallRoot, packagePath),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("runtime package source missing for %s", pkg.ID)
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
