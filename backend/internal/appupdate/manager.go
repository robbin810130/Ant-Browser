package appupdate

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ManifestProvider func(context.Context) (Manifest, ManifestSourceResolution, error)

type Manager struct {
	LocalAppVersion  string
	Layout           Layout
	Platform         PlatformUpdater
	ManifestProvider ManifestProvider
}

func (m Manager) Check(ctx context.Context) (State, error) {
	manifest, resolution, pkg, state, err := m.loadUpdate(ctx)
	if err != nil {
		if state.Kind == "" {
			return State{}, err
		}
		return state, nil
	}
	_ = manifest

	if err := m.Platform.ValidateInstallMode(m.Layout); err != nil {
		state.Kind = UpdateKindUnsupportedInstall
		state.ErrorCode = "APP-UPDATE-INSTALL-UNSUPPORTED"
		state.ErrorMessage = err.Error()
		return state, nil
	}

	status := PersistentStatusIdle
	if state.Kind == UpdateKindSoft || state.Kind == UpdateKindRequired {
		status = PersistentStatusAvailable
	}
	state.Status = status
	state.PayloadURL = pkg.URL
	if err := WriteState(m.Layout, PersistentState{
		Status:           status,
		LocalAppVersion:  m.LocalAppVersion,
		RemoteAppVersion: manifest.Version,
		ManifestSource:   resolution.Source,
		ManifestURL:      resolution.URL,
		PayloadURL:       pkg.URL,
		Target:           pkg.Target,
	}); err != nil {
		return State{}, err
	}
	return state, nil
}

func (m Manager) Download(ctx context.Context) (State, error) {
	manifest, resolution, pkg, state, err := m.loadUpdate(ctx)
	if err != nil {
		if state.Kind == "" {
			return State{}, err
		}
		return state, nil
	}

	if state.Kind != UpdateKindSoft && state.Kind != UpdateKindRequired {
		state.Status = PersistentStatusIdle
		return state, nil
	}
	if err := m.Platform.ValidateInstallMode(m.Layout); err != nil {
		state.Kind = UpdateKindUnsupportedInstall
		state.ErrorCode = "APP-UPDATE-INSTALL-UNSUPPORTED"
		state.ErrorMessage = err.Error()
		return state, nil
	}

	downloading := PersistentState{
		Status:           PersistentStatusDownloading,
		LocalAppVersion:  m.LocalAppVersion,
		RemoteAppVersion: manifest.Version,
		ManifestSource:   resolution.Source,
		ManifestURL:      resolution.URL,
		PayloadURL:       pkg.URL,
		Target:           pkg.Target,
	}
	if err := WriteState(m.Layout, downloading); err != nil {
		return State{}, err
	}

	payloadPath, err := DownloadPayload(ctx, pkg.URL, m.Layout.DownloadsRoot(), pkg.SHA256, pkg.Size)
	if err != nil {
		return m.failDownloadState(downloading, "APP-UPDATE-DOWNLOAD-FAILED", err)
	}

	stagedPath := filepath.Join(m.Layout.StagingRoot(), manifest.Version)
	if err := ExtractFullPayload(payloadPath, stagedPath); err != nil {
		return m.failDownloadState(downloading, "APP-UPDATE-STAGE-FAILED", err)
	}
	if err := ValidateStagedPayload(pkg.Target, stagedPath); err != nil {
		return m.failDownloadState(downloading, "APP-UPDATE-STAGED-PAYLOAD-INVALID", err)
	}

	staged := downloading
	staged.Status = PersistentStatusStaged
	if err := WriteState(m.Layout, staged); err != nil {
		return State{}, err
	}
	state.Status = PersistentStatusStaged
	state.PayloadURL = pkg.URL
	return state, nil
}

func (m Manager) Apply(ctx context.Context) (State, error) {
	_ = ctx
	if err := m.validateCore(); err != nil {
		return State{}, err
	}

	persistent, err := ReadState(m.Layout)
	if err != nil {
		return State{}, err
	}
	if persistent.Status != PersistentStatusStaged {
		return State{}, fmt.Errorf("app update is not staged: %s", persistent.Status)
	}

	stagedPath := filepath.Join(m.Layout.StagingRoot(), persistent.RemoteAppVersion)
	backupPath := filepath.Join(m.Layout.BackupsRoot(), persistent.LocalAppVersion+"-backup")
	plan := ApplyPlan{
		InstallRoot:      m.Layout.InstallRoot,
		StateRoot:        m.Layout.StateRoot,
		Target:           persistent.Target,
		OldAppVersion:    persistent.LocalAppVersion,
		NewAppVersion:    persistent.RemoteAppVersion,
		StagedPath:       stagedPath,
		BackupPath:       backupPath,
		ExpectedSHA256:   "",
		ManifestSource:   persistent.ManifestSource,
		ManifestURL:      persistent.ManifestURL,
		PayloadURL:       persistent.PayloadURL,
		WaitForProcessID: os.Getpid(),
	}
	plan.RunnerPath = newRunnerPath(m.Layout, plan)
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

	next := persistent
	next.Status = PersistentStatusApplying
	next.PlanPath = planPath
	next.BackupPath = backupPath
	if err := WriteState(m.Layout, next); err != nil {
		return State{}, err
	}
	return persistentStateToState(next, UpdateKindSoft), nil
}

