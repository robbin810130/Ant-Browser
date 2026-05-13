package release

import (
	"errors"
	"os"
	"testing"
)

var assertErrProbeFailed = errors.New("probe failed")

func TestClassifyUpdateRequired(t *testing.T) {
	manager := Manager{
		LocalManifest:  Manifest{AppVersion: "1.1.0", MinimumResourceVersion: "2026.05.12"},
		RemoteManifest: Manifest{AppVersion: "1.2.0", MinimumResourceVersion: "2026.06.01"},
	}
	state := manager.ClassifyUpdate("2026.05.12")
	if state.Kind != "required" {
		t.Fatalf("expected required update, got %#v", state)
	}
}

func TestClassifyUpdateSoft(t *testing.T) {
	manager := Manager{
		LocalManifest:  Manifest{AppVersion: "1.1.0", MinimumResourceVersion: "2026.05.12"},
		RemoteManifest: Manifest{AppVersion: "1.2.0", MinimumResourceVersion: "2026.05.12"},
	}
	state := manager.ClassifyUpdate("2026.05.12")
	if state.Kind != "soft" {
		t.Fatalf("expected soft update, got %#v", state)
	}
}

func TestActivateRuntimeRollbackOnProbeFailure(t *testing.T) {
	stateRoot := t.TempDir()
	layout := NewRuntimeLayout("/install", stateRoot)
	manager := Manager{Layout: layout}
	if err := manager.writeCurrentVersion("2026.05.12"); err != nil {
		t.Fatalf("write current version: %v", err)
	}

	err := manager.ActivateVersion("2026.06.01", func(string) error { return assertErrProbeFailed })
	if err == nil {
		t.Fatal("expected activation to fail")
	}
	if got := manager.CurrentVersion(); got != "2026.05.12" {
		t.Fatalf("expected rollback to previous version, got %s", got)
	}
}

func TestActivateRuntimeSuccessUpdatesPointer(t *testing.T) {
	stateRoot := t.TempDir()
	layout := NewRuntimeLayout("/install", stateRoot)
	manager := Manager{Layout: layout}
	if err := manager.writeCurrentVersion("2026.05.12"); err != nil {
		t.Fatalf("write current version: %v", err)
	}

	versionDir, err := layout.VersionDir("2026.06.01")
	if err != nil {
		t.Fatalf("version dir: %v", err)
	}
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}

	if err := manager.ActivateVersion("2026.06.01", func(path string) error {
		if path != versionDir {
			t.Fatalf("unexpected probe path: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatalf("ActivateVersion returned error: %v", err)
	}
	if got := manager.CurrentVersion(); got != "2026.06.01" {
		t.Fatalf("expected updated current version, got %s", got)
	}
}
