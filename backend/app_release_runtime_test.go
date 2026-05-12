package backend

import (
	"ant-chrome/backend/internal/browser"
	"context"
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/release"
)

func TestGetDesktopEnvironmentStatusCurrentPointerMissing(t *testing.T) {
	root := t.TempDir()
	_ = writeRuntimeManifestFixture(t, root)
	app := newRuntimeStatusTestApp(t, root)
	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StateRepairable {
		t.Fatalf("expected repairable state, got %s", result.State)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-RUNTIME-POINTER-MISSING" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
}

func TestGetDesktopEnvironmentStatusCurrentPointerInvalid(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":`), 0o600); err != nil {
		t.Fatalf("write invalid active runtime pointer: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StateRepairable {
		t.Fatalf("expected repairable state, got %s", result.State)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-RUNTIME-POINTER-INVALID" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
}

func TestGetDesktopEnvironmentStatusCurrentPointerMissingFields(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":""}`), 0o600); err != nil {
		t.Fatalf("write invalid active runtime pointer: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StateRepairable {
		t.Fatalf("expected repairable state, got %s", result.State)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-RUNTIME-POINTER-INVALID" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
}

func TestGetDesktopEnvironmentStatusPassesWhenPointerAndCoreAreHealthy(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	versionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("version dir: %v", err)
	}
	coreDir := filepath.Join(versionDir, "core")
	writeCoreFixture(t, coreDir)
	if err := os.WriteFile(layout.ActivePointerPath(), []byte(`{"version":"2026.05.12","resourceVersion":"2026.05.12"}`), 0o600); err != nil {
		t.Fatalf("write active runtime pointer: %v", err)
	}

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.GetDesktopEnvironmentStatus()
	if err != nil {
		t.Fatalf("GetDesktopEnvironmentStatus returned error: %v", err)
	}
	if result.State != release.StatePass {
		t.Fatalf("expected pass state, got %s with items %#v", result.State, result.Items)
	}
}

func TestRepairDesktopEnvironmentRepairsMissingPointer(t *testing.T) {
	root := t.TempDir()
	layout := writeRuntimeManifestFixture(t, root)
	versionDir, err := layout.VersionDir("2026.05.12")
	if err != nil {
		t.Fatalf("version dir: %v", err)
	}
	writeCoreFixture(t, filepath.Join(versionDir, "core"))

	app := newRuntimeStatusTestApp(t, root)
	result, err := app.RepairDesktopEnvironment()
	if err != nil {
		t.Fatalf("RepairDesktopEnvironment returned error: %v", err)
	}
	if result.State != release.StatePass {
		t.Fatalf("expected pass state after repair, got %s with items %#v", result.State, result.Items)
	}

	data, err := os.ReadFile(layout.ActivePointerPath())
	if err != nil {
		t.Fatalf("read active runtime pointer: %v", err)
	}
	expected := `{"version":"2026.05.12","resourceVersion":"2026.05.12"}`
	if string(data) != expected {
		t.Fatalf("unexpected active runtime pointer content: %s", string(data))
	}
}

func newRuntimeStatusTestApp(t *testing.T, root string) *App {
	t.Helper()
	app := NewApp(root)
	app.ctx = context.Background()
	return app
}

func writeRuntimeManifestFixture(t *testing.T, root string) release.RuntimeLayout {
	t.Helper()
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
	if err := os.MkdirAll(filepath.Dir(layout.ActivePointerPath()), 0o755); err != nil {
		t.Fatalf("mkdir runtime root: %v", err)
	}
	return layout
}

func writeCoreFixture(t *testing.T, coreDir string) {
	t.Helper()
	exePath := filepath.Join(coreDir, coreExecutableCandidateForTest())
	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("mkdir executable dir: %v", err)
	}
	if err := os.WriteFile(exePath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}

func coreExecutableCandidateForTest() string {
	return filepath.FromSlash(browser.CoreExecutableCandidates()[0])
}
