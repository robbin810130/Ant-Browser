package appupdate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type fakePlatform struct {
	target        string
	validateError error
	preparedPlan  ApplyPlan
	spawned       bool
}

func (f *fakePlatform) Target() string { return f.target }
func (f *fakePlatform) ValidateInstallMode(Layout) error {
	return f.validateError
}
func (f *fakePlatform) PrepareApply(plan ApplyPlan) error {
	f.preparedPlan = plan
	return nil
}
func (f *fakePlatform) SpawnApplyRunner(string) error {
	f.spawned = true
	return nil
}
func (f *fakePlatform) RunApply(string) error        { return nil }
func (f *fakePlatform) PostUpdateCheck(string) error { return nil }

func TestManagerCheckClassifiesUnsupportedInstall(t *testing.T) {
	manager := Manager{
		LocalAppVersion: "1.1.0",
		Layout:          NewLayout(filepath.Join(t.TempDir(), "install"), t.TempDir()),
		Platform:        &fakePlatform{target: "windows-amd64", validateError: ErrUnsupportedInstall},
		ManifestProvider: func(context.Context) (Manifest, ManifestSourceResolution, error) {
			return Manifest{
				SchemaVersion:     SchemaVersion,
				Version:           "1.2.0",
				MinimumAppVersion: "1.0.0",
				Packages: []Package{{
					Target:      "windows-amd64",
					PayloadType: PayloadTypeFull,
					URL:         "file:///tmp/app.zip",
					SHA256:      validSHA256,
				}},
			}, ManifestSourceResolution{Source: "override", URL: "file:///manifest.json"}, nil
		},
	}

	state, err := manager.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if state.Kind != UpdateKindUnsupportedInstall || state.ErrorCode != "APP-UPDATE-INSTALL-UNSUPPORTED" {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestManagerDownloadStagesPayload(t *testing.T) {
	payload := writeZip(t, map[string]string{
		"ant-chrome.exe":                "MZ",
		"publish/runtime-manifest.json": `{"schemaVersion":2}`,
	})
	data, err := os.ReadFile(payload)
	if err != nil {
		t.Fatalf("read payload: %v", err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	layout := NewLayout(filepath.Join(t.TempDir(), "install"), t.TempDir())
	manager := Manager{
		LocalAppVersion: "1.1.0",
		Layout:          layout,
		Platform:        &fakePlatform{target: "windows-amd64"},
		ManifestProvider: func(context.Context) (Manifest, ManifestSourceResolution, error) {
			return Manifest{
				SchemaVersion: SchemaVersion,
				Version:       "1.2.0",
				Packages: []Package{{
					Target:      "windows-amd64",
					PayloadType: PayloadTypeFull,
					URL:         payload,
					SHA256:      sum,
					Size:        int64(len(data)),
				}},
			}, ManifestSourceResolution{Source: "override", URL: "file:///manifest.json"}, nil
		},
	}

	state, err := manager.Download(context.Background())
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	if state.Status != PersistentStatusStaged || state.RemoteAppVersion != "1.2.0" {
		t.Fatalf("unexpected state: %+v", state)
	}
	if _, err := os.Stat(filepath.Join(layout.StagingRoot(), "1.2.0", "ant-chrome.exe")); err != nil {
		t.Fatalf("expected staged exe: %v", err)
	}
	persistent, err := ReadState(layout)
	if err != nil {
		t.Fatalf("ReadState returned error: %v", err)
	}
	if persistent.Status != PersistentStatusStaged || persistent.PayloadURL != payload {
		t.Fatalf("unexpected persistent state: %+v", persistent)
	}
}

func TestManagerApplySpawnsRunnerForStagedUpdate(t *testing.T) {
	stateRoot := t.TempDir()
	installRoot := filepath.Join(t.TempDir(), "install")
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatalf("mkdir install: %v", err)
	}
	layout := NewLayout(installRoot, stateRoot)
	staged := filepath.Join(layout.StagingRoot(), "1.2.0")
	if err := os.MkdirAll(filepath.Join(staged, "publish"), 0o755); err != nil {
		t.Fatalf("mkdir staged: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staged, "ant-chrome.exe"), []byte("MZ"), 0o600); err != nil {
		t.Fatalf("write exe: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staged, "publish", "runtime-manifest.json"), []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write runtime manifest: %v", err)
	}
	platform := &fakePlatform{target: "windows-amd64"}
	manager := Manager{
		LocalAppVersion: "1.1.0",
		Layout:          layout,
		Platform:        platform,
	}
	if err := WriteState(layout, PersistentState{
		Status:           PersistentStatusStaged,
		LocalAppVersion:  "1.1.0",
		RemoteAppVersion: "1.2.0",
		Target:           "windows-amd64",
		ManifestSource:   "override",
		ManifestURL:      "file:///manifest.json",
		PayloadURL:       "file:///payload.zip",
	}); err != nil {
		t.Fatalf("write state: %v", err)
	}

	state, err := manager.Apply(context.Background())
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !platform.spawned || state.Status != PersistentStatusApplying {
		t.Fatalf("expected runner spawn and applying state, got spawned=%v state=%+v", platform.spawned, state)
	}
	if platform.preparedPlan.StagedPath != staged {
		t.Fatalf("unexpected staged path in plan: %q", platform.preparedPlan.StagedPath)
	}
	if platform.preparedPlan.RunnerPath == "" {
		t.Fatal("expected runner path in prepared plan")
	}
	if filepath.Dir(platform.preparedPlan.RunnerPath) == layout.RunnerRoot() {
		t.Fatalf("expected runner path to use an attempt-specific directory, got %q", platform.preparedPlan.RunnerPath)
	}
	if rel, err := filepath.Rel(layout.RunnerRoot(), platform.preparedPlan.RunnerPath); err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) {
		t.Fatalf("runner path should stay under runner root: path=%q rel=%q err=%v", platform.preparedPlan.RunnerPath, rel, err)
	}
	plan, err := ReadPlan(layout.PlanPath())
	if err != nil {
		t.Fatalf("ReadPlan returned error: %v", err)
	}
	if plan.RunnerPath != platform.preparedPlan.RunnerPath {
		t.Fatalf("written plan runner path mismatch: got=%q want=%q", plan.RunnerPath, platform.preparedPlan.RunnerPath)
	}
}
