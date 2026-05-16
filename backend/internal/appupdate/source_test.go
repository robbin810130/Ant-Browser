package appupdate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestResolveManifestSourcePrefersRuntimeConfig(t *testing.T) {
	t.Setenv("DESKTOP_APP_UPDATE_MANIFEST_URL", "https://updates.example.com/env.json")

	runtimeDir := t.TempDir()
	configDir := filepath.Join(runtimeDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("创建 runtime config 目录失败: %v", err)
	}
	configPath := filepath.Join(configDir, "app-update.json")
	if err := os.WriteFile(configPath, []byte(`{"manifestUrl":" https://updates.example.com/runtime.json "}`), 0o644); err != nil {
		t.Fatalf("写入 runtime config 失败: %v", err)
	}

	resolution := ResolveManifestSource(runtimeDir, &config.Config{
		Release: config.ReleaseConfig{AppUpdateManifestURL: "https://updates.example.com/config.json"},
	})

	if resolution.URL != "https://updates.example.com/runtime.json" {
		t.Fatalf("URL 不正确: got=%q", resolution.URL)
	}
	if resolution.Source != "runtime-config" {
		t.Fatalf("Source 不正确: got=%q", resolution.Source)
	}
	if resolution.ConfigPath != configPath {
		t.Fatalf("ConfigPath 不正确: got=%q want=%q", resolution.ConfigPath, configPath)
	}
}

func TestResolveManifestSourceUsesEnvBeforeConfig(t *testing.T) {
	t.Setenv("DESKTOP_APP_UPDATE_MANIFEST_URL", " https://updates.example.com/env.json ")

	resolution := ResolveManifestSource(t.TempDir(), &config.Config{
		Release: config.ReleaseConfig{AppUpdateManifestURL: "https://updates.example.com/config.json"},
	})

	if resolution.URL != "https://updates.example.com/env.json" {
		t.Fatalf("URL 不正确: got=%q", resolution.URL)
	}
	if resolution.Source != "env:DESKTOP_APP_UPDATE_MANIFEST_URL" {
		t.Fatalf("Source 不正确: got=%q", resolution.Source)
	}
	if resolution.ConfigPath != "" {
		t.Fatalf("ConfigPath 应为空: got=%q", resolution.ConfigPath)
	}
}

func TestResolveManifestSourceUsesConfig(t *testing.T) {
	resolution := ResolveManifestSource(t.TempDir(), &config.Config{
		Release: config.ReleaseConfig{AppUpdateManifestURL: " https://updates.example.com/config.json "},
	})

	if resolution.URL != "https://updates.example.com/config.json" {
		t.Fatalf("URL 不正确: got=%q", resolution.URL)
	}
	if resolution.Source != "config.yaml" {
		t.Fatalf("Source 不正确: got=%q", resolution.Source)
	}
}

func TestLoadManifestFromSourceSupportsHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validManifestJSON()))
	}))
	defer server.Close()

	manifest, err := LoadManifestFromSource(context.Background(), ManifestSourceResolution{URL: server.URL})
	if err != nil {
		t.Fatalf("LoadManifestFromSource 返回错误: %v", err)
	}

	if manifest.SchemaVersion != SchemaVersion || manifest.Version != "1.2.3" {
		t.Fatalf("manifest 不正确: got=%+v", manifest)
	}
}

func TestLoadManifestFromSourceSupportsFileURL(t *testing.T) {
	path := writeManifest(t, validManifestJSON())

	manifest, err := LoadManifestFromSource(context.Background(), ManifestSourceResolution{URL: "file://" + path})
	if err != nil {
		t.Fatalf("LoadManifestFromSource 返回错误: %v", err)
	}

	if manifest.SchemaVersion != SchemaVersion || manifest.Version != "1.2.3" {
		t.Fatalf("manifest 不正确: got=%+v", manifest)
	}
}

func TestLoadManifestFromSourceTreatsWindowsPathAsLocal(t *testing.T) {
	originalLoadManifestFile := loadManifestFile
	t.Cleanup(func() {
		loadManifestFile = originalLoadManifestFile
	})

	var gotPath string
	loadManifestFile = func(path string) (Manifest, error) {
		gotPath = path
		return Manifest{SchemaVersion: SchemaVersion, Version: "1.2.3"}, nil
	}

	const windowsPath = `C:\updates\app-update.json`
	manifest, err := LoadManifestFromSource(context.Background(), ManifestSourceResolution{URL: windowsPath})
	if err != nil {
		t.Fatalf("LoadManifestFromSource 返回错误: %v", err)
	}

	if manifest.SchemaVersion != SchemaVersion || manifest.Version != "1.2.3" {
		t.Fatalf("manifest 不正确: got=%+v", manifest)
	}
	if gotPath != windowsPath {
		t.Fatalf("Windows 本地路径未路由到本地 loader: got=%q want=%q", gotPath, windowsPath)
	}
}

func validManifestJSON() string {
	return `{
		"schemaVersion": 1,
		"channel": "stable",
		"version": "1.2.3",
		"minimumRuntimeResourceVersion": "2026.01",
		"minimumAppVersion": "1.0.0",
		"publishedAt": "2026-05-01T00:00:00Z",
		"notes": "Ship it",
		"packages": [
			{
				"target": "windows-x64",
				"payloadType": "full",
				"url": "https://example.test/full.zip",
				"sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				"size": 123
			}
		]
	}`
}
