package appupdate

import "path/filepath"

type Layout struct {
	InstallRoot string
	StateRoot   string
}

func NewLayout(installRoot string, stateRoot string) Layout {
	return Layout{
		InstallRoot: filepath.Clean(installRoot),
		StateRoot:   filepath.Clean(stateRoot),
	}
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
