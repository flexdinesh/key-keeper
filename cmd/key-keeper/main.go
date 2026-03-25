package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/flexdinesh/key-keeper/internal/auth"
	"github.com/flexdinesh/key-keeper/internal/config"
	"github.com/flexdinesh/key-keeper/internal/httpserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))

	configPath := os.Getenv("KEY_KEEPER_CONFIG")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	evaluator := auth.NewEvaluator(cfg.Rules)
	handler := httpserver.NewHandler(evaluator, logger, cfg.Server.AccessLog, os.Stdout)

	server := &http.Server{
		Addr:    cfg.Server.ListenAddr,
		Handler: handler.Routes(),
	}

	logger.Info(
		"starting server",
		"listen_addr", cfg.Server.ListenAddr,
		"config_path", configPath,
		"rules", len(cfg.Rules),
		"rule_summary", summarizeRules(cfg.Rules),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown failed", "error", err)
		}
	}()

	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

func summarizeRules(rules []config.Rule) []map[string]any {
	summary := make([]map[string]any, 0, len(rules))
	for _, rule := range rules {
		summary = append(summary, map[string]any{
			"name":              rule.Name,
			"host_type":         rule.Host.Type,
			"host_value":        rule.Host.Value,
			"path_matchers":     summarizeMatchers(rule.Paths),
			"validator_headers": summarizeValidatorHeaders(rule.Validators),
		})
	}
	return summary
}

func summarizeMatchers(matchers []config.Matcher) []string {
	values := make([]string, 0, len(matchers))
	for _, matcher := range matchers {
		values = append(values, matcher.Type+":"+matcher.Value)
	}
	return values
}

func summarizeValidatorHeaders(validators []config.Validator) []string {
	headers := make([]string, 0, len(validators))
	for _, validator := range validators {
		headers = append(headers, validator.Type+":"+strings.ToLower(validator.Header))
	}
	return headers
}
