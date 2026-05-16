package appupdate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLayoutPaths(t *testing.T) {
	installRoot := filepath.Join(t.TempDir(), "install", "..", "install")
	stateRoot := filepath.Join(t.TempDir(), "state", ".", "root")

	layout := NewLayout(installRoot, stateRoot)

	if layout.InstallRoot != filepath.Clean(installRoot) {
		t.Fatalf("InstallRoot not cleaned: got=%q want=%q", layout.InstallRoot, filepath.Clean(installRoot))
	}
	if layout.StateRoot != filepath.Clean(stateRoot) {
		t.Fatalf("StateRoot not cleaned: got=%q want=%q", layout.StateRoot, filepath.Clean(stateRoot))
	}

	root := filepath.Join(filepath.Clean(stateRoot), "app-update")
	assertPath(t, layout.Root(), root)
	assertPath(t, layout.StatePath(), filepath.Join(root, "state.json"))
	assertPath(t, layout.PlanPath(), filepath.Join(root, "update-plan.json"))
	assertPath(t, layout.DownloadsRoot(), filepath.Join(root, "downloads"))
	assertPath(t, layout.StagingRoot(), filepath.Join(root, "staging"))
	assertPath(t, layout.BackupsRoot(), filepath.Join(root, "backups"))
	assertPath(t, layout.RunnerRoot(), filepath.Join(root, "runner"))
	assertPath(t, layout.LogsRoot(), filepath.Join(root, "logs"))
}

func TestWriteAndReadPersistentState(t *testing.T) {
	layout := NewLayout(filepath.Join(t.TempDir(), "install"), filepath.Join(t.TempDir(), "state"))
	state := PersistentState{
		Status:           PersistentStatusDownloading,
		LocalAppVersion:  "1.0.0",
		RemoteAppVersion: "1.1.0",
		ManifestSource:   "config.yaml",
		ManifestURL:      "https://updates.example.test/app-update.json",
		PayloadURL:       "https://updates.example.test/app.zip",
		Target:           "windows-x64",
		PlanPath:         layout.PlanPath(),
		LogPath:          filepath.Join(layout.LogsRoot(), "apply.log"),
		BackupPath:       filepath.Join(layout.BackupsRoot(), "backup.zip"),
		LastError: ErrorInfo{
			Code:    "download_failed",
			Message: "download failed",
			Details: map[string]string{
				"status": "503",
			},
		},
	}

	if err := WriteState(layout, state); err != nil {
		t.Fatalf("WriteState returned error: %v", err)
	}

	data, err := os.ReadFile(layout.StatePath())
	if err != nil {
		t.Fatalf("state file was not written: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatalf("state file should end with newline")
	}
	if !strings.Contains(string(data), "\n  \"status\": \"downloading\"") {
		t.Fatalf("state file should be pretty JSON: %s", data)
	}

	info, err := os.Stat(layout.StatePath())
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("state file permissions incorrect: got=%#o want=%#o", got, 0o600)
	}

	read, err := ReadState(layout)
	if err != nil {
		t.Fatalf("ReadState returned error: %v", err)
	}
	if read.Status != PersistentStatusDownloading {
		t.Fatalf("status incorrect: got=%q", read.Status)
	}
	if read.LocalAppVersion != state.LocalAppVersion || read.RemoteAppVersion != state.RemoteAppVersion {
		t.Fatalf("versions not preserved: got=%+v", read)
	}
	if read.LastError.Code != state.LastError.Code || read.LastError.Details["status"] != "503" {
		t.Fatalf("last error not preserved: got=%+v", read.LastError)
	}
	if read.UpdatedAt == "" {
		t.Fatalf("UpdatedAt should be set")
	}
	if _, err := time.Parse(time.RFC3339, read.UpdatedAt); err != nil {
		t.Fatalf("UpdatedAt should be RFC3339: %q", read.UpdatedAt)
	}
}

func TestWriteAndReadApplyPlan(t *testing.T) {
	layout := NewLayout(filepath.Join(t.TempDir(), "install"), filepath.Join(t.TempDir(), "state"))
	plan := ApplyPlan{
		InstallRoot:      layout.InstallRoot,
		StateRoot:        layout.StateRoot,
		Target:           "windows-x64",
		OldAppVersion:    "1.0.0",
		NewAppVersion:    "1.1.0",
		StagedPath:       filepath.Join(layout.StagingRoot(), "payload.zip"),
		BackupPath:       filepath.Join(layout.BackupsRoot(), "backup.zip"),
		CurrentExePath:   filepath.Join(layout.InstallRoot, "AntBrowser.exe"),
		ExpectedSHA256:   validSHA256,
		ManifestSource:   "runtime-config",
		ManifestURL:      "https://updates.example.test/app-update.json",
		PayloadURL:       "https://updates.example.test/app.zip",
		WaitForProcessID: 1234,
	}

	path, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}
	assertPath(t, path, layout.PlanPath())

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("plan file was not written: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatalf("plan file should end with newline")
	}
	if !strings.Contains(string(data), "\n  \"expectedSHA256\": \""+validSHA256+"\"") {
		t.Fatalf("plan file should be pretty JSON: %s", data)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat plan file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("plan file permissions incorrect: got=%#o want=%#o", got, 0o600)
	}

	read, err := ReadPlan(path)
	if err != nil {
		t.Fatalf("ReadPlan returned error: %v", err)
	}
	if read.Target != plan.Target || read.ExpectedSHA256 != plan.ExpectedSHA256 {
		t.Fatalf("plan not preserved: got=%+v", read)
	}
	if read.WaitForProcessID != 1234 {
		t.Fatalf("WaitForProcessID incorrect: got=%d", read.WaitForProcessID)
	}
}

func assertPath(t *testing.T, got string, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("path incorrect: got=%q want=%q", got, want)
	}
}
