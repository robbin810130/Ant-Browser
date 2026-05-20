package appupdate

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(file)
	for name, body := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
	return path
}

func TestExtractFullPayloadRejectsZipSlip(t *testing.T) {
	zipPath := writeZip(t, map[string]string{"../escape.txt": "bad"})
	if err := ExtractFullPayload(zipPath, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("expected zip slip rejection")
	}
}

func TestExtractFullPayloadExtractsFiles(t *testing.T) {
	zipPath := writeZip(t, map[string]string{
		"ant-chrome.exe":                  "MZ",
		"publish/runtime-manifest.json":   `{"schemaVersion":2}`,
		"resources/app-update-marker.txt": "ok",
	})
	dest := filepath.Join(t.TempDir(), "out")

	if err := ExtractFullPayload(zipPath, dest); err != nil {
		t.Fatalf("ExtractFullPayload returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "resources", "app-update-marker.txt"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(data) != "ok" {
		t.Fatalf("unexpected extracted file content: %q", string(data))
	}
	assertDirMode(t, dest, 0o700)
}

func TestExtractFullPayloadHandlesFileDirectoryConflict(t *testing.T) {
	zipPath := writeZip(t, map[string]string{
		"publish":                       "file that blocks publish directory",
		"publish/runtime-manifest.json": `{"schemaVersion":2}`,
		"ant-chrome.exe":                "MZ",
	})
	dest := filepath.Join(t.TempDir(), "out")

	if err := ExtractFullPayload(zipPath, dest); err != nil {
		t.Fatalf("ExtractFullPayload returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "publish", "runtime-manifest.json"))
	if err != nil {
		t.Fatalf("read runtime manifest: %v", err)
	}
	if string(data) != `{"schemaVersion":2}` {
		t.Fatalf("unexpected runtime manifest: %q", string(data))
	}
}

func TestValidateStagedWindowsPayloadRequiresCoreFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "publish"), 0o755); err != nil {
		t.Fatalf("mkdir publish: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ant-chrome.exe"), []byte("MZ"), 0o600); err != nil {
		t.Fatalf("write exe: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "publish", "runtime-manifest.json"), []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := ValidateStagedPayload("windows-amd64", dir); err != nil {
		t.Fatalf("ValidateStagedPayload returned error: %v", err)
	}
}

func TestValidateStagedWindowsPayloadRejectsMissingCoreFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ant-chrome.exe"), []byte("MZ"), 0o600); err != nil {
		t.Fatalf("write exe: %v", err)
	}
	if err := ValidateStagedPayload("windows-amd64", dir); err == nil {
		t.Fatal("expected missing runtime manifest error")
	}
}

func writeFakeDarwinBundle(t *testing.T, root string) string {
	t.Helper()
	appRoot := filepath.Join(root, "Ant Browser.app")
	macos := filepath.Join(appRoot, "Contents", "MacOS")
	if err := os.MkdirAll(filepath.Join(macos, "publish"), 0o755); err != nil {
		t.Fatalf("mkdir publish: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(macos, "bin"), 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appRoot, "Contents", "Info.plist"), []byte(`<plist></plist>`), 0o600); err != nil {
		t.Fatalf("write Info.plist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macos, "ant-chrome"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write ant-chrome: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macos, "publish", "runtime-manifest.json"), []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write runtime manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macos, "bin", "xray"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write xray: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macos, "bin", "sing-box"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write sing-box: %v", err)
	}
	return appRoot
}

func TestValidateStagedPayloadAcceptsDarwinBundle(t *testing.T) {
	root := t.TempDir()
	writeFakeDarwinBundle(t, root)
	if err := ValidateStagedPayload("darwin-arm64", root); err != nil {
		t.Fatalf("ValidateStagedPayload returned error: %v", err)
	}
}

func TestValidateStagedPayloadRejectsDarwinMissingInfoPlist(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	if err := os.Remove(filepath.Join(appRoot, "Contents", "Info.plist")); err != nil {
		t.Fatalf("remove Info.plist: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err == nil {
		t.Fatal("expected missing Info.plist error")
	}
}

func TestValidateStagedPayloadRejectsDarwinNonExecutableMainBinary(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	mainBinary := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome")
	if err := os.Chmod(mainBinary, 0o600); err != nil {
		t.Fatalf("chmod ant-chrome: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err == nil {
		t.Fatal("expected non-executable ant-chrome error")
	}
}

func TestValidateStagedPayloadRejectsDarwinMutableUserData(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	dataDir := filepath.Join(appRoot, "Contents", "MacOS", "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "app.sqlite"), []byte("db"), 0o600); err != nil {
		t.Fatalf("write sqlite: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err == nil {
		t.Fatal("expected mutable user data rejection")
	}
}
