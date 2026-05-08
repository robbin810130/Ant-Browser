package main

import "testing"

func TestIsWailsDevExecutableDir(t *testing.T) {
	t.Run("detects temp executable", func(t *testing.T) {
		if !isWailsDevExecutableDir("/var/folders/ab/tmp/wails-build", "/var/folders/ab/tmp") {
			t.Fatalf("expected temp dir to be treated as wails dev executable")
		}
	})

	t.Run("detects build bin root", func(t *testing.T) {
		if !isWailsDevExecutableDir("/Users/robbin/project/build/bin", "/var/folders/tmp") {
			t.Fatalf("expected build/bin root to be treated as wails dev executable")
		}
	})

	t.Run("detects mac app under build bin", func(t *testing.T) {
		exeDir := "/Users/robbin/project/build/bin/Ant Browser.app/Contents/MacOS"
		if !isWailsDevExecutableDir(exeDir, "/var/folders/tmp") {
			t.Fatalf("expected mac .app under build/bin to be treated as wails dev executable")
		}
	})

	t.Run("ignores packaged app outside build bin", func(t *testing.T) {
		exeDir := "/Applications/Ant Browser.app/Contents/MacOS"
		if isWailsDevExecutableDir(exeDir, "/var/folders/tmp") {
			t.Fatalf("expected packaged app outside build/bin to be treated as production")
		}
	})
}
