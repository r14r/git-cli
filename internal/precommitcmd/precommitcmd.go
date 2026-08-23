package precommitcmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git-cli/internal/config"
	gitutil "git-cli/internal/git"
	"git-cli/internal/precommit"
)

const managedMarker = "managed by git-cli precommit"

func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		help(out)
		return 0
	}

	switch args[0] {
	case "run":
		root, err := gitutil.Root()
		if err != nil { fmt.Fprintln(errOut, err); return 2 }
		if err := runStaged(root, out, errOut); err != nil { fmt.Fprintln(errOut, err); return 1 }
		return 0
	case "status":
		return status(out, errOut)
	case "list":
		fmt.Fprintln(out, strings.Join(precommit.Supported(), "\n"))
		return 0
	case "uninstall":
		return uninstall(out, errOut)
	}

	fs := flag.NewFlagSet("precommit", flag.ContinueOnError)
	fs.SetOutput(errOut)
	setup := fs.Bool("setup", false, "install pre-commit configuration and hook")
	preset := fs.String("for", "", "application preset")
	scan := fs.Bool("scan", false, "detect application preset")
	if err := fs.Parse(args); err != nil { return 2 }
	if !*setup { fmt.Fprintln(errOut, "precommit requires --setup, or use run|status|list|uninstall"); return 2 }
	if (*preset == "" && !*scan) || (*preset != "" && *scan) { fmt.Fprintln(errOut, "use exactly one of --for <preset> or --scan"); return 2 }

	root, err := gitutil.Root()
	if err != nil { fmt.Fprintln(errOut, err); return 2 }
	selected := *preset
	if *scan {
		selected, err = precommit.Detect(root)
		if err != nil { fmt.Fprintln(errOut, err); return 2 }
		fmt.Fprintf(out, "Detected application preset: %s\n", selected)
	}
	selected, err = precommit.NormalizePreset(selected)
	if err != nil { fmt.Fprintln(errOut, err); return 2 }
	if err := precommit.WriteConfig(root, selected); err != nil { fmt.Fprintln(errOut, err); return 2 }

	cfgPath := filepath.Join(root, config.DefaultFilename)
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		if err := config.WriteDefault(cfgPath); err != nil { fmt.Fprintln(errOut, err); return 2 }
	}

	hook, err := installHook(root)
	if err != nil { fmt.Fprintln(errOut, err); return 2 }
	fmt.Fprintf(out, "Installed pre-commit preset: %s\nHook: %s\nConfig: %s\n", selected, hook, filepath.Join(root, precommit.ConfigFilename))
	return 0
}

func help(out io.Writer) {
	fmt.Fprintln(out, `git-cli precommit - application-aware pre-commit setup

Usage:
  git-cli precommit --setup --for python
  git-cli precommit --setup --for fastapi
  git-cli precommit --setup --for django
  git-cli precommit --setup --for laravel
  git-cli precommit --setup --scan
  git-cli precommit run
  git-cli precommit status
  git-cli precommit list
  git-cli precommit uninstall`)
}

func hooksDir(root string) (string, error) {
	cmd := exec.Command("git", "config", "--get", "core.hooksPath")
	cmd.Dir = root
	b, err := cmd.Output()
	if err == nil {
		v := strings.TrimSpace(string(b))
		if v != "" {
			if filepath.IsAbs(v) { return filepath.Clean(v), nil }
			return filepath.Join(root, v), nil
		}
	}
	gitDir, err := gitutil.GitDir(root)
	if err != nil { return "", err }
	return filepath.Join(gitDir, "hooks"), nil
}

func installHook(root string) (string, error) {
	dir, err := hooksDir(root)
	if err != nil { return "", err }
	if err := os.MkdirAll(dir, 0o755); err != nil { return "", err }
	path := filepath.Join(dir, "pre-commit")
	if b, err := os.ReadFile(path); err == nil {
		s := string(b)
		if !strings.Contains(s, managedMarker) && !strings.Contains(s, "git-cli security check-staged") {
			return "", fmt.Errorf("refusing to overwrite existing pre-commit hook: %s", path)
		}
	}
	content := "#!/usr/bin/env bash\nset -e\n\n# " + managedMarker + "\nexec git-cli hook run pre-commit\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil { return "", err }
	return path, nil
}

