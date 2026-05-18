package appupdate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadPayloadFromHTTPVerifiesHashAndSize(t *testing.T) {
	body := []byte("payload")
	sum := fmt.Sprintf("%x", sha256.Sum256(body))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	dst, err := DownloadPayload(context.Background(), server.URL, t.TempDir(), sum, int64(len(body)))
	if err != nil {
		t.Fatalf("DownloadPayload returned error: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read downloaded payload: %v", err)
	}
	if string(data) != "payload" {
		t.Fatalf("unexpected payload: %q", string(data))
	}
	assertFileMode(t, dst, 0o600)
}

func TestDownloadPayloadFromFileURLVerifiesHash(t *testing.T) {
	body := []byte("payload")
	sum := fmt.Sprintf("%x", sha256.Sum256(body))
	src := filepath.Join(t.TempDir(), "payload.zip")
	if err := os.WriteFile(src, body, 0o600); err != nil {
		t.Fatalf("write source payload: %v", err)
	}

	dst, err := DownloadPayload(context.Background(), "file://"+filepath.ToSlash(src), t.TempDir(), sum, int64(len(body)))
	if err != nil {
		t.Fatalf("DownloadPayload returned error: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read downloaded payload: %v", err)
	}
	if string(data) != "payload" {
		t.Fatalf("unexpected payload: %q", string(data))
	}
}

func TestDownloadPayloadRejectsHashMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.zip")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	downloadsDir := t.TempDir()
	if _, err := DownloadPayload(context.Background(), path, downloadsDir, validSHA256, 0); err == nil {
		t.Fatal("expected hash mismatch")
	}
	if _, err := os.Stat(filepath.Join(downloadsDir, "payload.zip")); !os.IsNotExist(err) {
		t.Fatalf("mismatched payload should be removed, stat err=%v", err)
	}
}

func TestDownloadPayloadRejectsSizeMismatch(t *testing.T) {
	body := []byte("payload")
	sum := fmt.Sprintf("%x", sha256.Sum256(body))
	path := filepath.Join(t.TempDir(), "payload.zip")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	if _, err := DownloadPayload(context.Background(), path, t.TempDir(), sum, int64(len(body)+1)); err == nil {
		t.Fatal("expected size mismatch")
	}
}

func TestDownloadPayloadRejectsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	if _, err := DownloadPayload(context.Background(), server.URL, t.TempDir(), validSHA256, 0); err == nil {
		t.Fatal("expected HTTP failure")
	}
}
