package appupdate

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	PersistentStatusIdle               PersistentStatus = "idle"
	PersistentStatusAvailable          PersistentStatus = "available"
	PersistentStatusDownloading        PersistentStatus = "downloading"
	PersistentStatusStaged             PersistentStatus = "staged"
	PersistentStatusApplying           PersistentStatus = "applying"
	PersistentStatusVerifying          PersistentStatus = "verifying"
	PersistentStatusSucceeded          PersistentStatus = "succeeded"
	PersistentStatusRolledBack         PersistentStatus = "rolled_back"
	PersistentStatusFailedManualRepair PersistentStatus = "failed_manual_repair"
)

type ErrorInfo struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// PersistentState is the disk checkpoint; State in manifest.go is the API-facing view.
type PersistentState struct {
	Status           PersistentStatus `json:"status"`
	LocalAppVersion  string           `json:"localAppVersion"`
	RemoteAppVersion string           `json:"remoteAppVersion"`
	ManifestSource   string           `json:"manifestSource"`
	ManifestURL      string           `json:"manifestUrl"`
	PayloadURL       string           `json:"payloadUrl"`
	Target           string           `json:"target"`
	PlanPath         string           `json:"planPath"`
	LogPath          string           `json:"logPath"`
	BackupPath       string           `json:"backupPath"`
	LastError        ErrorInfo        `json:"lastError"`
	UpdatedAt        string           `json:"updatedAt"`
}

func WriteState(layout Layout, state PersistentState) error {
	if err := layout.Validate(); err != nil {
		return err
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSONFile(layout.StatePath(), state)
}

func ReadState(layout Layout) (PersistentState, error) {
	data, err := os.ReadFile(layout.StatePath())
	if err != nil {
		return PersistentState{}, err
	}

	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return PersistentState{}, err
	}
	return state, nil
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := atomicWriteFile(path, data, 0o600); err != nil {
		return err
	}
	return nil
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		_ = os.Remove(tempPath)
	}()

	if err := temp.Chmod(perm); err != nil {
		return err
	}
	if n, err := temp.Write(data); err != nil {
		return err
	} else if n != len(data) {
		return io.ErrShortWrite
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		closed = true
		return err
	}
	closed = true

	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	if err := syncDir(dir); err != nil {
		return err
	}
	return nil
}

func syncDir(dir string) error {
	parent, err := os.Open(dir)
	if err != nil {
		if isUnsupportedDirSyncError(err) {
			return nil
		}
		return err
	}
	defer parent.Close()

	if err := parent.Sync(); err != nil {
		if isUnsupportedDirSyncError(err) {
			return nil
		}
		return err
	}
	return nil
}

func isUnsupportedDirSyncError(err error) bool {
	if errors.Is(err, os.ErrInvalid) || errors.Is(err, os.ErrPermission) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not supported") ||
		strings.Contains(message, "operation not supported") ||
		strings.Contains(message, "invalid argument") ||
		strings.Contains(message, "access is denied") ||
		strings.Contains(message, "inappropriate ioctl")
}
