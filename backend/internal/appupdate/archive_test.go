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

func TestValidateStagedPayloadRejectsMacTargetsForPhaseOne(t *testing.T) {
	if err := ValidateStagedPayload("darwin-arm64", t.TempDir()); err == nil {
		t.Fatal("expected macOS target rejection")
	}
}
