package appupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"ant-chrome/backend/internal/config"
)

const envManifestURL = "DESKTOP_APP_UPDATE_MANIFEST_URL"

var loadManifestFile = LoadManifest

type ManifestSourceResolution struct {
	URL        string
	Source     string
	ConfigPath string
}

type runtimeManifestConfig struct {
	ManifestURL string `json:"manifestUrl"`
}

func ResolveManifestSource(runtimeDir string, cfg *config.Config) ManifestSourceResolution {
	if strings.TrimSpace(runtimeDir) != "" {
		configPath := filepath.Join(strings.TrimSpace(runtimeDir), "config", "app-update.json")
		if manifestURL := readRuntimeManifestURL(configPath); manifestURL != "" {
			return ManifestSourceResolution{
				URL:        manifestURL,
				Source:     "runtime-config",
				ConfigPath: configPath,
			}
		}
	}

	if manifestURL := strings.TrimSpace(os.Getenv(envManifestURL)); manifestURL != "" {
		return ManifestSourceResolution{
			URL:    manifestURL,
			Source: "env:" + envManifestURL,
		}
	}

	if cfg != nil {
		if manifestURL := strings.TrimSpace(cfg.Release.AppUpdateManifestURL); manifestURL != "" {
			return ManifestSourceResolution{
				URL:    manifestURL,
				Source: "config.yaml",
			}
		}
	}

	return ManifestSourceResolution{}
}

func readRuntimeManifestURL(configPath string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var runtimeConfig runtimeManifestConfig
	if err := json.Unmarshal(data, &runtimeConfig); err != nil {
		return ""
	}
	return strings.TrimSpace(runtimeConfig.ManifestURL)
}

func LoadManifestFromSource(ctx context.Context, source ManifestSourceResolution) (Manifest, error) {
	manifestURL := strings.TrimSpace(source.URL)
	if manifestURL == "" {
		return Manifest{}, fmt.Errorf("app update manifest source is empty")
	}
	if isWindowsDriveAbsolutePath(manifestURL) {
		return loadManifestFile(manifestURL)
	}

	parsed, err := url.Parse(manifestURL)
	if err != nil {
		return Manifest{}, err
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return loadManifestFromHTTP(ctx, manifestURL)
	case "file":
		path, err := fileURLPath(parsed)
		if err != nil {
			return Manifest{}, err
		}
		return loadManifestFile(path)
	case "":
		return loadManifestFile(manifestURL)
	default:
		return Manifest{}, fmt.Errorf("unsupported app update manifest source scheme: %s", parsed.Scheme)
	}
}

func loadManifestFromHTTP(ctx context.Context, manifestURL string) (Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return Manifest{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Manifest{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Manifest{}, fmt.Errorf("app update manifest request failed: HTTP %d", resp.StatusCode)
	}

	var manifest Manifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != SchemaVersion {
		return Manifest{}, fmt.Errorf("unsupported app update manifest schema version: %d", manifest.SchemaVersion)
	}
	return manifest, nil
}

func fileURLPath(parsed *url.URL) (string, error) {
	if parsed.Host == "" && parsed.Path == "" {
		return "", fmt.Errorf("file url path is required")
	}

	path, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return "", err
	}
	if parsed.Host != "" {
		path = "//" + parsed.Host + path
	}
	return path, nil
}

func isWindowsDriveAbsolutePath(path string) bool {
	if len(path) < 3 {
		return false
	}
	drive := path[0]
	if !((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) {
		return false
	}
	return path[1] == ':' && (path[2] == '\\' || path[2] == '/')
}
