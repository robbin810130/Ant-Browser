package release

import "testing"

func TestRuntimeLayoutPaths(t *testing.T) {
	layout := NewRuntimeLayout("/install/root", "/state/root")
	if got := layout.ActivePointerPath(); got != "/state/root/runtime/current.json" {
		t.Fatalf("unexpected active pointer path: %s", got)
	}
	if got := layout.VersionDir("2026.05.12"); got != "/state/root/runtime/versions/2026.05.12" {
		t.Fatalf("unexpected version dir: %s", got)
	}
}
