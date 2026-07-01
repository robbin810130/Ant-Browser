package appupdate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestResolveManifestSourceCanBeDisabledByEnv(t *testing.T) {
	t.Setenv("DESKTOP_APP_UPDATE_DISABLED", "1")
	t.Setenv("DESKTOP_APP_UPDATE_MANIFEST_URL", "https://updates.example.com/env.json")

	runtimeDir := t.TempDir()
	configDir := filepath.Join(runtimeDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("创建 runtime config 目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "app-update.json"), []byte(`{"manifestUrl":"https://updates.example.com/runtime.json"}`), 0o644); err != nil {
		t.Fatalf("写入 runtime config 失败: %v", err)
	}

	resolution := ResolveManifestSource(runtimeDir, &config.Config{
		Release: config.ReleaseConfig{AppUpdateManifestURL: "https://updates.example.com/config.json"},
	})

	if resolution.URL != "" || resolution.Source != "" || resolution.ConfigPath != "" {
		t.Fatalf("禁用更新后不应解析 manifest source: got=%+v", resolution)
	}
}

func TestResolveManifestSourceIgnoresLoopbackEnvByDefault(t *testing.T) {
	t.Setenv("DESKTOP_APP_UPDATE_MANIFEST_URL", "http://127.0.0.1:8080/app-update-stable.json")

	resolution := ResolveManifestSource(t.TempDir(), &config.Config{
		Release: config.ReleaseConfig{AppUpdateManifestURL: "https://updates.example.com/config.json"},
	})

	if resolution.URL != "https://updates.example.com/config.json" {
		t.Fatalf("loopback env source should be ignored in favor of config: got=%q", resolution.URL)
	}
	if resolution.Source != "config.yaml" {
		t.Fatalf("Source 不正确: got=%q", resolution.Source)
	}
}

func TestResolveManifestSourceAllowsLoopbackEnvWhenExplicitlyEnabled(t *testing.T) {
	t.Setenv("DESKTOP_APP_UPDATE_MANIFEST_URL", "http://localhost:8080/app-update-stable.json")
	t.Setenv("DESKTOP_APP_UPDATE_ALLOW_LOCAL_MANIFEST_URL", "true")

	resolution := ResolveManifestSource(t.TempDir(), &config.Config{
		Release: config.ReleaseConfig{AppUpdateManifestURL: "https://updates.example.com/config.json"},
	})

	if resolution.URL != "http://localhost:8080/app-update-stable.json" {
		t.Fatalf("URL 不正确: got=%q", resolution.URL)
	}
	if resolution.Source != "env:DESKTOP_APP_UPDATE_MANIFEST_URL" {
		t.Fatalf("Source 不正确: got=%q", resolution.Source)
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

func TestResolveManifestSourceUsesDefaultStableManifest(t *testing.T) {
	t.Setenv("DESKTOP_APP_UPDATE_MANIFEST_URL", "")
	t.Setenv("DESKTOP_APP_UPDATE_DISABLED", "")

	resolution := resolveManifestSourceForGOOS(t.TempDir(), &config.Config{}, "windows")

	if resolution.URL != DefaultStableManifestURL {
		t.Fatalf("default stable URL 不正确: got=%q want=%q", resolution.URL, DefaultStableManifestURL)
	}
	if resolution.Source != "default-stable" {
		t.Fatalf("Source 不正确: got=%q", resolution.Source)
	}
	if resolution.ConfigPath != "" {
		t.Fatalf("ConfigPath 应为空: got=%q", resolution.ConfigPath)
	}
}

func TestResolveManifestSourceDoesNotUseDefaultStableManifestOnDarwin(t *testing.T) {
	t.Setenv("DESKTOP_APP_UPDATE_MANIFEST_URL", "")
	t.Setenv("DESKTOP_APP_UPDATE_DISABLED", "")

	resolution := resolveManifestSourceForGOOS(t.TempDir(), &config.Config{}, "darwin")

	if resolution != (ManifestSourceResolution{}) {
		t.Fatalf("darwin 不应使用 Windows 默认 stable manifest: got=%+v", resolution)
	}
}

func TestManifestHTTPClientHasTimeout(t *testing.T) {
	if timeout := newManifestHTTPClient().Timeout; timeout != 15*time.Second {
		t.Fatalf("manifest HTTP client timeout 不正确: got=%s want=%s", timeout, 15*time.Second)
	}
}

func TestLoadManifestFromSourceRejectsCrossOriginRedirect(t *testing.T) {
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validManifestJSON()))
	}))
	defer targetServer.Close()

	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetServer.URL, http.StatusFound)
	}))
	defer sourceServer.Close()

	_, err := LoadManifestFromSource(context.Background(), ManifestSourceResolution{URL: sourceServer.URL})
	if err == nil || !strings.Contains(err.Error(), "cross-origin redirect") {
		t.Fatalf("跨源 redirect 应被拒绝: got=%v", err)
	}
}

func TestLoadManifestFromSourceAllowsThreeSameOriginRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/0":
			http.Redirect(w, r, "/1", http.StatusFound)
		case "/1":
			http.Redirect(w, r, "/2", http.StatusFound)
		case "/2":
			http.Redirect(w, r, "/3", http.StatusFound)
		case "/3":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(validManifestJSON()))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manifest, err := LoadManifestFromSource(context.Background(), ManifestSourceResolution{URL: server.URL + "/0"})
	if err != nil {
		t.Fatalf("三次同源 redirect 应成功: %v", err)
	}
	if manifest.Version != "1.2.3" {
		t.Fatalf("manifest 版本不正确: got=%q", manifest.Version)
	}
}

func TestLoadManifestFromSourceRejectsFourthSameOriginRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/0":
			http.Redirect(w, r, "/1", http.StatusFound)
		case "/1":
			http.Redirect(w, r, "/2", http.StatusFound)
		case "/2":
			http.Redirect(w, r, "/3", http.StatusFound)
		case "/3":
			http.Redirect(w, r, "/4", http.StatusFound)
		case "/4":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(validManifestJSON()))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	_, err := LoadManifestFromSource(context.Background(), ManifestSourceResolution{URL: server.URL + "/0"})
	if err == nil || !strings.Contains(err.Error(), "too many app update manifest redirects") {
		t.Fatalf("第四次同源 redirect 应被拒绝: got=%v", err)
	}
}

func TestLoadManifestFromSourceRejectsOversizedHTTPManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.Repeat("x", maxManifestBytes+1)))
	}))
	defer server.Close()

	_, err := LoadManifestFromSource(context.Background(), ManifestSourceResolution{URL: server.URL})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("超大 manifest 应被拒绝: got=%v", err)
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
