package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWorkspaceBaseURLFallsBackToDefaultWhenConfigMissing(t *testing.T) {
	appRoot := t.TempDir()

	got := resolveWorkspaceBaseURL(appRoot)

	if got != "http://127.0.0.1:4174" {
		t.Fatalf("unexpected fallback base url: %s", got)
	}
}

func TestResolveWorkspaceBaseURLReadsServerConnectionConfig(t *testing.T) {
	appRoot := t.TempDir()
	configDir := filepath.Join(appRoot, "runtime", "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "server-connection.json")
	content := `{"serverProtocol":"https","serverIp":"10.20.30.40","serverPort":9443}`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got := resolveWorkspaceBaseURL(appRoot)

	if got != "https://10.20.30.40:9443" {
		t.Fatalf("unexpected configured base url: %s", got)
	}
}
