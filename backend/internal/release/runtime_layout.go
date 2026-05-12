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
