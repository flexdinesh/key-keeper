package auth

import (
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"github.com/flexdinesh/key-keeper/internal/config"
)

type DecisionStatus string

const (
	StatusAllowed             DecisionStatus = "allowed"
	StatusDeniedNoRule        DecisionStatus = "denied_no_rule"
	StatusDeniedMissingHeader DecisionStatus = "denied_missing_header"
	StatusDeniedWrongValue    DecisionStatus = "denied_wrong_value"
	StatusDeniedBadRequest    DecisionStatus = "denied_bad_request"
)

type Decision struct {
	Status           DecisionStatus
	Rule             string
	Message          string
	ValidatorHeader  string
	ValidatorPresent bool
	ValidatorValue   string
}

type Evaluator struct {
	rules []config.Rule
}

func NewEvaluator(rules []config.Rule) Evaluator {
	return Evaluator{rules: append([]config.Rule(nil), rules...)}
}

func (e Evaluator) Evaluate(r *http.Request) Decision {
	host := normalizeHost(strings.TrimSpace(r.Header.Get("X-Forwarded-Host")))
	if host == "" {
		return Decision{Status: StatusDeniedBadRequest, Message: "missing X-Forwarded-Host"}
	}

	forwardedURI := strings.TrimSpace(r.Header.Get("X-Forwarded-Uri"))
	if forwardedURI == "" {
		return Decision{Status: StatusDeniedBadRequest, Message: "missing X-Forwarded-Uri"}
	}

	parsedURI, err := url.ParseRequestURI(forwardedURI)
	if err != nil {
		return Decision{Status: StatusDeniedBadRequest, Message: "invalid X-Forwarded-Uri"}
	}

	path := parsedURI.Path
	for _, rule := range e.rules {
		if !matches(rule.Host, host) {
			continue
		}
		if !matchesAny(rule.Paths, path) {
			continue
		}

		lastValidatorHeader := ""
		lastValidatorPresent := false
		lastValidatorValue := ""
		for _, validator := range rule.Validators {
			headerName := textproto.CanonicalMIMEHeaderKey(validator.Header)
			headerValues, ok := r.Header[headerName]
			headerValue := ""
			if len(headerValues) > 0 {
				headerValue = headerValues[0]
			}
			lastValidatorHeader = validator.Header
			lastValidatorPresent = ok
			lastValidatorValue = headerValue
			if !ok || headerValue == "" {
				return Decision{
					Status:           StatusDeniedMissingHeader,
					Rule:             rule.Name,
					Message:          "missing required header " + validator.Header,
					ValidatorHeader:  validator.Header,
					ValidatorPresent: ok,
				}
			}
			if headerValue != validator.Value {
				return Decision{
					Status:           StatusDeniedWrongValue,
					Rule:             rule.Name,
					Message:          "invalid value for header " + validator.Header,
					ValidatorHeader:  validator.Header,
					ValidatorPresent: true,
					ValidatorValue:   headerValue,
				}
			}
		}

		return Decision{
			Status:           StatusAllowed,
			Rule:             rule.Name,
			ValidatorHeader:  lastValidatorHeader,
			ValidatorPresent: lastValidatorPresent,
			ValidatorValue:   lastValidatorValue,
		}
	}

	return Decision{Status: StatusDeniedNoRule, Message: "no matching rule"}
}

func matchesAny(matchers []config.Matcher, input string) bool {
	for _, matcher := range matchers {
		if matches(matcher, input) {
			return true
		}
	}
	return false
}

func matches(matcher config.Matcher, input string) bool {
	switch matcher.Type {
	case "exact":
		return input == normalizeMatcherValue(matcher)
	case "prefix":
		return strings.HasPrefix(input, matcher.Value)
	case "wildcard_suffix":
		suffix := strings.TrimPrefix(normalizeMatcherValue(matcher), "*")
		return strings.HasSuffix(input, suffix)
	default:
		return false
	}
}

func normalizeMatcherValue(matcher config.Matcher) string {
	if matcher.Type == "prefix" {
		return matcher.Value
	}
	return normalizeHost(matcher.Value)
}

func normalizeHost(value string) string {
	value = strings.TrimSpace(strings.TrimSuffix(value, "."))
	if value == "" {
		return ""
	}

	if strings.HasPrefix(value, "[") {
		if host, _, err := net.SplitHostPort(value); err == nil {
			value = host
		} else if strings.HasSuffix(value, "]") {
			value = strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
		}
	} else if strings.Count(value, ":") == 1 {
		if host, _, err := net.SplitHostPort(value); err == nil {
			value = host
		}
	}

	return strings.ToLower(strings.TrimSuffix(value, "."))
}
