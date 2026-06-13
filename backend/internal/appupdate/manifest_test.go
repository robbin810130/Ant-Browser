package appupdate

import (
	"os"
	"path/filepath"
	"testing"
)

const validSHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestLoadManifestAcceptsSchemaOne(t *testing.T) {
	path := writeManifest(t, `{
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
				"sha256": "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF",
				"size": 123
			}
		]
	}`)

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest 返回错误: %v", err)
	}

	if manifest.SchemaVersion != 1 {
		t.Fatalf("schemaVersion 不正确: %d", manifest.SchemaVersion)
	}
	if manifest.Channel != "stable" {
		t.Fatalf("channel 不正确: %s", manifest.Channel)
	}
	if manifest.Version != "1.2.3" {
		t.Fatalf("version 不正确: %s", manifest.Version)
	}
	if manifest.MinimumRuntimeResourceVersion != "2026.01" {
		t.Fatalf("minimumRuntimeResourceVersion 不正确: %s", manifest.MinimumRuntimeResourceVersion)
	}
	if manifest.MinimumAppVersion != "1.0.0" {
		t.Fatalf("minimumAppVersion 不正确: %s", manifest.MinimumAppVersion)
	}
	if manifest.PublishedAt != "2026-05-01T00:00:00Z" {
		t.Fatalf("publishedAt 不正确: %s", manifest.PublishedAt)
	}
	if manifest.Notes != "Ship it" {
		t.Fatalf("notes 不正确: %s", manifest.Notes)
	}
	if len(manifest.Packages) != 1 {
		t.Fatalf("packages 数量不正确: %d", len(manifest.Packages))
	}
}

func TestLoadManifestRejectsUnsupportedSchema(t *testing.T) {
	path := writeManifest(t, `{"schemaVersion": 2}`)

	_, err := LoadManifest(path)
	if err == nil {
		t.Fatalf("LoadManifest 应拒绝不支持的 schema")
	}
}

func TestPackageForTargetSelectsFullPayload(t *testing.T) {
	manifest := Manifest{
		Packages: []Package{
			{
				Target:      "darwin-arm64",
				PayloadType: PayloadTypeFull,
				URL:         "https://example.test/mac.zip",
				SHA256:      validSHA256,
				Size:        10,
			},
			{
				Target:      "windows-x64",
				PayloadType: PayloadTypeFull,
				URL:         " https://example.test/win.zip ",
				SHA256:      " 0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF ",
				Size:        20,
			},
		},
	}

	pkg, err := manifest.PackageForTarget("windows-x64")
	if err != nil {
		t.Fatalf("PackageForTarget 返回错误: %v", err)
	}

	if pkg.Target != "windows-x64" {
		t.Fatalf("target 不正确: %s", pkg.Target)
	}
	if pkg.PayloadType != PayloadTypeFull {
		t.Fatalf("payloadType 不正确: %s", pkg.PayloadType)
	}
	if pkg.URL != "https://example.test/win.zip" {
		t.Fatalf("url 未 trim: %q", pkg.URL)
	}
	if pkg.SHA256 != validSHA256 {
		t.Fatalf("sha256 未规范化: %q", pkg.SHA256)
	}
	if pkg.Size != 20 {
		t.Fatalf("size 不正确: %d", pkg.Size)
	}
}

func TestPackageForTargetAcceptsSupportedSources(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "http", url: "http://example.test/full.zip"},
		{name: "https", url: "https://example.test/full.zip"},
		{name: "file", url: "file:///tmp/full.zip"},
		{name: "absolute path", url: filepath.Join(t.TempDir(), "full.zip")},
		{name: "relative path", url: filepath.Join("updates", "full.zip")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := Manifest{
				Packages: []Package{
					{
						Target:      "windows-x64",
						PayloadType: PayloadTypeFull,
						URL:         tt.url,
						SHA256:      validSHA256,
					},
				},
			}

			if _, err := manifest.PackageForTarget("windows-x64"); err != nil {
				t.Fatalf("PackageForTarget 返回错误: %v", err)
			}
		})
	}
}

func TestPackageForTargetRejectsDeltaInPhaseOne(t *testing.T) {
	manifest := Manifest{
		Packages: []Package{
			{
				Target:      "windows-x64",
				PayloadType: "delta",
				URL:         "https://example.test/delta.zip",
				SHA256:      validSHA256,
			},
		},
	}

	_, err := manifest.PackageForTarget("windows-x64")
	if err == nil {
		t.Fatalf("PackageForTarget 应拒绝 phase one 不支持的 delta payload")
	}
}

func TestPackageForTargetRejectsEmptyTarget(t *testing.T) {
	manifest := Manifest{
		Packages: []Package{
			{
				Target:      "windows-x64",
				PayloadType: PayloadTypeFull,
				URL:         "https://example.test/full.zip",
				SHA256:      validSHA256,
			},
		},
	}

	_, err := manifest.PackageForTarget(" ")
	if err == nil {
		t.Fatalf("PackageForTarget 应拒绝空 target")
	}
}

