package authsession

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreSaveLoadAndClearToken(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	if _, err := store.Load(); err != nil {
		t.Fatalf("load empty session failed: %v", err)
	}

	input := Session{
		AccessToken: "token-123",
		RememberMe:  true,
	}
	if err := store.Save(input); err != nil {
		t.Fatalf("save session failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load session failed: %v", err)
	}
	if loaded.AccessToken != "token-123" || !loaded.RememberMe {
		t.Fatalf("unexpected session: %#v", loaded)
	}

	if err := store.Save(Session{}); err != nil {
		t.Fatalf("save empty session failed: %v", err)
	}

	cleared, err := store.Load()
	if err != nil {
		t.Fatalf("load cleared session failed: %v", err)
	}
	if cleared.AccessToken != "" || cleared.RememberMe {
		t.Fatalf("expected empty cleared session, got %#v", cleared)
	}
}

func TestStoreSkipsDiskWriteWhenRememberMeDisabled(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	if err := store.Save(Session{
		AccessToken: "memory-only-token",
		RememberMe:  false,
	}); err != nil {
		t.Fatalf("save memory-only session failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load memory-only session failed: %v", err)
	}
	if loaded.AccessToken != "" || loaded.RememberMe {
		t.Fatalf("expected no persisted session, got %#v", loaded)
	}
}

func TestStoreSaveEmptyTokenClearsPersistedState(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	if err := store.Save(Session{
		AccessToken: "token-123",
		RememberMe:  true,
	}); err != nil {
		t.Fatalf("save initial session failed: %v", err)
	}

	if err := store.Save(Session{
		AccessToken: "   ",
		RememberMe:  true,
	}); err != nil {
		t.Fatalf("save empty-token session failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load cleared session failed: %v", err)
	}
	if loaded.AccessToken != "" || loaded.RememberMe {
		t.Fatalf("expected empty session after clearing token, got %#v", loaded)
	}
}

func TestStoreLoadIgnoresPersistedSessionWhenRememberMeFalse(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	if err := os.MkdirAll(filepath.Dir(store.path), 0o755); err != nil {
		t.Fatalf("create config dir failed: %v", err)
	}

	payload := []byte("{\n  \"accessToken\": \"token-123\",\n  \"rememberMe\": false\n}\n")
	if err := os.WriteFile(store.path, payload, 0o600); err != nil {
		t.Fatalf("write stale session failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load stale session failed: %v", err)
	}
	if loaded.AccessToken != "" || loaded.RememberMe {
		t.Fatalf("expected stale remember=false session to be ignored, got %#v", loaded)
	}
	if _, err := os.Stat(store.path); !os.IsNotExist(err) {
		t.Fatalf("expected stale session file to be removed, stat err=%v", err)
	}
}

func TestStoreLoadIgnoresPersistedBlankTokenWhenRemembered(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	if err := os.MkdirAll(filepath.Dir(store.path), 0o755); err != nil {
		t.Fatalf("create config dir failed: %v", err)
	}

	payload := []byte("{\n  \"accessToken\": \"   \",\n  \"rememberMe\": true\n}\n")
	if err := os.WriteFile(store.path, payload, 0o600); err != nil {
		t.Fatalf("write remembered session failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load remembered session failed: %v", err)
	}
	if loaded.AccessToken != "" || loaded.RememberMe {
		t.Fatalf("expected invalid remembered session to be ignored, got %#v", loaded)
	}
	if _, err := os.Stat(store.path); !os.IsNotExist(err) {
		t.Fatalf("expected invalid remembered file to be removed, stat err=%v", err)
	}
}

func TestStoreWithEmptyRootIsNoOp(t *testing.T) {
	cwd := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	store := NewStore("   ")
	if store.path != "" {
		t.Fatalf("expected empty path for blank root, got %q", store.path)
	}

	if err := store.Save(Session{
		AccessToken: "token-123",
		RememberMe:  true,
	}); err != nil {
		t.Fatalf("save no-op session failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load no-op session failed: %v", err)
	}
	if loaded.AccessToken != "" || loaded.RememberMe {
		t.Fatalf("expected empty session from no-op store, got %#v", loaded)
	}

	if _, err := os.Stat(filepath.Join(cwd, "config")); !os.IsNotExist(err) {
		t.Fatalf("expected no config directory to be created, stat err=%v", err)
	}
}
