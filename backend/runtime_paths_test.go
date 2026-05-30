package backend

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"
)

func TestRuntimeReleaseLayoutBridgesAppRoots(t *testing.T) {
	appRoot := t.TempDir()
	layout := RuntimeReleaseLayout(appRoot)
	wantRoot := filepath.Clean(appRoot)

	if layout.InstallRoot != wantRoot {
		t.Fatalf("unexpected install root: got=%s want=%s", layout.InstallRoot, wantRoot)
	}
	if layout.StateRoot != wantRoot {
		t.Fatalf("unexpected state root: got=%s want=%s", layout.StateRoot, wantRoot)
	}
	if got := layout.RuntimeRoot(); got != filepath.Join(wantRoot, "runtime") {
		t.Fatalf("unexpected runtime root: %s", got)
	}
	if got := layout.VersionsRoot(); got != filepath.Join(wantRoot, "runtime", "versions") {
		t.Fatalf("unexpected versions root: %s", got)
	}
	if got := layout.StagingRoot(); got != filepath.Join(wantRoot, "runtime", "staging") {
		t.Fatalf("unexpected staging root: %s", got)
	}
	if got := layout.DiagnosticsRoot(); got != filepath.Join(wantRoot, "diagnostics") {
		t.Fatalf("unexpected diagnostics root: %s", got)
	}

	app := NewApp(appRoot)
	if got := app.runtimeLayout(); got != layout {
		t.Fatalf("unexpected app runtime layout: got=%#v want=%#v", got, layout)
	}
	if got := app.appRootAbs(); got != wantRoot {
		t.Fatalf("unexpected app root abs: got=%s want=%s", got, wantRoot)
	}
	if got := app.appStateRootAbs(); got != wantRoot {
		t.Fatalf("unexpected app state root abs: got=%s want=%s", got, wantRoot)
	}
}

func TestRuntimeReleaseLayoutDetachedState(t *testing.T) {
	if goruntime.GOOS != "linux" {
		t.Skip("linux-only detached state behavior")
	}

	xdgDataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDataHome)

	installRoot := filepath.Join(t.TempDir(), "opt-app")
	if err := os.MkdirAll(filepath.Join(installRoot, "bin"), 0o755); err != nil {
		t.Fatalf("create install root: %v", err)
	}
	if err := os.Chmod(installRoot, 0o555); err != nil {
		t.Fatalf("chmod install root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(installRoot, 0o755)
		_ = os.Chmod(filepath.Join(installRoot, "bin"), 0o755)
	})

	layout := RuntimeReleaseLayout(installRoot)
	wantStateRoot := filepath.Join(xdgDataHome, "ant-browser")
	if layout.InstallRoot != filepath.Clean(installRoot) {
		t.Fatalf("unexpected detached install root: got=%s want=%s", layout.InstallRoot, filepath.Clean(installRoot))
	}
	if layout.StateRoot != wantStateRoot {
		t.Fatalf("unexpected detached state root: got=%s want=%s", layout.StateRoot, wantStateRoot)
	}
	if got := layout.RuntimeRoot(); got != filepath.Join(wantStateRoot, "runtime") {
		t.Fatalf("unexpected detached runtime root: %s", got)
	}
	if got := layout.VersionsRoot(); got != filepath.Join(wantStateRoot, "runtime", "versions") {
		t.Fatalf("unexpected detached versions root: %s", got)
	}
	if got := layout.StagingRoot(); got != filepath.Join(wantStateRoot, "runtime", "staging") {
		t.Fatalf("unexpected detached staging root: %s", got)
	}
	if got := layout.DiagnosticsRoot(); got != filepath.Join(wantStateRoot, "diagnostics") {
		t.Fatalf("unexpected detached diagnostics root: %s", got)
	}
}
