package release

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type UpdateState struct {
	Kind             string `json:"kind"`
	LocalAppVersion  string `json:"localAppVersion"`
	RemoteAppVersion string `json:"remoteAppVersion"`
	ResourceVersion  string `json:"resourceVersion"`
}

type Manager struct {
	LocalManifest  Manifest
	RemoteManifest Manifest
	Layout         RuntimeLayout
}

type activePointer struct {
	Version         string `json:"version"`
	ResourceVersion string `json:"resourceVersion"`
}

func (m Manager) ClassifyUpdate(localResourceVersion string) UpdateState {
	state := UpdateState{
		Kind:             "none",
		LocalAppVersion:  strings.TrimSpace(m.LocalManifest.AppVersion),
		RemoteAppVersion: strings.TrimSpace(m.RemoteManifest.AppVersion),
		ResourceVersion:  strings.TrimSpace(m.RemoteManifest.MinimumResourceVersion),
	}

	if state.RemoteAppVersion == "" {
		state.RemoteAppVersion = state.LocalAppVersion
	}
	if state.ResourceVersion == "" {
		state.ResourceVersion = strings.TrimSpace(m.LocalManifest.MinimumResourceVersion)
	}

	if !m.RemoteManifest.ResourceCompatible(localResourceVersion) {
		state.Kind = "required"
		return state
	}
	if compareDottedVersionMust(state.RemoteAppVersion, state.LocalAppVersion) > 0 {
		state.Kind = "soft"
	}
	return state
}

func (m Manager) CurrentVersion() string {
	pointer, err := m.readActivePointer()
	if err != nil {
		return ""
	}
	if version := strings.TrimSpace(pointer.Version); version != "" {
		return version
	}
	return strings.TrimSpace(pointer.ResourceVersion)
}

func (m Manager) CurrentResourceVersion() string {
	pointer, err := m.readActivePointer()
	if err != nil {
		return ""
	}
	if version := strings.TrimSpace(pointer.ResourceVersion); version != "" {
		return version
	}
	return strings.TrimSpace(pointer.Version)
}

func (m Manager) ActivateVersion(version string, probe func(string) error) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return errors.New("version is required")
	}

	versionDir, err := m.Layout.VersionDir(version)
	if err != nil {
		return err
	}
	info, err := os.Stat(versionDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("runtime version path is not a directory: %s", versionDir)
	}

	previous := m.CurrentVersion()
	if err := m.writeCurrentVersion(version); err != nil {
		return err
	}
	if probe != nil {
		if err := probe(versionDir); err != nil {
			_ = m.rollback(previous)
			return err
		}
	}
	return nil
}

func (m Manager) rollback(previous string) error {
	previous = strings.TrimSpace(previous)
	if previous == "" {
		if err := os.Remove(m.Layout.ActivePointerPath()); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return m.writeCurrentVersion(previous)
}

func (m Manager) readActivePointer() (activePointer, error) {
	data, err := os.ReadFile(m.Layout.ActivePointerPath())
	if err != nil {
		return activePointer{}, err
	}
	var pointer activePointer
	if err := json.Unmarshal(data, &pointer); err != nil {
		return activePointer{}, err
	}
	return pointer, nil
}

func (m Manager) writeCurrentVersion(version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return errors.New("version is required")
	}
	pointer := activePointer{
		Version:         version,
		ResourceVersion: version,
	}
	data, err := json.Marshal(pointer)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.Layout.ActivePointerPath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.Layout.ActivePointerPath(), data, 0o600)
}

func compareDottedVersionMust(a, b string) int {
	av, okA := parseDottedVersion(a)
	bv, okB := parseDottedVersion(b)
	switch {
	case !okA && !okB:
		return 0
	case !okA:
		return 0
	case !okB:
		return 0
	default:
		return compareDottedVersion(av, bv)
	}
}
