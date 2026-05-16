package appupdate

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

const (
	SchemaVersion = 1

	PayloadTypeFull = "full"
)

type UpdateKind string

const (
	UpdateKindNone               UpdateKind = "none"
	UpdateKindSoft               UpdateKind = "soft"
	UpdateKindRequired           UpdateKind = "required"
	UpdateKindUnsupportedInstall UpdateKind = "unsupported_install"
	UpdateKindFailed             UpdateKind = "failed"
)

// PersistentStatus is extended by a later task. Keep this minimal for phase 1.
type PersistentStatus string

type Manifest struct {
	SchemaVersion                 int       `json:"schemaVersion"`
	Channel                       string    `json:"channel"`
	Version                       string    `json:"version"`
	MinimumRuntimeResourceVersion string    `json:"minimumRuntimeResourceVersion"`
	MinimumAppVersion             string    `json:"minimumAppVersion"`
	PublishedAt                   string    `json:"publishedAt"`
	Notes                         string    `json:"notes"`
	Packages                      []Package `json:"packages"`
}

type Package struct {
	Target      string `json:"target"`
	PayloadType string `json:"payloadType"`
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
}

type State struct {
	Kind                          UpdateKind        `json:"kind"`
	Status                        PersistentStatus  `json:"status"`
	LocalAppVersion               string            `json:"localAppVersion"`
	RemoteAppVersion              string            `json:"remoteAppVersion"`
	MinimumRuntimeResourceVersion string            `json:"minimumRuntimeResourceVersion"`
	ManifestSource                string            `json:"manifestSource"`
	ManifestURL                   string            `json:"manifestUrl"`
	PayloadURL                    string            `json:"payloadUrl"`
	Target                        string            `json:"target"`
	Notes                         string            `json:"notes"`
	ErrorCode                     string            `json:"errorCode"`
	ErrorMessage                  string            `json:"errorMessage"`
	Details                       map[string]string `json:"details"`
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != SchemaVersion {
		return Manifest{}, fmt.Errorf("unsupported app update manifest schema version: %d", manifest.SchemaVersion)
	}
	return manifest, nil
}

func (m Manifest) PackageForTarget(target string) (Package, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return Package{}, fmt.Errorf("target is required")
	}

	for _, pkg := range m.Packages {
		if strings.TrimSpace(pkg.Target) != target {
			continue
		}
		if strings.TrimSpace(pkg.PayloadType) != PayloadTypeFull {
			return Package{}, fmt.Errorf("unsupported app update payload type for target %s: %s", target, pkg.PayloadType)
		}
		pkg.Target = strings.TrimSpace(pkg.Target)
		pkg.PayloadType = strings.TrimSpace(pkg.PayloadType)
		pkg.URL = strings.TrimSpace(pkg.URL)
		pkg.SHA256 = strings.ToLower(strings.TrimSpace(pkg.SHA256))
		if pkg.URL == "" {
			return Package{}, fmt.Errorf("app update package url is required for target %s", target)
		}
		if pkg.SHA256 == "" {
			return Package{}, fmt.Errorf("app update package sha256 is required for target %s", target)
		}
		if !isValidSHA256(pkg.SHA256) {
			return Package{}, fmt.Errorf("app update package sha256 must be 64 hex characters for target %s", target)
		}
		if err := validatePackageSource(pkg.URL); err != nil {
			return Package{}, fmt.Errorf("app update package url is invalid for target %s: %w", target, err)
		}
		return pkg, nil
	}
	return Package{}, fmt.Errorf("no app update package for target: %s", target)
}

func isValidSHA256(sum string) bool {
	if len(sum) != 64 {
		return false
	}
	for _, r := range sum {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}

func validatePackageSource(source string) error {
	if source == "" {
		return fmt.Errorf("source is required")
	}
	for _, r := range source {
		if unicode.IsControl(r) {
			return fmt.Errorf("source contains control characters")
		}
	}
	if isWindowsAbsolutePath(source) || filepath.IsAbs(source) {
		return nil
	}

	parsed, err := url.Parse(source)
	if err != nil {
		return err
	}
	if parsed.Scheme == "" {
		return nil
	}

	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "http", "https":
		if parsed.Host == "" {
			return fmt.Errorf("%s url host is required", scheme)
		}
		if containsSpace(source) {
			return fmt.Errorf("%s url contains whitespace", scheme)
		}
		return nil
	case "file":
		if parsed.Host == "" && parsed.Path == "" {
			return fmt.Errorf("file url path is required")
		}
		if containsSpace(source) {
			return fmt.Errorf("file url contains whitespace")
		}
		return nil
	default:
		return fmt.Errorf("unsupported source scheme %s", parsed.Scheme)
	}
}

func containsSpace(value string) bool {
	return strings.IndexFunc(value, unicode.IsSpace) >= 0
}

func isWindowsAbsolutePath(path string) bool {
	if len(path) >= 3 && isASCIIAlpha(path[0]) && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	return strings.HasPrefix(path, `\\`)
}

func isASCIIAlpha(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func Classify(localVersion string, manifest Manifest) UpdateKind {
	local, ok := parseVersion(localVersion)
	if !ok {
		return UpdateKindNone
	}

	remote, ok := parseVersion(manifest.Version)
	if !ok {
		return UpdateKindNone
	}

	if strings.TrimSpace(manifest.MinimumAppVersion) != "" {
		minimum, ok := parseVersion(manifest.MinimumAppVersion)
		if !ok {
			return UpdateKindNone
		}
		if compareVersions(local, minimum) < 0 {
			return UpdateKindRequired
		}
	}

	if compareVersions(local, remote) < 0 {
		return UpdateKindSoft
	}
	return UpdateKindNone
}

func parseVersion(version string) ([]int, bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, false
	}
	if idx := strings.IndexAny(version, "-+"); idx >= 0 {
		version = version[:idx]
	}
	if version == "" {
		return nil, false
	}

	parts := strings.Split(version, ".")
	parsed := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil, false
		}
		parsed = append(parsed, n)
	}
	return parsed, true
}

func compareVersions(a, b []int) int {
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	for i := 0; i < maxLen; i++ {
		av := 0
		if i < len(a) {
			av = a[i]
		}
		bv := 0
		if i < len(b) {
			bv = b[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}
