package release

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckerMarksMissingManifestBlocked(t *testing.T) {
	checker := Checker{}
	result := checker.Run(CheckInput{
		ManifestPath: "/missing/runtime-manifest.json",
		Target:       "darwin-arm64",
	})
	if result.State != StateBlocked {
		t.Fatalf("expected blocked state, got %s", result.State)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "ENV-MANIFEST-MISSING" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
}

func TestCheckerMarksMissingRuntimeRepairable(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "runtime-manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	checker := Checker{Manifest: Manifest{
		MinimumResourceVersion: "2026.05.12",
		Packages:               []RuntimePackage{{ID: "mac-core", Target: "darwin-arm64", Kind: "browser-core", Required: true}},
	}}
	result := checker.Run(CheckInput{
		ManifestPath:    manifestPath,
		Target:          "darwin-arm64",
		ResourceVersion: "2026.05.12",
	})
	if result.State != StateRepairable {
		t.Fatalf("expected repairable state, got %s", result.State)
	}
}
