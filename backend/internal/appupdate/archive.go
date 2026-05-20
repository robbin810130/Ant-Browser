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
	requiredDirs, err := requiredZipDirs(destination, reader.File)
	if err != nil {
		return err
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
			if err := mkdirAllSafe(destination, target, dirMode(file.Mode())); err != nil {
				return err
			}
			continue
		}
		if requiredDirs[filepath.Clean(target)] {
			continue
		}
		if err := mkdirAllSafe(destination, filepath.Dir(target), 0o700); err != nil {
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
		return rejectMutableUserData(stagedRoot)
	case "darwin-amd64", "darwin-arm64":
		return validateDarwinStagedPayload(stagedRoot)
	default:
		return fmt.Errorf("unsupported app update target: %s", target)
	}
}

func validateDarwinStagedPayload(stagedRoot string) error {
	appRoot := filepath.Join(stagedRoot, "Ant Browser.app")
	info, err := os.Lstat(appRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("staged payload missing app bundle: Ant Browser.app")
		}
		return fmt.Errorf("staged payload app bundle is not readable: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("staged payload app bundle must not be a symlink: Ant Browser.app")
	}
	if !info.IsDir() {
		return fmt.Errorf("staged payload app bundle is not a directory: Ant Browser.app")
	}

	macos := filepath.Join(appRoot, "Contents", "MacOS")
	required := []string{
		filepath.Join("Ant Browser.app", "Contents", "Info.plist"),
		filepath.Join("Ant Browser.app", "Contents", "MacOS", "publish", "runtime-manifest.json"),
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

	for _, rel := range []string{
		filepath.Join(macos, "ant-chrome"),
		filepath.Join(macos, "bin", "xray"),
		filepath.Join(macos, "bin", "sing-box"),
	} {
		if err := requireExecutable(rel); err != nil {
			return err
		}
	}
	return rejectMutableUserData(stagedRoot)
}

func requireExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("staged payload missing executable: %s", path)
	}
	if info.IsDir() {
		return fmt.Errorf("staged payload executable path is a directory: %s", path)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("staged payload executable bit is not set: %s", path)
	}
	return nil
}

func rejectMutableUserData(stagedRoot string) error {
	return filepath.WalkDir(stagedRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(stagedRoot, path)
		if err != nil {
			return err
		}
		rel = strings.ToLower(filepath.ToSlash(filepath.Clean(rel)))
		if rel == "." {
			return nil
		}
		if rel == "data" || strings.HasPrefix(rel, "data/") {
			return fmt.Errorf("staged payload contains mutable user data: %s", path)
		}
		if hasPathSegment(rel, "user data") {
			return fmt.Errorf("staged payload contains mutable user data: %s", path)
		}
		darwinData := "ant browser.app/contents/macos/data"
		if rel == darwinData || strings.HasPrefix(rel, darwinData+"/") {
			return fmt.Errorf("staged payload contains mutable user data: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".db", ".sqlite", ".sqlite3":
			return fmt.Errorf("staged payload contains mutable user data: %s", path)
		default:
			return nil
		}
	})
}

func hasPathSegment(path, segment string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == segment {
			return true
		}
	}
	return false
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

func requiredZipDirs(destination string, files []*zip.File) (map[string]bool, error) {
	dirs := map[string]bool{
		filepath.Clean(destination): true,
	}
	for _, file := range files {
		target, err := safeZipEntryPath(destination, file.Name)
		if err != nil {
			return nil, err
		}
		if file.FileInfo().IsDir() {
			dirs[filepath.Clean(target)] = true
			continue
		}
		dir := filepath.Clean(filepath.Dir(target))
		for {
			dirs[dir] = true
			parent := filepath.Dir(dir)
			if parent == dir || dir == filepath.Clean(destination) {
				break
			}
			dir = parent
		}
	}
	return dirs, nil
}

func mkdirAllSafe(root, path string, mode os.FileMode) error {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if path == root {
		return os.MkdirAll(root, mode)
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("directory escapes destination: %s", path)
	}

	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err == nil {
			if info.IsDir() {
				continue
			}
			if err := os.RemoveAll(current); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Mkdir(current, mode); err != nil && !os.IsExist(err) {
			return err
		}
	}
	return nil
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
