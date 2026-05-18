package backend

import (
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/appupdate"
)

func TestGetDesktopAppUpdateStateReturnsIdleWhenMissing(t *testing.T) {
	app := NewApp(t.TempDir(), "1.1.0")
	state, err := app.GetDesktopAppUpdateState()
	if err != nil {
		t.Fatalf("GetDesktopAppUpdateState returned error: %v", err)
	}
	if state.Kind != appupdate.UpdateKindNone {
		t.Fatalf("expected none, got %+v", state)
	}
	if state.LocalAppVersion != "1.1.0" {
		t.Fatalf("expected local version, got %+v", state)
	}
}

func TestClearDesktopAppUpdateFailureRemovesState(t *testing.T) {
	root := t.TempDir()
	app := NewApp(root, "1.1.0")
	layout := app.appUpdateLayout()
	if err := os.MkdirAll(filepath.Dir(layout.StatePath()), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(layout.StatePath(), []byte(`{"status":"failed_manual_repair"}`), 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}
	if err := app.ClearDesktopAppUpdateFailure(); err != nil {
		t.Fatalf("ClearDesktopAppUpdateFailure returned error: %v", err)
	}
	if _, err := os.Stat(layout.StatePath()); !os.IsNotExist(err) {
		t.Fatalf("expected state file removed, err=%v", err)
	}
}

func TestDownloadDesktopAppUpdateUsesManager(t *testing.T) {
	app := NewApp(t.TempDir(), "1.1.0")
	state, err := app.DownloadDesktopAppUpdate()
	if err != nil {
		t.Fatalf("DownloadDesktopAppUpdate returned error: %v", err)
	}
	if state.Kind != appupdate.UpdateKindFailed || state.ErrorCode != "APP-UPDATE-MANIFEST-LOAD-FAILED" {
		t.Fatalf("expected missing manifest source failure state, got %+v", state)
	}
}