func TestPackageForTargetRejectsEmptyURL(t *testing.T) {
	manifest := Manifest{
		Packages: []Package{
			{
				Target:      "windows-x64",
				PayloadType: PayloadTypeFull,
				URL:         " ",
				SHA256:      validSHA256,
			},
		},
	}

	_, err := manifest.PackageForTarget("windows-x64")
	if err == nil {
		t.Fatalf("PackageForTarget 应拒绝空 url")
	}
}

func TestPackageForTargetRejectsWhitespaceURL(t *testing.T) {
	manifest := Manifest{
		Packages: []Package{
			{
				Target:      "windows-x64",
				PayloadType: PayloadTypeFull,
				URL:         "\t\n ",
				SHA256:      validSHA256,
			},
		},
	}

	_, err := manifest.PackageForTarget("windows-x64")
	if err == nil {
		t.Fatalf("PackageForTarget 应拒绝纯空白 url")
	}
}

func TestPackageForTargetRejectsEmptySHA256(t *testing.T) {
	manifest := Manifest{
		Packages: []Package{
			{
				Target:      "windows-x64",
				PayloadType: PayloadTypeFull,
				URL:         "https://example.test/full.zip",
				SHA256:      " ",
			},
		},
	}

	_, err := manifest.PackageForTarget("windows-x64")
	if err == nil {
		t.Fatalf("PackageForTarget 应拒绝空 sha256")
	}
}

func TestPackageForTargetRejectsInvalidSHA256(t *testing.T) {
	manifest := Manifest{
		Packages: []Package{
			{
				Target:      "windows-x64",
				PayloadType: PayloadTypeFull,
				URL:         "https://example.test/full.zip",
				SHA256:      "abc",
			},
		},
	}

	_, err := manifest.PackageForTarget("windows-x64")
	if err == nil {
		t.Fatalf("PackageForTarget 应拒绝无效 sha256")
	}
}

func TestClassifyUsesRemoteAndMinimumAppVersions(t *testing.T) {
	tests := []struct {
		name         string
		localVersion string
		manifest     Manifest
		want         UpdateKind
	}{
		{
			name:         "minimum app version requires update",
			localVersion: "1.0.0",
			manifest: Manifest{
				Version:           "1.5.0",
				MinimumAppVersion: "1.1.0",
			},
			want: UpdateKindRequired,
		},
		{
			name:         "remote newer is soft",
			localVersion: "1.2.0",
			manifest: Manifest{
				Version:           "1.5.0",
				MinimumAppVersion: "1.1.0",
			},
			want: UpdateKindSoft,
		},
		{
			name:         "equal remote is none",
			localVersion: "1.5.0",
			manifest: Manifest{
				Version:           "1.5.0",
				MinimumAppVersion: "1.1.0",
			},
			want: UpdateKindNone,
		},
		{
			name:         "newer local is none",
			localVersion: "1.6.0",
			manifest: Manifest{
				Version:           "1.5.0",
				MinimumAppVersion: "1.1.0",
			},
			want: UpdateKindNone,
		},
		{
			name:         "malformed local is none",
			localVersion: "dev",
			manifest: Manifest{
				Version:           "1.5.0",
				MinimumAppVersion: "1.1.0",
			},
			want: UpdateKindNone,
		},
		{
			name:         "semver suffix is ignored",
			localVersion: "1.2.3-beta+build.5",
			manifest: Manifest{
				Version:           "1.2.4+build.6",
				MinimumAppVersion: "1.2.0",
			},
			want: UpdateKindSoft,
		},
		{
			name:         "numeric segments are compared numerically",
			localVersion: "1.10.0",
			manifest: Manifest{
				Version:           "1.2.0",
				MinimumAppVersion: "1.0.0",
			},
			want: UpdateKindNone,
		},
		{
			name:         "missing patch segment equals zero",
			localVersion: "1.2",
			manifest: Manifest{
				Version:           "1.2.0",
				MinimumAppVersion: "1.0.0",
			},
			want: UpdateKindNone,
		},
		{
			name:         "malformed remote is none",
			localVersion: "1.0.0",
			manifest: Manifest{
				Version:           "new",
				MinimumAppVersion: "0.9.0",
			},
			want: UpdateKindNone,
		},
		{
			name:         "malformed minimum is none",
			localVersion: "1.0.0",
			manifest: Manifest{
				Version:           "2.0.0",
				MinimumAppVersion: "old",
			},
			want: UpdateKindNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.localVersion, tt.manifest)
			if got != tt.want {
				t.Fatalf("Classify() = %s, want %s", got, tt.want)
			}
		})
	}
}

func writeManifest(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("写入 manifest 失败: %v", err)
	}
	return path
}