func uninstall(out, errOut io.Writer) int {
	root, err := gitutil.Root()
	if err != nil { fmt.Fprintln(errOut, err); return 2 }
	dir, err := hooksDir(root)
	if err != nil { fmt.Fprintln(errOut, err); return 2 }
	path := filepath.Join(dir, "pre-commit")
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(filepath.Join(root, precommit.ConfigFilename))
		fmt.Fprintln(out, "No git-cli precommit hook installed.")
		return 0
	}
	if err != nil { fmt.Fprintln(errOut, err); return 2 }
	if !strings.Contains(string(b), managedMarker) { fmt.Fprintln(errOut, "pre-commit hook is not managed by git-cli precommit"); return 2 }
	securityOnly := "#!/usr/bin/env bash\nset -e\n\nexec git-cli security check-staged\n"
	if err := os.WriteFile(path, []byte(securityOnly), 0o755); err != nil { fmt.Fprintln(errOut, err); return 2 }
	_ = os.Remove(filepath.Join(root, precommit.ConfigFilename))
	fmt.Fprintln(out, "Removed application precommit checks; security scanning remains enabled.")
	return 0
}

func status(out, errOut io.Writer) int {
	root, err := gitutil.Root()
	if err != nil { fmt.Fprintln(errOut, err); return 2 }
	dir, err := hooksDir(root)
	if err != nil { fmt.Fprintln(errOut, err); return 2 }
	path := filepath.Join(dir, "pre-commit")
	managed := false
	if b, err := os.ReadFile(path); err == nil { managed = strings.Contains(string(b), managedMarker) }
	fmt.Fprintf(out, "Repository: %s\nHooks path: %s\nManaged hook: %v\n", root, dir, managed)
	cfg, err := precommit.Load(root)
	if err != nil { fmt.Fprintf(out, "Preset: not configured (%s)\n", precommit.ConfigFilename); return 0 }
	fmt.Fprintf(out, "Preset: %s\nConfig: %s\n", cfg.Preset, filepath.Join(root, precommit.ConfigFilename))
	return 0
}

func runStaged(root string, out, errOut io.Writer) error {
	cfg, err := precommit.Load(root)
	if err != nil { return fmt.Errorf("precommit config: %w", err) }
	files, err := gitutil.StagedFiles(root)
	if err != nil { return err }
	fmt.Fprintf(out, "Precommit preset: %s\n", cfg.Preset)
	if len(files) == 0 { fmt.Fprintln(out, "No staged files."); return nil }

	gitDir, err := gitutil.GitDir(root)
	if err != nil { return err }
	tmp, err := os.MkdirTemp(gitDir, "git-cli-staged-")
	if err != nil { return err }
	defer os.RemoveAll(tmp)

	materialized := map[string]string{}
	for _, file := range files {
		cmd := exec.Command("git", "show", ":"+file)
		cmd.Dir = root
		b, err := cmd.Output()
		if err != nil { return fmt.Errorf("read staged %s: %w", file, err) }
		p := filepath.Join(tmp, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { return err }
		if err := os.WriteFile(p, b, 0o644); err != nil { return err }
		materialized[file] = p
	}

	switch cfg.Preset {
	case "python", "fastapi":
		return runPython(root, materialized, out, errOut)
	case "django":
		if err := runPython(root, materialized, out, errOut); err != nil { return err }
		return run(root, out, errOut, pythonCommand(), "manage.py", "check")
	case "laravel":
		return runPHP(root, materialized, out, errOut)
	default:
		return fmt.Errorf("unsupported preset %q", cfg.Preset)
	}
}

func runPython(root string, files map[string]string, out, errOut io.Writer) error {
	var py []string
	for original, staged := range files { if strings.EqualFold(filepath.Ext(original), ".py") { py = append(py, staged) } }
	if len(py) == 0 { fmt.Fprintln(out, "Python: no staged .py files"); return nil }
	if commandExists("ruff") { return run(root, out, errOut, "ruff", append([]string{"check"}, py...)...) }
	return run(root, out, errOut, pythonCommand(), append([]string{"-m", "py_compile"}, py...)...)
}

func runPHP(root string, files map[string]string, out, errOut io.Writer) error {
	if !commandExists("php") { return errors.New("php is required for the laravel precommit preset") }
	found := false
	for original, staged := range files {
		if !strings.EqualFold(filepath.Ext(original), ".php") { continue }
		found = true
		if err := run(root, out, errOut, "php", "-l", staged); err != nil { return err }
	}
	if !found { fmt.Fprintln(out, "Laravel: no staged .php files") }
	return nil
}

func run(root string, out, errOut io.Writer, name string, args ...string) error {
	if !commandExists(name) { return fmt.Errorf("required command not installed: %s", name) }
	fmt.Fprintf(out, "> %s %s\n", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Dir = root
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil { return fmt.Errorf("%s failed: %w", name, err) }
	return nil
}

func pythonCommand() string { if commandExists("python3") { return "python3" }; return "python" }
func commandExists(name string) bool { _, err := exec.LookPath(name); return err == nil }
