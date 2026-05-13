package release

import (
	"strings"
	"testing"
)

func TestDiagnosticBundleRedactsSecrets(t *testing.T) {
	events := []DiagnosticEvent{{
		Stage:  "update",
		Result: "failure",
		Fields: map[string]string{
			"accessToken":   "secret-token",
			"proxyPassword": "top-secret",
			"summary":       "hash mismatch",
		},
	}}

	bundle := BuildDiagnosticBundle(events)
	if strings.Contains(bundle, "secret-token") || strings.Contains(bundle, "top-secret") {
		t.Fatal("expected diagnostic bundle to redact sensitive values")
	}
	if !strings.Contains(bundle, "[REDACTED]") {
		t.Fatal("expected diagnostic bundle to contain redacted marker")
	}
}
