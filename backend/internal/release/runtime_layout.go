package release

import (
	"fmt"
	"path/filepath"
	"strings"
)

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

func (l RuntimeLayout) VersionDir(version string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" || version == "." || version == ".." {
		return "", fmt.Errorf("invalid version")
	}
	if strings.ContainsAny(version, `/\`) {
		return "", fmt.Errorf("invalid version")
	}
	if cleaned := filepath.Clean(version); cleaned != version || cleaned == "." || cleaned == ".." {
		return "", fmt.Errorf("invalid version")
	}
	return filepath.Join(l.VersionsRoot(), version), nil
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
