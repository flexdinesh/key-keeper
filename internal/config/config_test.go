package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadInterpolatesEnvVar(t *testing.T) {
	// actual value: super-secret
	t.Setenv("OTEL_INGEST_API_KEY_BASE64", "c3VwZXItc2VjcmV0")

	path := writeTempConfig(t, `
server:
  listen_addr: ":9999"
  access_log:
    format: logfmt
rules:
  - name: ingest
    host:
      type: exact
      value: ingest-grpc.example.com
    paths:
      - type: prefix
        value: /v1/traces
    validators:
      - type: header_exact
        header: x-api-key
        # actual value from OTEL_INGEST_API_KEY_BASE64: super-secret
        value: ${OTEL_INGEST_API_KEY_BASE64}
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got, want := cfg.Server.ListenAddr, ":9999"; got != want {
		t.Fatalf("ListenAddr = %q, want %q", got, want)
	}
	if got, want := cfg.Server.AccessLog.Format, "logfmt"; got != want {
		t.Fatalf("AccessLog.Format = %q, want %q", got, want)
	}
	if got, want := cfg.Rules[0].Validators[0].Value, "super-secret"; got != want {
		t.Fatalf("validator value = %q, want %q", got, want)
	}
}

func TestLoadDecodesLiteralBase64Value(t *testing.T) {
	path := writeTempConfig(t, `
rules:
  - name: ingest
    host:
      type: exact
      value: ingest-grpc.example.com
    paths:
      - type: prefix
        value: /v1/traces
    validators:
      - type: header_exact
        header: x-api-key
        # actual value: hardcoded-secret
        value: aGFyZGNvZGVkLXNlY3JldA==
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got, want := cfg.Rules[0].Validators[0].Value, "hardcoded-secret"; got != want {
		t.Fatalf("validator value = %q, want %q", got, want)
	}
}

func TestLoadAcceptsUnpaddedStandardBase64(t *testing.T) {
	path := writeTempConfig(t, `
rules:
  - name: ingest
    host:
      type: exact
      value: ingest-grpc.example.com
    paths:
      - type: prefix
        value: /v1/traces
    validators:
      - type: header_exact
        header: x-api-key
        # actual value: super-secret
        value: c3VwZXItc2VjcmV0
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got, want := cfg.Rules[0].Validators[0].Value, "super-secret"; got != want {
		t.Fatalf("validator value = %q, want %q", got, want)
	}
}

func TestLoadFailsWhenEnvVarMissing(t *testing.T) {
	path := writeTempConfig(t, `
rules:
  - name: ingest
    host:
      type: exact
      value: ingest-grpc.example.com
    paths:
      - type: prefix
        value: /v1/traces
    validators:
      - type: header_exact
        header: x-api-key
        value: ${OTEL_INGEST_API_KEY_BASE64}
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load returned nil error")
	}
	if !strings.Contains(err.Error(), `missing env var "OTEL_INGEST_API_KEY_BASE64"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadFailsWhenValidatorValueIsNotBase64(t *testing.T) {
	path := writeTempConfig(t, `
rules:
  - name: ingest
    host:
      type: exact
      value: ingest-grpc.example.com
    paths:
      - type: prefix
        value: /v1/traces
    validators:
      - type: header_exact
        header: x-api-key
        value: not-base64
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load returned nil error")
	}
	if !strings.Contains(err.Error(), `rule "ingest": validator "x-api-key" value must be base64`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadFailsWhenDecodedValidatorValueIsEmpty(t *testing.T) {
	// actual value: empty string
	t.Setenv("OTEL_INGEST_API_KEY_BASE64", "")

	path := writeTempConfig(t, `
rules:
  - name: ingest
    host:
      type: exact
      value: ingest-grpc.example.com
    paths:
      - type: prefix
        value: /v1/traces
    validators:
      - type: header_exact
        header: x-api-key
        # actual value from OTEL_INGEST_API_KEY_BASE64: empty string
        value: ${OTEL_INGEST_API_KEY_BASE64}
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load returned nil error")
	}
	if !strings.Contains(err.Error(), `rule "ingest": validator "x-api-key" decoded value is empty`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadFailsForInvalidMatcher(t *testing.T) {
	path := writeTempConfig(t, `
rules:
  - name: ingest
    host:
      type: something
      value: ingest-grpc.example.com
    paths:
      - type: prefix
        value: /v1/traces
    validators:
      - type: header_exact
        header: x-api-key
        # actual value: secret
        value: c2VjcmV0
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load returned nil error")
	}
	if !strings.Contains(err.Error(), `unsupported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadFailsForInvalidAccessLogFormat(t *testing.T) {
	path := writeTempConfig(t, `
server:
  access_log:
    format: text
rules:
  - name: ingest
    host:
      type: exact
      value: ingest-grpc.example.com
    paths:
      - type: prefix
        value: /v1/traces
    validators:
      - type: header_exact
        header: x-api-key
        # actual value: secret
        value: c2VjcmV0
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load returned nil error")
	}
	if !strings.Contains(err.Error(), `server.access_log.format "text" is unsupported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}
