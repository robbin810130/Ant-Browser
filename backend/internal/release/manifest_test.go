package release

import "testing"

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

func TestManifestDetectsResourceVersionBelowFloor(t *testing.T) {
	manifest := Manifest{MinimumResourceVersion: "2026.05.12"}
	if manifest.ResourceCompatible("2026.05.01") {
		t.Fatal("expected old resource version to be incompatible")
	}
}
