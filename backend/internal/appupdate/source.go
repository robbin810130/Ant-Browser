package appupdate

import (
	"bytes"
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

const (
	envManifestURL               = "DESKTOP_APP_UPDATE_MANIFEST_URL"
	envUpdatesDisabled           = "DESKTOP_APP_UPDATE_DISABLED"
	envAllowLocalhostManifestURL = "DESKTOP_APP_UPDATE_ALLOW_LOCAL_MANIFEST_URL"
)

type ManifestSourceResolution struct {
	URL        string
	Source     string
	ConfigPath string
}

type runtimeManifestConfig struct {
	ManifestURL string `json:"manifestUrl"`
}

type manifestSourceKind string

const (
	manifestSourceHTTP  manifestSourceKind = "http"
	manifestSourceFile  manifestSourceKind = "file"
	manifestSourceLocal manifestSourceKind = "local"
)

func ResolveManifestSource(runtimeDir string, cfg *config.Config) ManifestSourceResolution {
	if truthyEnv(envUpdatesDisabled) {
		return ManifestSourceResolution{}
	}

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
		if !isLoopbackHTTPManifestURL(manifestURL) || truthyEnv(envAllowLocalhostManifestURL) {
			return ManifestSourceResolution{
				URL:    manifestURL,
				Source: "env:" + envManifestURL,
			}
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

func truthyEnv(name string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func isLoopbackHTTPManifestURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func readRuntimeManifestURL(configPath string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	var runtimeConfig runtimeManifestConfig
	if err := json.Unmarshal(data, &runtimeConfig); err != nil {
		return ""
	}
	return strings.TrimSpace(runtimeConfig.ManifestURL)
}

func LoadManifestFromSource(ctx context.Context, source ManifestSourceResolution) (Manifest, error) {
	kind, location, err := resolveManifestSourceLocation(source.URL)
	if err != nil {
		return Manifest{}, err
	}

	switch kind {
	case manifestSourceHTTP:
		return loadManifestFromHTTP(ctx, location)
	case manifestSourceFile, manifestSourceLocal:
		return LoadManifest(location)
	default:
		return Manifest{}, fmt.Errorf("unsupported app update manifest source kind: %s", kind)
	}
}

func resolveManifestSourceLocation(source string) (manifestSourceKind, string, error) {
	manifestURL := strings.TrimSpace(source)
	if manifestURL == "" {
		return "", "", fmt.Errorf("app update manifest source is empty")
	}
	if isWindowsAbsolutePath(manifestURL) {
		return manifestSourceLocal, manifestURL, nil
	}

	parsed, err := url.Parse(manifestURL)
	if err != nil {
		return "", "", err
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return manifestSourceHTTP, manifestURL, nil
	case "file":
		path, err := fileURLPath(parsed)
		if err != nil {
			return "", "", err
		}
		return manifestSourceFile, path, nil
	case "":
		return manifestSourceLocal, manifestURL, nil
	default:
		return "", "", fmt.Errorf("unsupported app update manifest source scheme: %s", parsed.Scheme)
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
