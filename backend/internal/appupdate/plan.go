package appupdate

import (
	"encoding/json"
	"os"
)

type ApplyPlan struct {
	InstallRoot      string `json:"installRoot"`
	StateRoot        string `json:"stateRoot"`
	Target           string `json:"target"`
	OldAppVersion    string `json:"oldAppVersion"`
	NewAppVersion    string `json:"newAppVersion"`
	StagedPath       string `json:"stagedPath"`
	BackupPath       string `json:"backupPath"`
	CurrentExePath   string `json:"currentExePath"`
	ExpectedSHA256   string `json:"expectedSHA256"`
	ManifestSource   string `json:"manifestSource"`
	ManifestURL      string `json:"manifestUrl"`
	PayloadURL       string `json:"payloadUrl"`
	WaitForProcessID int    `json:"waitForProcessId"`
}

func WritePlan(layout Layout, plan ApplyPlan) (string, error) {
	path := layout.PlanPath()
	if err := writeJSONFile(path, plan); err != nil {
		return "", err
	}
	return path, nil
}

func ReadPlan(path string) (ApplyPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ApplyPlan{}, err
	}

	var plan ApplyPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return ApplyPlan{}, err
	}
	return plan, nil
}
