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

func TestResolveManifestSourceAcceptsRuntimeConfigWithBOM(t *testing.T) {
	runtimeDir := t.TempDir()
	configDir := filepath.Join(runtimeDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("创建 runtime config 目录失败: %v", err)
	}
	configPath := filepath.Join(configDir, "app-update.json")
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"manifestUrl":"http://127.0.0.1:8080/app-update-stable.json"}`)...)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("写入 runtime config 失败: %v", err)
	}

	resolution := ResolveManifestSource(runtimeDir, &config.Config{})

	if resolution.URL != "http://127.0.0.1:8080/app-update-stable.json" {
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
	const windowsPath = `C:\updates\app-update.json`
	kind, path, err := resolveManifestSourceLocation(windowsPath)
	if err != nil {
		t.Fatalf("resolveManifestSourceLocation 返回错误: %v", err)
	}
	if kind != manifestSourceLocal {
		t.Fatalf("Windows 本地路径应被识别为 local: got=%q", kind)
	}
	if path != windowsPath {
		t.Fatalf("Windows 本地路径不应被改写: got=%q want=%q", path, windowsPath)
	}
}

func TestResolveManifestSourceLocationClassifiesSupportedSources(t *testing.T) {
	localPath := filepath.Join("updates", "app-update.json")
	filePath := filepath.Join(t.TempDir(), "app-update.json")

	tests := []struct {
		name     string
		source   string
		wantKind manifestSourceKind
		wantPath string
	}{
		{
			name:     "http",
			source:   "https://updates.example.com/app-update.json",
			wantKind: manifestSourceHTTP,
			wantPath: "https://updates.example.com/app-update.json",
		},
		{
			name:     "file url",
			source:   "file://" + filePath,
			wantKind: manifestSourceFile,
			wantPath: filePath,
		},
		{
			name:     "local path",
			source:   localPath,
			wantKind: manifestSourceLocal,
			wantPath: localPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, path, err := resolveManifestSourceLocation(tt.source)
			if err != nil {
				t.Fatalf("resolveManifestSourceLocation 返回错误: %v", err)
			}
			if kind != tt.wantKind {
				t.Fatalf("kind 不正确: got=%q want=%q", kind, tt.wantKind)
			}
			if path != tt.wantPath {
				t.Fatalf("path 不正确: got=%q want=%q", path, tt.wantPath)
			}
		})
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
