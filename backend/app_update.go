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
		Platform: appupdate.WindowsBackend{
			ProcessID: os.Getpid(),
		},
		ManifestProvider: appupdate.DefaultManifestProvider(func() appupdate.ManifestSourceResolution {
			return appupdate.ResolveManifestSource(resolveWorkspaceRuntimeDirWithConfig(a.config), a.config)
		}),
	}
}

func (a *App) CheckDesktopAppUpdate() (appupdate.State, error) {
	return a.appUpdateManager().Check(a.appUpdateContext())
}

func (a *App) DownloadDesktopAppUpdate() (appupdate.State, error) {
	return a.appUpdateManager().Download(a.appUpdateContext())
}

func (a *App) ApplyDesktopAppUpdate() (appupdate.State, error) {
	return a.appUpdateManager().Apply(a.appUpdateContext())
}

func (a *App) GetDesktopAppUpdateState() (appupdate.State, error) {
	persistent, err := appupdate.ReadState(a.appUpdateLayout())
	if os.IsNotExist(err) {
		return appupdate.State{
			Kind:            appupdate.UpdateKindNone,
			Status:          appupdate.PersistentStatusIdle,
			LocalAppVersion: a.appVersion(),
		}, nil
	}
	if err != nil {
		return appupdate.State{}, err
	}
	return appUpdatePersistentStateToAPI(persistent, a.appVersion()), nil
}

func (a *App) ClearDesktopAppUpdateFailure() error {
	layout := a.appUpdateLayout()
	if err := os.Remove(layout.StatePath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(layout.PlanPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (a *App) appUpdateContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func appUpdatePersistentStateToAPI(persistent appupdate.PersistentState, fallbackLocalVersion string) appupdate.State {
	kind := appupdate.UpdateKindNone
	if persistent.Status == appupdate.PersistentStatusRolledBack ||
		persistent.Status == appupdate.PersistentStatusFailedManualRepair ||
		persistent.LastError.Code != "" {
		kind = appupdate.UpdateKindFailed
	}
	localVersion := persistent.LocalAppVersion
	if localVersion == "" {
		localVersion = fallbackLocalVersion
	}
	return appupdate.State{
		Kind:             kind,
		Status:           persistent.Status,
		LocalAppVersion:  localVersion,
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
