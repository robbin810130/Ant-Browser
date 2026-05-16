package appupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
