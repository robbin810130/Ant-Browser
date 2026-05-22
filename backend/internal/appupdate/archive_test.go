package appupdate

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
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

func writeZipWithSymlink(t *testing.T, entries map[string]string, symlinks map[string]string) string {
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
	for name, target := range symlinks {
		header := &zip.FileHeader{Name: name}
		header.SetMode(os.ModeSymlink | 0o777)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("create symlink entry: %v", err)
		}
		if _, err := entry.Write([]byte(target)); err != nil {
			t.Fatalf("write symlink entry: %v", err)
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

func TestExtractFullPayloadExtractsSafeSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink extraction requires symlink support")
	}
	zipPath := writeZipWithSymlink(t,
		map[string]string{
			"Framework.framework/Versions/A/Resources/file.txt": "ok",
		},
		map[string]string{
			"Framework.framework/Resources": "Versions/A/Resources",
		},
	)
	dest := filepath.Join(t.TempDir(), "out")

	if err := ExtractFullPayload(zipPath, dest); err != nil {
		t.Fatalf("ExtractFullPayload returned error: %v", err)
	}
	target, err := os.Readlink(filepath.Join(dest, "Framework.framework", "Resources"))
	if err != nil {
		t.Fatalf("read symlink: %v", err)
	}
	if target != "Versions/A/Resources" {
		t.Fatalf("unexpected symlink target: %q", target)
	}
}

func TestExtractFullPayloadRejectsEscapingSymlink(t *testing.T) {
	zipPath := writeZipWithSymlink(t, nil, map[string]string{
		"Framework.framework/Resources": "../../escape",
	})
	if err := ExtractFullPayload(zipPath, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("expected escaping symlink rejection")
	}
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
	writeFakeWindowsPayload(t, dir)
	if err := ValidateStagedPayload("windows-amd64", dir); err != nil {
		t.Fatalf("ValidateStagedPayload returned error: %v", err)
	}
}

func TestValidateStagedWindowsPayloadRejectsSymlinkedStagedRoot(t *testing.T) {
	target := t.TempDir()
	writeFakeWindowsPayload(t, target)
	root := filepath.Join(t.TempDir(), "staged")
	if err := os.Symlink(target, root); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := ValidateStagedPayload("windows-amd64", root); err == nil {
		t.Fatal("expected symlinked staged root rejection")
	}
}

