package release

import (
	"path/filepath"
	"testing"
)

func TestRuntimeLayoutPaths(t *testing.T) {
	layout := NewRuntimeLayout("/install/root", "/state/root")
	if got := layout.ActivePointerPath(); got != "/state/root/runtime/current.json" {
		t.Fatalf("unexpected active pointer path: %s", got)
	}
	if got, err := layout.VersionDir("2026.05.12"); err != nil || got != "/state/root/runtime/versions/2026.05.12" {
		t.Fatalf("unexpected version dir: got=%s err=%v", got, err)
	}
}

func TestRuntimeLayoutVersionDirRejectsUnsafeNames(t *testing.T) {
	layout := NewRuntimeLayout("/install/root", "/state/root")
	for _, version := range []string{"", ".", "..", "2026/05/12", "2026\\05\\12", "../escape"} {
		if got, err := layout.VersionDir(version); err == nil {
			t.Fatalf("expected version %q to be rejected, got=%s", version, got)
		}
	}
}

func TestRuntimeLayoutCoreRoots(t *testing.T) {
	layout := NewRuntimeLayout("/install/root", "/state/root")
	if got := layout.RuntimeRoot(); got != filepath.Join("/state/root", "runtime") {
		t.Fatalf("unexpected runtime root: %s", got)
	}
	if got := layout.VersionsRoot(); got != filepath.Join("/state/root", "runtime", "versions") {
		t.Fatalf("unexpected versions root: %s", got)
	}
	if got := layout.StagingRoot(); got != filepath.Join("/state/root", "runtime", "staging") {
		t.Fatalf("unexpected staging root: %s", got)
	}
	if got := layout.DiagnosticsRoot(); got != filepath.Join("/state/root", "diagnostics") {
		t.Fatalf("unexpected diagnostics root: %s", got)
	}
}
