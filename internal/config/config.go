package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var envPattern = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Rules  []Rule       `yaml:"rules"`
}

type ServerConfig struct {
	ListenAddr      string          `yaml:"listen_addr"`
	ShutdownTimeout time.Duration   `yaml:"shutdown_timeout"`
	AccessLog       AccessLogConfig `yaml:"access_log"`
}

type AccessLogConfig struct {
	Format string `yaml:"format"`
}

type Rule struct {
	Name       string      `yaml:"name"`
	Host       Matcher     `yaml:"host"`
	Paths      []Matcher   `yaml:"paths"`
	Validators []Validator `yaml:"validators"`
}

type Matcher struct {
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

type Validator struct {
	Type   string `yaml:"type"`
	Header string `yaml:"header"`
	Value  string `yaml:"value"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	expanded, err := expandEnvPlaceholders(string(data))
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	applyDefaults(&cfg)
	if err := decodeValidatorValues(&cfg); err != nil {
		return Config{}, err
	}
	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func expandEnvPlaceholders(raw string) (string, error) {
	var expandErr error

	expanded := envPattern.ReplaceAllStringFunc(raw, func(match string) string {
		if expandErr != nil {
			return match
		}

		submatches := envPattern.FindStringSubmatch(match)
		if len(submatches) != 2 {
			return match
		}

		value, ok := os.LookupEnv(submatches[1])
		if !ok {
			expandErr = fmt.Errorf("missing env var %q", submatches[1])
			return match
		}

		return value
	})

	if expandErr != nil {
		return "", expandErr
	}

	return expanded, nil
}

func decodeValidatorValues(cfg *Config) error {
	for ruleIndex := range cfg.Rules {
		rule := &cfg.Rules[ruleIndex]
		for validatorIndex := range rule.Validators {
			validator := &rule.Validators[validatorIndex]
			decodedValue, err := decodeBase64Value(validator.Value)
			if err != nil {
				return fmt.Errorf("rule %q: validator %q value must be base64: %w", ruleLabel(*rule, ruleIndex), validator.Header, err)
			}
			if decodedValue == "" {
				return fmt.Errorf("rule %q: validator %q decoded value is empty", ruleLabel(*rule, ruleIndex), validator.Header)
			}
			validator.Value = decodedValue
		}
	}

	return nil
}

func decodeBase64Value(value string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err == nil {
		return string(decoded), nil
	}

	decoded, rawErr := base64.RawStdEncoding.DecodeString(value)
	if rawErr == nil {
		return string(decoded), nil
	}

	return "", err
}

func ruleLabel(rule Rule, index int) string {
	if strings.TrimSpace(rule.Name) != "" {
		return rule.Name
	}
	return fmt.Sprintf("#%d", index)
}

func applyDefaults(cfg *Config) {
	if cfg.Server.ListenAddr == "" {
		cfg.Server.ListenAddr = ":8080"
	}
	if cfg.Server.ShutdownTimeout == 0 {
		cfg.Server.ShutdownTimeout = 5 * time.Second
	}
	if cfg.Server.AccessLog.Format == "" {
		cfg.Server.AccessLog.Format = "json"
	}
}

func validate(cfg Config) error {
	if len(cfg.Rules) == 0 {
		return fmt.Errorf("config must define at least one rule")
	}

	for i, rule := range cfg.Rules {
		if strings.TrimSpace(rule.Name) == "" {
			return fmt.Errorf("rule %d: name is required", i)
		}
		if err := validateMatcher("host", rule.Host); err != nil {
			return fmt.Errorf("rule %q: %w", rule.Name, err)
		}
		if len(rule.Paths) == 0 {
			return fmt.Errorf("rule %q: at least one path matcher is required", rule.Name)
		}
		for _, pathMatcher := range rule.Paths {
			if err := validateMatcher("path", pathMatcher); err != nil {
				return fmt.Errorf("rule %q: %w", rule.Name, err)
			}
		}
		if len(rule.Validators) == 0 {
			return fmt.Errorf("rule %q: at least one validator is required", rule.Name)
		}
		for _, validator := range rule.Validators {
			if validator.Type != "header_exact" {
				return fmt.Errorf("rule %q: unsupported validator type %q", rule.Name, validator.Type)
			}
			if strings.TrimSpace(validator.Header) == "" {
				return fmt.Errorf("rule %q: validator header is required", rule.Name)
			}
			if validator.Value == "" {
				return fmt.Errorf("rule %q: validator %q value is required", rule.Name, validator.Header)
			}
		}
	}

	switch cfg.Server.AccessLog.Format {
	case "json", "logfmt":
	default:
		return fmt.Errorf("server.access_log.format %q is unsupported", cfg.Server.AccessLog.Format)
	}

	return nil
}

func validateMatcher(name string, matcher Matcher) error {
	switch matcher.Type {
	case "exact", "prefix", "wildcard_suffix":
	default:
		return fmt.Errorf("%s matcher type %q is unsupported", name, matcher.Type)
	}

	if strings.TrimSpace(matcher.Value) == "" {
		return fmt.Errorf("%s matcher value is required", name)
	}

	if name == "path" && !strings.HasPrefix(matcher.Value, "/") {
		return fmt.Errorf("path matcher value must start with '/'")
	}

	if name == "host" && matcher.Type == "wildcard_suffix" && !strings.HasPrefix(matcher.Value, "*.") {
		return fmt.Errorf("wildcard host matcher value must start with '*.'")
	}

	return nil
}