func newRunnerPath(layout Layout, plan ApplyPlan) string {
	segment := safeRunnerPathSegment(plan.NewAppVersion)
	if segment == "" {
		segment = "update"
	}
	return filepath.Join(layout.RunnerRoot(), fmt.Sprintf("%s-%d-%d", segment, plan.WaitForProcessID, time.Now().UnixNano()), "ant-chrome-update-runner.exe")
}

func safeRunnerPathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), ".-_")
}

func DefaultManifestProvider(source func() ManifestSourceResolution) ManifestProvider {
	return func(ctx context.Context) (Manifest, ManifestSourceResolution, error) {
		resolution := source()
		if resolution.URL == "" {
			return Manifest{}, resolution, os.ErrNotExist
		}
		manifest, err := LoadManifestFromSource(ctx, resolution)
		return manifest, resolution, err
	}
}

func (m Manager) loadUpdate(ctx context.Context) (Manifest, ManifestSourceResolution, Package, State, error) {
	if err := m.validateWithProvider(); err != nil {
		return Manifest{}, ManifestSourceResolution{}, Package{}, State{}, err
	}

	manifest, resolution, err := m.ManifestProvider(ctx)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && resolution.URL == "" {
			state := State{
				Kind:            UpdateKindNone,
				Status:          PersistentStatusIdle,
				LocalAppVersion: m.LocalAppVersion,
			}
			return Manifest{}, resolution, Package{}, state, err
		}
		state := State{
			Kind:            UpdateKindFailed,
			LocalAppVersion: m.LocalAppVersion,
			ManifestSource:  resolution.Source,
			ManifestURL:     resolution.URL,
			ErrorCode:       "APP-UPDATE-MANIFEST-LOAD-FAILED",
			ErrorMessage:    err.Error(),
		}
		return Manifest{}, resolution, Package{}, state, err
	}

	target := m.Platform.Target()
	state := State{
		Kind:                          Classify(m.LocalAppVersion, manifest),
		LocalAppVersion:               m.LocalAppVersion,
		RemoteAppVersion:              manifest.Version,
		MinimumRuntimeResourceVersion: manifest.MinimumRuntimeResourceVersion,
		ManifestSource:                resolution.Source,
		ManifestURL:                   resolution.URL,
		Target:                        target,
		Notes:                         manifest.Notes,
	}

	pkg, err := manifest.PackageForTarget(target)
	if err != nil {
		state.Kind = UpdateKindFailed
		state.ErrorCode = "APP-UPDATE-TARGET-MISSING"
		state.ErrorMessage = err.Error()
		return manifest, resolution, Package{}, state, err
	}
	pkg.URL = resolvePackageSource(resolution.URL, pkg.URL)
	state.PayloadURL = pkg.URL
	return manifest, resolution, pkg, state, nil
}

func resolvePackageSource(manifestSourceURL string, packageURL string) string {
	packageURL = strings.TrimSpace(packageURL)
	if packageURL == "" || isAbsolutePackageSource(packageURL) {
		return packageURL
	}

	manifestSourceURL = strings.TrimSpace(manifestSourceURL)
	if manifestSourceURL == "" {
		return packageURL
	}

	kind, location, err := resolveManifestSourceLocation(manifestSourceURL)
	if err != nil {
		return packageURL
	}
	switch kind {
	case manifestSourceHTTP:
		base, err := url.Parse(location)
		if err != nil {
			return packageURL
		}
		ref, err := url.Parse(packageURL)
		if err != nil {
			return packageURL
		}
		return base.ResolveReference(ref).String()
	case manifestSourceFile:
		return filepath.Join(filepath.Dir(location), filepath.FromSlash(packageURL))
	case manifestSourceLocal:
		return filepath.Join(filepath.Dir(location), filepath.FromSlash(packageURL))
	default:
		return packageURL
	}
}

func isAbsolutePackageSource(source string) bool {
	if isWindowsAbsolutePath(source) || filepath.IsAbs(source) {
		return true
	}
	parsed, err := url.Parse(source)
	return err == nil && parsed.Scheme != ""
}

func (m Manager) validateWithProvider() error {
	if err := m.validateCore(); err != nil {
		return err
	}
	if m.ManifestProvider == nil {
		return fmt.Errorf("app update manifest provider is required")
	}
	return nil
}

func (m Manager) validateCore() error {
	if err := m.Layout.Validate(); err != nil {
		return err
	}
	if m.Platform == nil {
		return fmt.Errorf("app update platform is required")
	}
	return nil
}

func (m Manager) failDownloadState(base PersistentState, code string, err error) (State, error) {
	base.Status = PersistentStatusAvailable
	base.LastError = ErrorInfo{
		Code:    code,
		Message: err.Error(),
	}
	if writeErr := WriteState(m.Layout, base); writeErr != nil {
		return State{}, writeErr
	}
	return persistentStateToState(base, UpdateKindFailed), nil
}

func persistentStateToState(persistent PersistentState, kind UpdateKind) State {
	return State{
		Kind:             kind,
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
	}
}
