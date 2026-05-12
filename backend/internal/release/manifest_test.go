package release

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifestSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(`{
		"schemaVersion": 2,
		"appVersion": "1.2.3",
		"minimumResourceVersion": "2026.05.12",
		"packages": [
			{"id":"pkg-1","target":"windows-amd64","kind":"runtime-binary","required":true,"version":"1.0.0","sha256":"abc","path":"bin/a"}
		]
	}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest returned error: %v", err)
	}
	if manifest.SchemaVersion != 2 {
		t.Fatalf("expected schemaVersion 2, got %d", manifest.SchemaVersion)
	}
	if len(manifest.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(manifest.Packages))
	}
}

func TestLoadManifestRejectsUnsupportedSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(`{
		"schemaVersion": 1,
		"packages": []
	}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected LoadManifest to reject unsupported schemaVersion")
	}
}

func TestManifestSelectPackagesForTarget(t *testing.T) {
	manifest := Manifest{
		AppVersion:             "1.2.0",
		MinimumResourceVersion: "2026.05.12",
		Packages: []RuntimePackage{
			{ID: "win-core", Target: "windows-amd64", Kind: "browser-core", Required: true, Version: "136.0.0"},
			{ID: "mac-core", Target: "darwin-arm64", Kind: "browser-core", Required: true, Version: "136.0.0"},
		},
	}

	pkgs, err := manifest.RequiredPackages("windows-amd64")
	if err != nil {
		t.Fatalf("RequiredPackages returned error: %v", err)
	}
	if len(pkgs) != 1 || pkgs[0].ID != "win-core" {
		t.Fatalf("expected win-core package, got %#v", pkgs)
	}
}

func TestRequiredPackagesRejectsEmptyTarget(t *testing.T) {
	manifest := Manifest{
		Packages: []RuntimePackage{
			{ID: "win-core", Target: "windows-amd64", Required: true},
		},
	}

	if _, err := manifest.RequiredPackages(""); err == nil {
		t.Fatal("expected empty target to fail")
	}
}

func TestRequiredPackagesRejectsMissingTarget(t *testing.T) {
	manifest := Manifest{
		Packages: []RuntimePackage{
			{ID: "win-core", Target: "windows-amd64", Required: true},
		},
	}

	if _, err := manifest.RequiredPackages("darwin-arm64"); err == nil {
		t.Fatal("expected missing target to fail")
	}
}

func TestManifestDetectsResourceVersionBelowFloor(t *testing.T) {
	manifest := Manifest{MinimumResourceVersion: "2026.05.12"}
	if manifest.ResourceCompatible("2026.05.01") {
		t.Fatal("expected old resource version to be incompatible")
	}
}

func TestManifestResourceCompatibleHandlesNonZeroPaddedVersions(t *testing.T) {
	manifest := Manifest{MinimumResourceVersion: "2026.05.12"}
	if !manifest.ResourceCompatible("2026.5.12") {
		t.Fatal("expected non-zero-padded version to be compatible")
	}
	if manifest.ResourceCompatible("2026.5.11") {
		t.Fatal("expected lower non-zero-padded version to be incompatible")
	}
	if manifest.ResourceCompatible("2026.05.bad") {
		t.Fatal("expected malformed version to be incompatible")
	}
}

func TestLoadCurrentRuntimeManifest(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "..", "publish", "runtime-manifest.json"))
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest returned error: %v", err)
	}
	if manifest.SchemaVersion != 2 {
		t.Fatalf("expected schemaVersion 2, got %d", manifest.SchemaVersion)
	}
	if len(manifest.Packages) == 0 {
		t.Fatal("expected manifest packages to be populated")
	}

	pkgs, err := manifest.RequiredPackages("darwin-arm64")
	if err != nil {
		pkgs, err = manifest.RequiredPackages("windows-amd64")
	}
	if err != nil {
		t.Fatalf("expected at least one required package for darwin-arm64 or windows-amd64: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("expected at least one required package")
	}
}
