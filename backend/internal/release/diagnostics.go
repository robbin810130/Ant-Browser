package release

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

type DiagnosticEvent struct {
	EventTime       string            `json:"eventTime"`
	Stage           string            `json:"stage"`
	Result          string            `json:"result"`
	ErrorCode       string            `json:"errorCode,omitempty"`
	AppVersion      string            `json:"appVersion,omitempty"`
	ManifestVersion string            `json:"manifestVersion,omitempty"`
	ResourceVersion string            `json:"resourceVersion,omitempty"`
	Summary         string            `json:"summary,omitempty"`
	Fields          map[string]string `json:"fields,omitempty"`
}

type DiagnosticLogEntry struct {
	Time      string            `json:"time"`
	Level     string            `json:"level"`
	Component string            `json:"component"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
}

type DiagnosticBundle struct {
	ExportedAt       string               `json:"exportedAt"`
	Platform         string               `json:"platform"`
	AppVersion       string               `json:"appVersion,omitempty"`
	ManifestVersion  string               `json:"manifestVersion,omitempty"`
	ResourceVersion  string               `json:"resourceVersion,omitempty"`
	EnvironmentState string               `json:"environmentState,omitempty"`
	ErrorCodes       []string             `json:"errorCodes,omitempty"`
	Summary          string               `json:"summary,omitempty"`
	Paths            map[string]string    `json:"paths,omitempty"`
	Events           []DiagnosticEvent    `json:"events,omitempty"`
	Logs             []DiagnosticLogEntry `json:"logs,omitempty"`
}

func BuildDiagnosticBundle(events []DiagnosticEvent) string {
	bundle := DiagnosticBundle{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Platform:   currentPlatform(),
		Events:     sanitizeEvents(events),
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return `{"events":[]}`
	}
	return string(data)
}

func WriteDiagnosticBundle(root string, bundle DiagnosticBundle) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("diagnostics root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}

	bundle.ExportedAt = time.Now().UTC().Format(time.RFC3339)
	if strings.TrimSpace(bundle.Platform) == "" {
		bundle.Platform = currentPlatform()
	}
	bundle.Events = sanitizeEvents(bundle.Events)
	bundle.Logs = sanitizeLogs(bundle.Logs)

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("diagnostics-%s.json", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(root, filename)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func SanitizeDiagnosticFields(fields map[string]string) map[string]string {
	if len(fields) == 0 {
		return nil
	}

	out := make(map[string]string, len(fields))
	for key, value := range fields {
		if shouldRedactDiagnosticField(key) {
			out[key] = "[REDACTED]"
			continue
		}
		out[key] = strings.TrimSpace(value)
	}
	return out
}

func SanitizeLogFields(fields map[string]interface{}) map[string]string {
	if len(fields) == 0 {
		return nil
	}

	flattened := make(map[string]string, len(fields))
	for key, value := range fields {
		flattened[key] = strings.TrimSpace(fmt.Sprint(value))
	}
	return SanitizeDiagnosticFields(flattened)
}

func sanitizeEvents(events []DiagnosticEvent) []DiagnosticEvent {
	if len(events) == 0 {
		return nil
	}

	out := make([]DiagnosticEvent, 0, len(events))
	for _, event := range events {
		event.Fields = SanitizeDiagnosticFields(event.Fields)
		out = append(out, event)
	}
	return out
}

func sanitizeLogs(logs []DiagnosticLogEntry) []DiagnosticLogEntry {
	if len(logs) == 0 {
		return nil
	}

	out := make([]DiagnosticLogEntry, 0, len(logs))
	for _, entry := range logs {
		entry.Fields = SanitizeDiagnosticFields(entry.Fields)
		out = append(out, entry)
	}
	return out
}

func shouldRedactDiagnosticField(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "accesstoken", "token", "password", "proxypassword", "cookie", "session", "sessiontoken", "authorization":
		return true
	default:
		return strings.Contains(normalized, "password") || strings.Contains(normalized, "token") || strings.Contains(normalized, "cookie")
	}
}

func currentPlatform() string {
	return fmt.Sprintf("%s-%s", goruntime.GOOS, goruntime.GOARCH)
}
