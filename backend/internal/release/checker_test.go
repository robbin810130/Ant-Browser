package release

import (
	"os"
	"path/filepath"
	goruntime "runtime"
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
	if result.State != StatePass {
		t.Fatalf("expected pass state, got %s with items %#v", result.State, result.Items)
	}
}

func TestCheckerPassesWhenRuntimeIsHealthy(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "runtime-manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	coreDir := filepath.Join(dir, "core")
	if err := os.MkdirAll(coreDir, 0o755); err != nil {
		t.Fatalf("mkdir core dir: %v", err)
	}
	exePath := filepath.Join(coreDir, coreExecutableCandidateForTest())
	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("mkdir executable dir: %v", err)
	}
	if err := os.WriteFile(exePath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}

	checker := Checker{Manifest: Manifest{
		MinimumResourceVersion: "2026.05.12",
		Packages:               []RuntimePackage{{ID: "desktop-core", Target: DefaultTarget(), Kind: "browser-core", Required: true}},
	}}
	result := checker.Run(CheckInput{
		ManifestPath:    manifestPath,
		Target:          DefaultTarget(),
		ResourceVersion: "2026.05.12",
		BrowserCorePath: coreDir,
	})
	if result.State != StatePass {
		t.Fatalf("expected pass state, got %s with items %#v", result.State, result.Items)
	}
}

func TestCheckerBlocksWhenTargetPackagesMissing(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "runtime-manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	checker := Checker{Manifest: Manifest{
		MinimumResourceVersion: "2026.05.12",
		Packages:               []RuntimePackage{{ID: "other-core", Target: "other-target", Kind: "browser-core", Required: true}},
	}}
	result := checker.Run(CheckInput{
		ManifestPath:    manifestPath,
		Target:          DefaultTarget(),
		ResourceVersion: "2026.05.12",
	})
	if result.State != StateBlocked {
		t.Fatalf("expected blocked state, got %s", result.State)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "PKG-TARGET-MISSING" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
}

func TestCheckerMarksOutdatedResourceRepairable(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "runtime-manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	checker := Checker{Manifest: Manifest{
		MinimumResourceVersion: "2026.05.12",
		Packages:               []RuntimePackage{{ID: "desktop-core", Target: DefaultTarget(), Kind: "browser-core", Required: true}},
	}}
	result := checker.Run(CheckInput{
		ManifestPath:    manifestPath,
		Target:          DefaultTarget(),
		ResourceVersion: "2026.05.01",
	})
	if result.State != StateRepairable {
		t.Fatalf("expected repairable state, got %s", result.State)
	}
	if len(result.Items) == 0 || result.Items[0].Code != "PKG-RESOURCE-OUTDATED" {
		t.Fatalf("unexpected failure items: %#v", result.Items)
	}
}

func TestCheckerDoesNotBlockStartupWhenBrowserCoreMissing(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "runtime-manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	checker := Checker{Manifest: Manifest{
		MinimumResourceVersion: "2026.05.12",
		Packages:               []RuntimePackage{{ID: "desktop-core", Target: DefaultTarget(), Kind: "browser-core", Required: true}},
	}}
	result := checker.Run(CheckInput{
		ManifestPath:    manifestPath,
		Target:          DefaultTarget(),
		ResourceVersion: "2026.05.12",
	})
	if result.State != StatePass {
		t.Fatalf("expected pass state, got %s with items %#v", result.State, result.Items)
	}
}

func TestResolvePackagePathRejectsEscapingPath(t *testing.T) {
	versionDir := filepath.Join(t.TempDir(), "runtime", "versions", "2026.05.12")
	got := ResolvePackagePath(versionDir, RuntimePackage{Path: "../escape"})
	if got != "" {
		t.Fatalf("expected escaping package path to be rejected, got %q", got)
	}
}

func coreExecutableCandidateForTest() string {
	switch goruntime.GOOS {
	case "windows":
		return "chrome.exe"
	case "darwin":
		return filepath.FromSlash("Chromium.app/Contents/MacOS/Chromium")
	default:
		return "chrome"
	}
}
