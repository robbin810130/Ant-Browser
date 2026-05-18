package appupdate

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ExtractFullPayload(zipPath, destination string) error {
	if strings.TrimSpace(destination) == "" {
		return fmt.Errorf("payload destination is required")
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if _, err := safeZipEntryPath(destination, file.Name); err != nil {
			return err
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("zip symlink entries are not supported: %s", file.Name)
		}
	}

	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}

	for _, file := range reader.File {
		target, err := safeZipEntryPath(destination, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, dirMode(file.Mode())); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := extractZipFile(file, target); err != nil {
			return err
		}
	}
	return nil
}

func ValidateStagedPayload(target, stagedRoot string) error {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "windows-amd64":
		required := []string{
			"ant-chrome.exe",
			filepath.Join("publish", "runtime-manifest.json"),
		}
		for _, rel := range required {
			info, err := os.Stat(filepath.Join(stagedRoot, rel))
			if err != nil {
				return fmt.Errorf("staged payload missing required file: %s", rel)
			}
			if info.IsDir() {
				return fmt.Errorf("staged payload required path is a directory: %s", rel)
			}
		}
		return nil
	case "darwin-amd64", "darwin-arm64":
		return fmt.Errorf("macOS app update backend is not implemented in Phase 1")
	default:
		return fmt.Errorf("unsupported app update target: %s", target)
	}
}

func safeZipEntryPath(destination, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("zip entry name is empty")
	}

	entryPath := filepath.FromSlash(name)
	if filepath.IsAbs(entryPath) {
		return "", fmt.Errorf("zip entry uses absolute path: %s", name)
	}

	target := filepath.Join(destination, entryPath)
	rel, err := filepath.Rel(destination, target)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("zip entry escapes destination: %s", name)
	}
	return target, nil
}

func extractZipFile(file *zip.File, target string) error {
	in, err := file.Open()
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fileMode(file.Mode()))
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func dirMode(mode os.FileMode) os.FileMode {
	if perm := mode.Perm(); perm != 0 {
		return perm
	}
	return 0o700
}

func fileMode(mode os.FileMode) os.FileMode {
	if perm := mode.Perm(); perm != 0 {
		return perm
	}
	return 0o600
}
