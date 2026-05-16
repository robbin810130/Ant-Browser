package appupdate

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Layout struct {
	InstallRoot string
	StateRoot   string
}

func NewLayout(installRoot string, stateRoot string) Layout {
	return Layout{
		InstallRoot: cleanRoot(installRoot),
		StateRoot:   cleanRoot(stateRoot),
	}
}

func (l Layout) Validate() error {
	if isUnsafeRoot(l.InstallRoot) {
		return fmt.Errorf("app update install root is required")
	}
	if isUnsafeRoot(l.StateRoot) {
		return fmt.Errorf("app update state root is required")
	}
	return nil
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

func cleanRoot(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	return filepath.Clean(root)
}

func isUnsafeRoot(root string) bool {
	trimmed := strings.TrimSpace(root)
	return trimmed == "" || trimmed == "."
}
