package appupdate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func DownloadPayload(ctx context.Context, source, downloadsDir, expectedSHA256 string, expectedSize int64) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("payload source is required")
	}
	if strings.TrimSpace(downloadsDir) == "" {
		return "", fmt.Errorf("downloads dir is required")
	}
	if err := os.MkdirAll(downloadsDir, 0o700); err != nil {
		return "", err
	}

	dst := filepath.Join(downloadsDir, "payload.zip")
	temp, err := os.CreateTemp(downloadsDir, ".payload.zip.tmp-*")
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		_ = os.Remove(tempPath)
	}()

	if err := temp.Chmod(0o600); err != nil {
		return "", err
	}

	in, err := openPayloadSource(ctx, source)
	if err != nil {
		return "", err
	}
	defer in.Close()

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temp, hash), in)
	if err != nil {
		return "", err
	}
	if expectedSize > 0 && written != expectedSize {
		return "", fmt.Errorf("payload size mismatch: expected %d, got %d", expectedSize, written)
	}

	actualSHA256 := fmt.Sprintf("%x", hash.Sum(nil))
	if expected := strings.ToLower(strings.TrimSpace(expectedSHA256)); expected != "" && actualSHA256 != expected {
		return "", fmt.Errorf("payload sha256 mismatch: expected %s, got %s", expected, actualSHA256)
	}

	if err := temp.Sync(); err != nil {
		return "", err
	}
	if err := temp.Close(); err != nil {
		closed = true
		return "", err
	}
	closed = true

	_ = os.Remove(dst)
	if err := os.Rename(tempPath, dst); err != nil {
		return "", err
	}
	if err := syncDir(downloadsDir); err != nil {
		return "", err
	}
	return dst, nil
}

func openPayloadSource(ctx context.Context, source string) (io.ReadCloser, error) {
	if isWindowsAbsolutePath(source) {
		return os.Open(source)
	}

	parsed, err := url.Parse(source)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("payload download failed: HTTP %d", resp.StatusCode)
		}
		return resp.Body, nil
	case "file":
		path, err := fileURLPath(parsed)
		if err != nil {
			return nil, err
		}
		return os.Open(path)
	case "":
		return os.Open(source)
	default:
		return nil, fmt.Errorf("unsupported payload source scheme: %s", parsed.Scheme)
	}
}
