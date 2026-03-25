package httpserver

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/flexdinesh/key-keeper/internal/auth"
	"github.com/flexdinesh/key-keeper/internal/config"
)

type Handler struct {
	evaluator auth.Evaluator
	logger    *slog.Logger
	logConfig config.AccessLogConfig
	logWriter io.Writer
}

const (
	decisionHeader = "X-Key-Keeper-Decision"
	ruleHeader     = "X-Key-Keeper-Rule"
	reasonHeader   = "X-Key-Keeper-Reason"
	redactedValue  = "[REDACTED]"
	missingValue   = "[MISSING]"
	emptyValue     = "[EMPTY]"
)

func NewHandler(evaluator auth.Evaluator, logger *slog.Logger, logConfig config.AccessLogConfig, logWriter io.Writer) Handler {
	return Handler{
		evaluator: evaluator,
		logger:    logger,
		logConfig: logConfig,
		logWriter: logWriter,
	}
}

func (h Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/auth", h.handleAuth)
	return h.withAccessLog(mux)
}

func (h Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h Handler) handleAuth(w http.ResponseWriter, r *http.Request) {
	decision := h.evaluator.Evaluate(r)
	if recorder, ok := w.(interface {
		SetAuthMetadata(auth.Decision)
	}); ok {
		recorder.SetAuthMetadata(decision)
	}
	w.Header().Set(decisionHeader, string(decision.Status))
	if decision.Rule != "" {
		w.Header().Set(ruleHeader, decision.Rule)
	}
	if decision.Message != "" {
		w.Header().Set(reasonHeader, decision.Message)
	}

	switch decision.Status {
	case auth.StatusAllowed:
		h.logger.Info(
			"request authorized",
			"rule", decision.Rule,
			"validator_header", decision.ValidatorHeader,
			"validator_value", formatValidatorValue(decision.ValidatorPresent, decision.ValidatorValue),
		)
		w.WriteHeader(http.StatusOK)
	case auth.StatusDeniedMissingHeader:
		h.logger.Warn(
			"request denied",
			"rule", decision.Rule,
			"reason", decision.Message,
			"status", http.StatusUnauthorized,
			"validator_header", decision.ValidatorHeader,
			"validator_value", formatValidatorValue(decision.ValidatorPresent, decision.ValidatorValue),
		)
		http.Error(w, decision.Message, http.StatusUnauthorized)
	case auth.StatusDeniedWrongValue:
		h.logger.Warn(
			"request denied",
			"rule", decision.Rule,
			"reason", decision.Message,
			"status", http.StatusForbidden,
			"validator_header", decision.ValidatorHeader,
			"validator_value", formatValidatorValue(decision.ValidatorPresent, decision.ValidatorValue),
		)
		http.Error(w, decision.Message, http.StatusForbidden)
	case auth.StatusDeniedBadRequest:
		h.logger.Warn("invalid auth request", "reason", decision.Message, "status", http.StatusBadRequest)
		http.Error(w, decision.Message, http.StatusBadRequest)
	default:
		h.logger.Warn("request denied", "reason", decision.Message, "status", http.StatusForbidden)
		http.Error(w, decision.Message, http.StatusForbidden)
	}
}

