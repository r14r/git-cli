package precommit

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gitutil "git-cli/internal/git"
)

const ConfigFilename = ".git-cli-precommit.yaml"

var supported = []string{"python", "fastapi", "django", "laravel"}

type Config struct {
	Preset string
}

func Supported() []string { return append([]string(nil), supported...) }

func NormalizePreset(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "laracel" { // common typo, kept as a compatibility alias
		v = "laravel"
	}
	for _, p := range supported {
		if v == p {
			return v, nil
		}
	}
	return "", fmt.Errorf("unsupported precommit preset %q (supported: %s)", v, strings.Join(supported, ", "))
}

func Detect(root string) (string, error) {
	if fileExists(filepath.Join(root, "artisan")) && fileContains(filepath.Join(root, "composer.json"), "laravel/framework") {
		return "laravel", nil
	}
	if fileExists(filepath.Join(root, "manage.py")) {
		return "django", nil
	}
	for _, p := range []string{"pyproject.toml", "requirements.txt", "requirements-dev.txt", "Pipfile"} {
		if fileContains(filepath.Join(root, p), "fastapi") {
			return "fastapi", nil
		}
	}
	for _, p := range []string{"pyproject.toml", "requirements.txt", "requirements-dev.txt", "Pipfile", "setup.py", "setup.cfg"} {
		if fileExists(filepath.Join(root, p)) {
			return "python", nil
		}
	}
	return "", errors.New("could not detect application type; use --for python|fastapi|django|laravel")
}

func Load(root string) (Config, error) {
	f, err := os.Open(filepath.Join(root, ConfigFilename))
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	cfg := Config{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == "preset" {
			cfg.Preset = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		}
	}
	if err := s.Err(); err != nil {
		return cfg, err
	}
	p, err := NormalizePreset(cfg.Preset)
	if err != nil {
		return cfg, err
	}
	cfg.Preset = p
	return cfg, nil
}

func WriteConfig(root, preset string) error {
	p, err := NormalizePreset(preset)
	if err != nil {
		return err
	}
	content := fmt.Sprintf("preset: %s\n", p)
	return os.WriteFile(filepath.Join(root, ConfigFilename), []byte(content), 0o644)
}

func HookContent() string {
	return `#!/usr/bin/env bash
set -e

# managed by git-cli precommit
git-cli security check-staged
exec git-cli precommit run
`
}

func ManagedHook(content string) bool {
	return strings.Contains(content, "managed by git-cli precommit") || strings.Contains(content, "git-cli security check-staged")
}

func Setup(root, preset string) (string, error) {
	if err := WriteConfig(root, preset); err != nil {
		return "", err
	}
	gitDir, err := gitutil.GitDir(root)
	if err != nil {
		return "", err
	}
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return "", err
	}
	hook := filepath.Join(hooksDir, "pre-commit")
	if b, err := os.ReadFile(hook); err == nil && !ManagedHook(string(b)) {
		return "", fmt.Errorf("refusing to overwrite existing pre-commit hook: %s", hook)
	}
	if err := os.WriteFile(hook, []byte(HookContent()), 0o755); err != nil {
		return "", err
	}
	return hook, nil
}

func Run(root string, out, errOut io.Writer) error {
	cfg, err := Load(root)
	if err != nil {
		return fmt.Errorf("precommit config: %w", err)
	}
	files, err := gitutil.StagedFiles(root)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Precommit preset: %s\n", cfg.Preset)
	switch cfg.Preset {
	case "python", "fastapi":
		return runPython(root, files, out, errOut)
	case "django":
		if err := runPython(root, files, out, errOut); err != nil {
			return err
		}
		return run(root, out, errOut, pythonCommand(), "manage.py", "check")
	case "laravel":
		return runPHP(root, files, out, errOut)
	default:
		return fmt.Errorf("unsupported preset %q", cfg.Preset)
	}
}

func runPython(root string, files []string, out, errOut io.Writer) error {
	py := filter(files, ".py")
	if len(py) == 0 {
		fmt.Fprintln(out, "Python: no staged .py files")
		return nil
	}
	if commandExists("ruff") {
		args := append([]string{"check"}, py...)
		return run(root, out, errOut, "ruff", args...)
	}
	args := append([]string{"-m", "py_compile"}, py...)
	return run(root, out, errOut, pythonCommand(), args...)
}

func runPHP(root string, files []string, out, errOut io.Writer) error {
	php := filter(files, ".php")
	if len(php) == 0 {
		fmt.Fprintln(out, "Laravel: no staged .php files")
		return nil
	}
	if !commandExists("php") {
		return errors.New("php is required for the laravel precommit preset")
	}
	for _, f := range php {
		if err := run(root, out, errOut, "php", "-l", f); err != nil {
			return err
		}
	}
	return nil
}

func run(root string, out, errOut io.Writer, name string, args ...string) error {
	if !commandExists(name) {
		return fmt.Errorf("required command not installed: %s", name)
	}
	fmt.Fprintf(out, "> %s %s\n", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Dir = root
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func filter(files []string, ext string) []string {
	var out []string
	for _, f := range files {
		if strings.EqualFold(filepath.Ext(f), ext) {
			out = append(out, f)
		}
	}
	return out
}

func pythonCommand() string {
	if commandExists("python3") {
		return "python3"
	}
	return "python"
}

func commandExists(name string) bool { _, err := exec.LookPath(name); return err == nil }
func fileExists(path string) bool     { _, err := os.Stat(path); return err == nil }
func fileContains(path, needle string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(strings.ToLower(string(b)), strings.ToLower(needle))
}
