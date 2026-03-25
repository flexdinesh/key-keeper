package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/flexdinesh/key-keeper/internal/config"
)

func TestEvaluateAllowsMatchingRule(t *testing.T) {
	evaluator := NewEvaluator([]config.Rule{
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

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "INGEST-GRPC.EXAMPLE.COM")
	req.Header.Set("X-Forwarded-Uri", "/v1/traces?foo=bar")
	req.Header.Set("X-Api-Key", "secret")

	decision := evaluator.Evaluate(req)
	if decision.Status != StatusAllowed {
		t.Fatalf("status = %q, want %q", decision.Status, StatusAllowed)
	}
	if decision.ValidatorHeader != "x-api-key" {
		t.Fatalf("validator header = %q, want %q", decision.ValidatorHeader, "x-api-key")
	}
	if !decision.ValidatorPresent {
		t.Fatal("validator present = false, want true")
	}
	if decision.ValidatorValue != "secret" {
		t.Fatalf("validator value = %q, want %q", decision.ValidatorValue, "secret")
	}
}

func TestEvaluateAllowsMatchingRuleWhenForwardedHostIncludesPort(t *testing.T) {
	evaluator := NewEvaluator(sampleRules())

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "ingest-grpc.example.com:443")
	req.Header.Set("X-Forwarded-Uri", "/v1/traces")
	req.Header.Set("X-Api-Key", "secret")

	decision := evaluator.Evaluate(req)
	if decision.Status != StatusAllowed {
		t.Fatalf("status = %q, want %q", decision.Status, StatusAllowed)
	}
}

func TestEvaluateMissingHeaderReturnsUnauthorized(t *testing.T) {
	evaluator := NewEvaluator(sampleRules())

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "ingest-grpc.example.com")
	req.Header.Set("X-Forwarded-Uri", "/v1/traces")

	decision := evaluator.Evaluate(req)
	if decision.Status != StatusDeniedMissingHeader {
		t.Fatalf("status = %q, want %q", decision.Status, StatusDeniedMissingHeader)
	}
	if decision.ValidatorHeader != "x-api-key" {
		t.Fatalf("validator header = %q, want %q", decision.ValidatorHeader, "x-api-key")
	}
	if decision.ValidatorPresent {
		t.Fatal("validator present = true, want false")
	}
	if decision.ValidatorValue != "" {
		t.Fatalf("validator value = %q, want empty", decision.ValidatorValue)
	}
}

func TestEvaluateEmptyHeaderReturnsUnauthorized(t *testing.T) {
	evaluator := NewEvaluator(sampleRules())

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "ingest-grpc.example.com")
	req.Header.Set("X-Forwarded-Uri", "/v1/traces")
	req.Header.Set("X-Api-Key", "")

	decision := evaluator.Evaluate(req)
	if decision.Status != StatusDeniedMissingHeader {
		t.Fatalf("status = %q, want %q", decision.Status, StatusDeniedMissingHeader)
	}
	if decision.ValidatorHeader != "x-api-key" {
		t.Fatalf("validator header = %q, want %q", decision.ValidatorHeader, "x-api-key")
	}
	if !decision.ValidatorPresent {
		t.Fatal("validator present = false, want true")
	}
	if decision.ValidatorValue != "" {
		t.Fatalf("validator value = %q, want empty", decision.ValidatorValue)
	}
}

func TestEvaluateWrongHeaderValueReturnsForbidden(t *testing.T) {
	evaluator := NewEvaluator(sampleRules())

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "ingest-grpc.example.com")
	req.Header.Set("X-Forwarded-Uri", "/v1/traces")
	req.Header.Set("X-Api-Key", "wrong")

	decision := evaluator.Evaluate(req)
	if decision.Status != StatusDeniedWrongValue {
		t.Fatalf("status = %q, want %q", decision.Status, StatusDeniedWrongValue)
	}
	if decision.ValidatorHeader != "x-api-key" {
		t.Fatalf("validator header = %q, want %q", decision.ValidatorHeader, "x-api-key")
	}
	if !decision.ValidatorPresent {
		t.Fatal("validator present = false, want true")
	}
	if decision.ValidatorValue != "wrong" {
		t.Fatalf("validator value = %q, want %q", decision.ValidatorValue, "wrong")
	}
}

func TestEvaluateNoMatchingRule(t *testing.T) {
	evaluator := NewEvaluator(sampleRules())

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "other.example.com")
	req.Header.Set("X-Forwarded-Uri", "/v1/traces")
	req.Header.Set("X-Api-Key", "secret")

	decision := evaluator.Evaluate(req)
	if decision.Status != StatusDeniedNoRule {
		t.Fatalf("status = %q, want %q", decision.Status, StatusDeniedNoRule)
	}
}

func TestEvaluateWildcardHostMatches(t *testing.T) {
	evaluator := NewEvaluator([]config.Rule{
		{
			Name: "wildcard",
			Host: config.Matcher{Type: "wildcard_suffix", Value: "*.customer.example.com"},
			Paths: []config.Matcher{
				{Type: "exact", Value: "/v1/traces"},
			},
			Validators: []config.Validator{
				{Type: "header_exact", Header: "x-api-key", Value: "secret"},
			},
		},
	})

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "tenant.customer.example.com")
	req.Header.Set("X-Forwarded-Uri", "/v1/traces")
	req.Header.Set("X-Api-Key", "secret")

	decision := evaluator.Evaluate(req)
	if decision.Status != StatusAllowed {
		t.Fatalf("status = %q, want %q", decision.Status, StatusAllowed)
	}
}

func TestEvaluateWildcardHostMatchesWhenForwardedHostIncludesPort(t *testing.T) {
	evaluator := NewEvaluator([]config.Rule{
		{
			Name: "wildcard",
			Host: config.Matcher{Type: "wildcard_suffix", Value: "*.customer.example.com"},
			Paths: []config.Matcher{
				{Type: "exact", Value: "/v1/traces"},
			},
			Validators: []config.Validator{
				{Type: "header_exact", Header: "x-api-key", Value: "secret"},
			},
		},
	})

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "tenant.customer.example.com:443")
	req.Header.Set("X-Forwarded-Uri", "/v1/traces")
	req.Header.Set("X-Api-Key", "secret")

	decision := evaluator.Evaluate(req)
	if decision.Status != StatusAllowed {
		t.Fatalf("status = %q, want %q", decision.Status, StatusAllowed)
	}
}

func TestEvaluateAllowsAnyConfiguredPathInRule(t *testing.T) {
	evaluator := NewEvaluator([]config.Rule{
		{
			Name: "multi-path",
			Host: config.Matcher{Type: "exact", Value: "ingest-http.example.com"},
			Paths: []config.Matcher{
				{Type: "exact", Value: "/v1/traces"},
				{Type: "exact", Value: "/v1/metrics"},
			},
			Validators: []config.Validator{
				{Type: "header_exact", Header: "x-api-key", Value: "secret"},
			},
		},
	})

	req := httptest.NewRequest("GET", "/auth", nil)
	req.Header.Set("X-Forwarded-Host", "ingest-http.example.com")
	req.Header.Set("X-Forwarded-Uri", "/v1/metrics")
	req.Header.Set("X-Api-Key", "secret")

	decision := evaluator.Evaluate(req)
	if decision.Status != StatusAllowed {
		t.Fatalf("status = %q, want %q", decision.Status, StatusAllowed)
	}
}

func sampleRules() []config.Rule {
	return []config.Rule{
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
	}
}
