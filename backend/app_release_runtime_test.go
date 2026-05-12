package backend

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/release"
)

func TestGetDesktopEnvironmentStatus(t *testing.T) {
	root := t.TempDir()
	layout := RuntimeReleaseLayout(root)
	manifestPath := filepath.Clean(filepath.Join(root, "publish", "runtime-manifest.json"))
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte(`{
		"schemaVersion": 2,
		"appVersion": "1.0.0",
		"minimumResourceVersion": "2026.05.12",
		"packages": [
			{"id":"desktop-core","target":"`+release.DefaultTarget()+`","kind":"browser-core","required":true,"version":"136.0.0","path":"core"}
		]
	}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	versionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("version dir: %v", err)
	}
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":"2026.05.12","resourceVersion":"2026.05.12"}`), 0o600); err != nil {
		t.Fatalf("write active runtime pointer: %v", err)
	}

	app := NewApp(root)
	app.ctx = context.Background()
	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StateRepairable {
		t.Fatalf("expected repairable state, got %s", result.State)
	}
}
