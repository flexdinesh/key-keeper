package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flexdinesh/key-keeper/internal/auth"
	"github.com/flexdinesh/key-keeper/internal/config"
)

func TestHandleAuthStatusCodes(t *testing.T) {
	handler := newTestHandler()

	testCases := []struct {
		name       string
		headers    map[string]string
		wantStatus int
	}{
		{
			name: "bad request when forwarded host missing",
			headers: map[string]string{
				"X-Forwarded-Uri": "/v1/traces",
				"X-Api-Key":       "secret",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "unauthorized when auth header missing",
			headers: map[string]string{
				"X-Forwarded-Host": "ingest-grpc.example.com",
				"X-Forwarded-Uri":  "/v1/traces",
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "forbidden when auth header wrong",
			headers: map[string]string{
				"X-Forwarded-Host": "ingest-grpc.example.com",
				"X-Forwarded-Uri":  "/v1/traces",
				"X-Api-Key":        "wrong",
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "ok when authorized",
			headers: map[string]string{
				"X-Forwarded-Host": "ingest-grpc.example.com",
				"X-Forwarded-Uri":  "/v1/traces?qs=1",
				"X-Api-Key":        "secret",
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/auth", nil)
			for key, value := range tc.headers {
				req.Header.Set(key, value)
			}

			recorder := httptest.NewRecorder()
			handler.Routes().ServeHTTP(recorder, req)

			if got := recorder.Code; got != tc.wantStatus {
				t.Fatalf("status = %d, want %d", got, tc.wantStatus)
			}
		})
	}
}

func TestAccessLogJSONFormat(t *testing.T) {
	var logOutput strings.Builder
	logger := slog.New(slog.NewJSONHandler(&logOutput, nil))
	handler := NewHandler(sampleEvaluator(), logger, config.AccessLogConfig{Format: "json"}, &logOutput)

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "ingest-grpc.example.com")
	req.Header.Set("X-Forwarded-Uri", "/v1/traces")
	req.Header.Set("X-Api-Key", "secret")
	req.Header.Set("User-Agent", "otel-client/1.0")
	req.RemoteAddr = "127.0.0.1:12345"

	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, req)

	output := logOutput.String()
	if !strings.Contains(output, `"msg":"auth access"`) {
		t.Fatalf("expected auth access log, got %q", output)
	}
	if !strings.Contains(output, `"status":200`) {
		t.Fatalf("expected status in log, got %q", output)
	}
	if !strings.Contains(output, `"decision":"allowed"`) {
		t.Fatalf("expected decision in log, got %q", output)
	}
	if !strings.Contains(output, `"validator_header":"x-api-key"`) {
		t.Fatalf("expected validator header in log, got %q", output)
	}
	if !strings.Contains(output, `"validator_value":"[REDACTED]"`) {
		t.Fatalf("expected validator value in log, got %q", output)
	}
	if strings.Contains(output, `"validator_value":"secret"`) {
		t.Fatalf("expected secret redacted, got %q", output)
	}
}

func TestAccessLogLogfmtFormat(t *testing.T) {
	var logOutput strings.Builder
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(sampleEvaluator(), logger, config.AccessLogConfig{Format: "logfmt"}, &logOutput)

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "ingest-grpc.example.com")
	req.Header.Set("X-Forwarded-Uri", "/v1/traces")
	req.Header.Set("X-Api-Key", "wrong")
	req.Header.Set("User-Agent", "otel client")
	req.RemoteAddr = "127.0.0.1:12345"

	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, req)

	output := logOutput.String()
	if !strings.Contains(output, `msg="auth access"`) {
		t.Fatalf("expected auth access log, got %q", output)
	}
	if !strings.Contains(output, `status=403`) {
		t.Fatalf("expected status in log, got %q", output)
	}
	if !strings.Contains(output, `decision=denied_wrong_value`) {
		t.Fatalf("expected decision in log, got %q", output)
	}
	if !strings.Contains(output, `reason="invalid value for header x-api-key"`) {
		t.Fatalf("expected deny reason in log, got %q", output)
	}
	if !strings.Contains(output, `validator_header=x-api-key`) {
		t.Fatalf("expected validator header in log, got %q", output)
	}
	if !strings.Contains(output, `validator_value=[REDACTED]`) {
		t.Fatalf("expected validator value in log, got %q", output)
	}
	if strings.Contains(output, `validator_value=wrong`) {
		t.Fatalf("expected wrong value redacted, got %q", output)
	}
	if !strings.Contains(output, `forwarded_uri=/v1/traces`) {
		t.Fatalf("expected forwarded uri in log, got %q", output)
	}
}

func TestAccessLogMarksMissingValidatorValue(t *testing.T) {
	var logOutput strings.Builder
	logger := slog.New(slog.NewJSONHandler(&logOutput, nil))
	handler := NewHandler(sampleEvaluator(), logger, config.AccessLogConfig{Format: "json"}, &logOutput)

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "ingest-grpc.example.com")
	req.Header.Set("X-Forwarded-Uri", "/v1/traces")

	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, req)

	output := logOutput.String()
	if !strings.Contains(output, `"validator_value":"[MISSING]"`) {
		t.Fatalf("expected missing marker in log, got %q", output)
	}
}

func TestAccessLogMarksEmptyValidatorValue(t *testing.T) {
	var logOutput strings.Builder
	logger := slog.New(slog.NewJSONHandler(&logOutput, nil))
	handler := NewHandler(sampleEvaluator(), logger, config.AccessLogConfig{Format: "json"}, &logOutput)

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "ingest-grpc.example.com")
	req.Header.Set("X-Forwarded-Uri", "/v1/traces")
	req.Header.Set("X-Api-Key", "")

	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, req)

	output := logOutput.String()
	if !strings.Contains(output, `"validator_value":"[EMPTY]"`) {
		t.Fatalf("expected empty marker in log, got %q", output)
	}
}

func TestHandleAuthDecisionLogsNeverContainRawValidatorValue(t *testing.T) {
	var logOutput strings.Builder
	logger := slog.New(slog.NewJSONHandler(&logOutput, nil))
	handler := NewHandler(sampleEvaluator(), logger, config.AccessLogConfig{Format: "json"}, io.Discard)

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "ingest-grpc.example.com")
	req.Header.Set("X-Forwarded-Uri", "/v1/traces")
	req.Header.Set("X-Api-Key", "wrong")

	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, req)

	output := logOutput.String()
	if !strings.Contains(output, `"msg":"request denied"`) {
		t.Fatalf("expected decision log, got %q", output)
	}
	if !strings.Contains(output, `"validator_value":"[REDACTED]"`) {
		t.Fatalf("expected redacted validator value, got %q", output)
	}
	if strings.Contains(output, `"validator_value":"wrong"`) {
		t.Fatalf("expected raw validator value omitted, got %q", output)
	}
}

func newTestHandler() Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(sampleEvaluator(), logger, config.AccessLogConfig{Format: "json"}, io.Discard)
}

func sampleEvaluator() auth.Evaluator {
	return auth.NewEvaluator([]config.Rule{
		{
			Name: "otel",
			Host: config.Matcher{Type: "exact", Value: "ingest-grpc.example.com"},
			Paths: []config.Matcher{
				{Type: "prefix", Value: "/v1/traces"},
			},
			Validators: []config.Validator{
				{Type: "header_exact", Header: "x-api-key", Value: "secret"},
			},
		},
	})
}