func TestValidateStagedWindowsPayloadRejectsSymlinkedMainExecutable(t *testing.T) {
	dir := t.TempDir()
	writeFakeWindowsPayload(t, dir)
	external := filepath.Join(t.TempDir(), "ant-chrome.exe")
	if err := os.WriteFile(external, []byte("MZ"), 0o600); err != nil {
		t.Fatalf("write external exe: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "ant-chrome.exe")); err != nil {
		t.Fatalf("remove exe: %v", err)
	}
	if err := os.Symlink(external, filepath.Join(dir, "ant-chrome.exe")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := ValidateStagedPayload("windows-amd64", dir); err == nil {
		t.Fatal("expected symlinked ant-chrome.exe rejection")
	}
}

func TestValidateStagedWindowsPayloadRejectsSymlinkedPublishDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFakeWindowsPayload(t, dir)
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "runtime-manifest.json"), []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write external manifest: %v", err)
	}
	publish := filepath.Join(dir, "publish")
	if err := os.RemoveAll(publish); err != nil {
		t.Fatalf("remove publish: %v", err)
	}
	if err := os.Symlink(external, publish); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := ValidateStagedPayload("windows-amd64", dir); err == nil {
		t.Fatal("expected symlinked publish directory rejection")
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

func writeFakeWindowsPayload(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "publish"), 0o755); err != nil {
		t.Fatalf("mkdir publish: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "ant-chrome.exe"), []byte("MZ"), 0o600); err != nil {
		t.Fatalf("write exe: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "publish", "runtime-manifest.json"), []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func TestValidateStagedWindowsPayloadRejectsRootDataDir(t *testing.T) {
	dir := t.TempDir()
	writeFakeWindowsPayload(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	if err := ValidateStagedPayload("windows-amd64", dir); err == nil {
		t.Fatal("expected root data dir rejection")
	}
}

func TestValidateStagedWindowsPayloadRejectsDatabaseFile(t *testing.T) {
	dir := t.TempDir()
	writeFakeWindowsPayload(t, dir)
	resources := filepath.Join(dir, "resources")
	if err := os.MkdirAll(resources, 0o755); err != nil {
		t.Fatalf("mkdir resources: %v", err)
	}
	if err := os.WriteFile(filepath.Join(resources, "fixture.db"), []byte("db"), 0o600); err != nil {
		t.Fatalf("write db: %v", err)
	}
	if err := ValidateStagedPayload("windows-amd64", dir); err == nil {
		t.Fatal("expected database file rejection")
	}
}

func TestValidateStagedWindowsPayloadAcceptsBundledResourceDataDir(t *testing.T) {
	dir := t.TempDir()
	writeFakeWindowsPayload(t, dir)
	resourceData := filepath.Join(dir, "resources", "data")
	if err := os.MkdirAll(resourceData, 0o755); err != nil {
		t.Fatalf("mkdir resource data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(resourceData, "fixture.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := ValidateStagedPayload("windows-amd64", dir); err != nil {
		t.Fatalf("ValidateStagedPayload returned error: %v", err)
	}
}

func TestValidateStagedPayloadRejectsUserDataCookies(t *testing.T) {
	dir := t.TempDir()
	writeFakeWindowsPayload(t, dir)
	userDataDefault := filepath.Join(dir, "User Data", "Default")
	if err := os.MkdirAll(userDataDefault, 0o755); err != nil {
		t.Fatalf("mkdir User Data profile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDataDefault, "Cookies"), []byte("cookies"), 0o600); err != nil {
		t.Fatalf("write Cookies: %v", err)
	}
	if err := ValidateStagedPayload("windows-amd64", dir); err == nil {
		t.Fatal("expected User Data cookies rejection")
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

func skipOnWindowsForExecutableBits(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not reliably preserve POSIX executable bits in this test fixture")
	}
}

func TestValidateStagedPayloadAcceptsDarwinBundle(t *testing.T) {
	skipOnWindowsForExecutableBits(t)
	root := t.TempDir()
	writeFakeDarwinBundle(t, root)
	if err := ValidateStagedPayload("darwin-arm64", root); err != nil {
		t.Fatalf("ValidateStagedPayload returned error: %v", err)
	}
}

func TestValidateStagedPayloadRejectsDarwinSymlinkedAppBundle(t *testing.T) {
	externalRoot := t.TempDir()
	externalApp := writeFakeDarwinBundle(t, externalRoot)
	root := t.TempDir()
	link := filepath.Join(root, "Ant Browser.app")
	if err := os.Symlink(externalApp, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err == nil {
		t.Fatal("expected symlinked app bundle rejection")
	}
}

func TestValidateStagedPayloadRejectsDarwinSymlinkedContentsDirectory(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	externalRoot := t.TempDir()
	externalApp := writeFakeDarwinBundle(t, externalRoot)
	contents := filepath.Join(appRoot, "Contents")
	if err := os.RemoveAll(contents); err != nil {
		t.Fatalf("remove Contents: %v", err)
	}
	if err := os.Symlink(filepath.Join(externalApp, "Contents"), contents); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err == nil {
		t.Fatal("expected symlinked Contents directory rejection")
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

func TestValidateStagedPayloadRejectsDarwinSymlinkedInfoPlist(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	external := filepath.Join(t.TempDir(), "Info.plist")
	if err := os.WriteFile(external, []byte(`<plist></plist>`), 0o600); err != nil {
		t.Fatalf("write external Info.plist: %v", err)
	}
	infoPlist := filepath.Join(appRoot, "Contents", "Info.plist")
	if err := os.Remove(infoPlist); err != nil {
		t.Fatalf("remove Info.plist: %v", err)
	}
	if err := os.Symlink(external, infoPlist); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err == nil {
		t.Fatal("expected symlinked Info.plist rejection")
	}
}

func TestValidateStagedPayloadRejectsDarwinNonExecutableMainBinary(t *testing.T) {
	skipOnWindowsForExecutableBits(t)
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

func TestValidateStagedPayloadRejectsDarwinSymlinkedMainBinary(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	external := filepath.Join(t.TempDir(), "ant-chrome")
	if err := os.WriteFile(external, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write external ant-chrome: %v", err)
	}
	mainBinary := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome")
	if err := os.Remove(mainBinary); err != nil {
		t.Fatalf("remove ant-chrome: %v", err)
	}
	if err := os.Symlink(external, mainBinary); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err == nil {
		t.Fatal("expected symlinked ant-chrome rejection")
	}
}

func TestValidateStagedPayloadRejectsDarwinSymlinkedBinDirectory(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	externalRoot := t.TempDir()
	externalApp := writeFakeDarwinBundle(t, externalRoot)
	bin := filepath.Join(appRoot, "Contents", "MacOS", "bin")
	if err := os.RemoveAll(bin); err != nil {
		t.Fatalf("remove bin: %v", err)
	}
	if err := os.Symlink(filepath.Join(externalApp, "Contents", "MacOS", "bin"), bin); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err == nil {
		t.Fatal("expected symlinked bin directory rejection")
	}
}

func TestValidateStagedPayloadRejectsDarwinMutableUserData(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	dataDir := filepath.Join(appRoot, "Contents", "MacOS", "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err == nil {
		t.Fatal("expected mutable user data rejection")
	}
}

func TestValidateStagedPayloadRejectsDarwinSQLiteOutsideDataDir(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	cacheDir := filepath.Join(appRoot, "Contents", "Resources", "fixtures")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir fixtures: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "app.sqlite"), []byte("db"), 0o600); err != nil {
		t.Fatalf("write sqlite: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err == nil {
		t.Fatal("expected sqlite file rejection")
	}
}

func TestValidateStagedPayloadAcceptsDarwinBundledResourceDataDir(t *testing.T) {
	skipOnWindowsForExecutableBits(t)
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	resourceData := filepath.Join(appRoot, "Contents", "Resources", "data")
	if err := os.MkdirAll(resourceData, 0o755); err != nil {
		t.Fatalf("mkdir resource data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(resourceData, "fixture.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := ValidateStagedPayload("darwin-arm64", root); err != nil {
		t.Fatalf("ValidateStagedPayload returned error: %v", err)
	}
}
