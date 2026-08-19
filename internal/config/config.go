package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

const DefaultFilename = ".git-cli.yaml"

type ScannerConfig struct {
	Enabled  bool
	Required bool
}

type Config struct {
	FailOn         string
	Scanners       map[string]ScannerConfig
	GitleaksConfig string
	Baseline       string
}

func Default() Config {
	return Config{
		FailOn: "high",
		Scanners: map[string]ScannerConfig{
			"gitleaks":       {Enabled: true, Required: true},
			"detect-secrets": {Enabled: true, Required: true},
			"trufflehog":     {Enabled: true, Required: false},
		},
		Baseline: ".secrets.baseline",
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	currentScanner := ""
	s := bufio.NewScanner(f)
	for s.Scan() {
		raw := s.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))

		if indent == 0 {
			currentScanner = ""
			key, value, ok := splitKV(line)
			if !ok {
				continue
			}
			switch key {
			case "fail_on":
				cfg.FailOn = unquote(value)
			case "gitleaks_config":
				cfg.GitleaksConfig = unquote(value)
			case "detect_secrets_baseline":
				cfg.Baseline = unquote(value)
			}
			continue
		}

		if indent == 2 && strings.HasSuffix(line, ":") {
			name := strings.TrimSuffix(line, ":")
			if _, ok := cfg.Scanners[name]; ok {
				currentScanner = name
			}
			continue
		}

		if indent >= 4 && currentScanner != "" {
			key, value, ok := splitKV(line)
			if !ok {
				continue
			}
			sc := cfg.Scanners[currentScanner]
			switch key {
			case "enabled":
				v, err := parseBool(value)
				if err != nil {
					return cfg, fmt.Errorf("%s.%s: %w", currentScanner, key, err)
				}
				sc.Enabled = v
			case "required":
				v, err := parseBool(value)
				if err != nil {
					return cfg, fmt.Errorf("%s.%s: %w", currentScanner, key, err)
				}
				sc.Required = v
			}
			cfg.Scanners[currentScanner] = sc
		}
	}
	if err := s.Err(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func splitKV(line string) (string, string, bool) {
	p := strings.SplitN(line, ":", 2)
	if len(p) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(p[0]), strings.TrimSpace(p[1]), true
}

func unquote(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
		return v[1 : len(v)-1]
	}
	return v
}

func parseBool(v string) (bool, error) {
	switch strings.ToLower(unquote(v)) {
	case "true", "yes", "on":
		return true, nil
	case "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", v)
	}
}

func WriteDefault(path string) error {
	const content = `fail_on: high
scanners:
  gitleaks:
    enabled: true
    required: true
  detect-secrets:
    enabled: true
    required: true
  trufflehog:
    enabled: true
    required: false
detect_secrets_baseline: .secrets.baseline
# gitleaks_config: .gitleaks.toml
`
	return os.WriteFile(path, []byte(content), 0o644)
}
