package appupdate

import (
	"errors"
	"testing"
)

func TestNewPlatformBackendMapsWindowsAMD64(t *testing.T) {
	backend, err := NewPlatformBackend("windows", "amd64", PlatformOptions{
		CurrentExePath:    "/tmp/current.exe",
		CurrentAppVersion: "1.2.0",
		ProcessID:         123,
		SuppressRelaunch:  true,
	})
	if err != nil {
		t.Fatalf("NewPlatformBackend returned error: %v", err)
	}
	if backend.Target() != "windows-amd64" {
		t.Fatalf("unexpected target: %s", backend.Target())
	}
	windowsBackend, ok := backend.(WindowsBackend)
	if !ok {
		t.Fatalf("expected WindowsBackend, got %T", backend)
	}
	if windowsBackend.CurrentExePath != "/tmp/current.exe" {
		t.Fatalf("unexpected CurrentExePath: %s", windowsBackend.CurrentExePath)
	}
	if windowsBackend.CurrentAppVersion != "1.2.0" {
		t.Fatalf("unexpected CurrentAppVersion: %s", windowsBackend.CurrentAppVersion)
	}
	if windowsBackend.ProcessID != 123 {
		t.Fatalf("unexpected ProcessID: %d", windowsBackend.ProcessID)
	}
	if !windowsBackend.SuppressRelaunch {
		t.Fatalf("expected SuppressRelaunch to be copied")
	}
}

func TestNewPlatformBackendMapsDarwinARM64(t *testing.T) {
	backend, err := NewPlatformBackend("darwin", "arm64", PlatformOptions{
		CurrentExePath:    "/Applications/Ant.app/Contents/MacOS/Ant",
		CurrentAppVersion: "1.2.0",
		ProcessID:         456,
		SuppressRelaunch:  true,
	})
	if err != nil {
		t.Fatalf("NewPlatformBackend returned error: %v", err)
	}
	if backend.Target() != "darwin-arm64" {
		t.Fatalf("unexpected target: %s", backend.Target())
	}
	darwinBackend, ok := backend.(DarwinBackend)
	if !ok {
		t.Fatalf("expected DarwinBackend, got %T", backend)
	}
	if darwinBackend.CurrentExePath != "/Applications/Ant.app/Contents/MacOS/Ant" {
		t.Fatalf("unexpected CurrentExePath: %s", darwinBackend.CurrentExePath)
	}
	if darwinBackend.CurrentAppVersion != "1.2.0" {
		t.Fatalf("unexpected CurrentAppVersion: %s", darwinBackend.CurrentAppVersion)
	}
	if darwinBackend.ProcessID != 456 {
		t.Fatalf("unexpected ProcessID: %d", darwinBackend.ProcessID)
	}
	if !darwinBackend.SuppressRelaunch {
		t.Fatalf("expected SuppressRelaunch to be copied")
	}
}

func TestNewPlatformBackendMapsDarwinAMD64(t *testing.T) {
	backend, err := NewPlatformBackend("darwin", "amd64", PlatformOptions{})
	if err != nil {
		t.Fatalf("NewPlatformBackend returned error: %v", err)
	}
	if backend.Target() != "darwin-amd64" {
		t.Fatalf("unexpected target: %s", backend.Target())
	}
}

func TestNewPlatformBackendRejectsUnsupportedPlatform(t *testing.T) {
	_, err := NewPlatformBackend("linux", "amd64", PlatformOptions{})
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("expected ErrUnsupportedPlatform, got %v", err)
	}
}