func (h Handler) withAccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(recorder, r)

		fields := accessLogFields{
			Method:          r.Method,
			ForwardedHost:   r.Header.Get("X-Forwarded-Host"),
			ForwardedURI:    r.Header.Get("X-Forwarded-Uri"),
			Path:            r.URL.Path,
			Status:          recorder.statusCode,
			Decision:        recorder.Header().Get(decisionHeader),
			Rule:            recorder.Header().Get(ruleHeader),
			Reason:          recorder.Header().Get(reasonHeader),
			ValidatorHeader: recorder.validatorHeader,
			ValidatorValue:  formatValidatorValue(recorder.validatorPresent, recorder.validatorValue),
			Duration:        time.Since(start),
			RemoteAddr:      r.RemoteAddr,
			ForwardedFor:    firstForwardedFor(r.Header.Get("X-Forwarded-For")),
			UserAgent:       r.UserAgent(),
			ContentLength:   r.ContentLength,
			ForwardedProto:  r.Header.Get("X-Forwarded-Proto"),
		}

		switch h.logConfig.Format {
		case "logfmt":
			_, _ = fmt.Fprintln(h.logWriter, formatLogfmt(fields))
		default:
			h.logger.Info(
				"auth access",
				"method", fields.Method,
				"forwarded_host", fields.ForwardedHost,
				"forwarded_uri", fields.ForwardedURI,
				"path", fields.Path,
				"status", fields.Status,
				"decision", fields.Decision,
				"rule", fields.Rule,
				"reason", fields.Reason,
				"validator_header", fields.ValidatorHeader,
				"validator_value", fields.ValidatorValue,
				"duration_ms", fields.Duration.Milliseconds(),
				"remote_addr", fields.RemoteAddr,
				"forwarded_for", fields.ForwardedFor,
				"user_agent", fields.UserAgent,
				"content_length", fields.ContentLength,
				"forwarded_proto", fields.ForwardedProto,
			)
		}
	})
}

type accessLogFields struct {
	Method          string
	ForwardedHost   string
	ForwardedURI    string
	Path            string
	Status          int
	Decision        string
	Rule            string
	Reason          string
	ValidatorHeader string
	ValidatorValue  string
	Duration        time.Duration
	RemoteAddr      string
	ForwardedFor    string
	UserAgent       string
	ContentLength   int64
	ForwardedProto  string
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode       int
	validatorHeader  string
	validatorPresent bool
	validatorValue   string
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) SetAuthMetadata(decision auth.Decision) {
	r.validatorHeader = decision.ValidatorHeader
	r.validatorPresent = decision.ValidatorPresent
	r.validatorValue = decision.ValidatorValue
}

func formatValidatorValue(present bool, value string) string {
	if !present {
		return missingValue
	}
	if value == "" {
		return emptyValue
	}
	return redactedValue
}

func formatLogfmt(fields accessLogFields) string {
	var builder strings.Builder
	writeLogfmtKV(&builder, "msg", "auth access")
	writeLogfmtKV(&builder, "method", fields.Method)
	writeLogfmtKV(&builder, "forwarded_host", fields.ForwardedHost)
	writeLogfmtKV(&builder, "forwarded_uri", fields.ForwardedURI)
	writeLogfmtKV(&builder, "path", fields.Path)
	writeLogfmtKV(&builder, "status", fmt.Sprintf("%d", fields.Status))
	writeLogfmtKV(&builder, "decision", fields.Decision)
	writeLogfmtKV(&builder, "rule", fields.Rule)
	writeLogfmtKV(&builder, "reason", fields.Reason)
	writeLogfmtKV(&builder, "validator_header", fields.ValidatorHeader)
	writeLogfmtKV(&builder, "validator_value", fields.ValidatorValue)
	writeLogfmtKV(&builder, "duration_ms", fmt.Sprintf("%d", fields.Duration.Milliseconds()))
	writeLogfmtKV(&builder, "remote_addr", fields.RemoteAddr)
	writeLogfmtKV(&builder, "forwarded_for", fields.ForwardedFor)
	writeLogfmtKV(&builder, "user_agent", fields.UserAgent)
	writeLogfmtKV(&builder, "content_length", fmt.Sprintf("%d", fields.ContentLength))
	writeLogfmtKV(&builder, "forwarded_proto", fields.ForwardedProto)
	return builder.String()
}

func writeLogfmtKV(builder io.StringWriter, key string, value string) {
	if value == "" {
		value = "-"
	}
	_, _ = builder.WriteString(key)
	_, _ = builder.WriteString("=")
	if needsQuote(value) {
		_, _ = builder.WriteString(strconvQuote(value))
	} else {
		_, _ = builder.WriteString(value)
	}
	_, _ = builder.WriteString(" ")
}

func needsQuote(value string) bool {
	return strings.ContainsAny(value, " \t\n\r\"=")
}

func strconvQuote(value string) string {
	return fmt.Sprintf("%q", value)
}

func firstForwardedFor(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	return strings.TrimSpace(parts[0])
}
