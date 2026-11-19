package doctor

import (
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

func Run(out, errOut io.Writer) int { return RunArgs(nil, out, errOut) }

func RunArgs(args []string, out, errOut io.Writer) int {
	fix := false
	if len(args) == 1 && args[0] == "--fix" {
		fix = true
	} else if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Fprintln(out, "Usage: git-cli doctor [--fix]")
		return 0
	} else if len(args) != 0 {
		fmt.Fprintln(errOut, "Usage: git-cli doctor [--fix]")
		return 2
	}
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(errOut, "✗ Git repository:", err)
		return 2
	}
	fmt.Fprintln(out, "Git CLI Doctor")
	fmt.Fprintf(out, "Repository: %s\n\n", root)
	problems := 0
	if fix {
		applyFixes(root, out, errOut)
		fmt.Fprintln(out, "\nRechecking repository...")
	}

	fmt.Fprintln(out, "Repository")
	fmt.Fprintln(out, "  ✓ Git repository")
	hooks, err := hooksDir(root)
	if err != nil {
		fmt.Fprintf(out, "  ✗ Hooks path: %v\n", err)
		problems++
	} else {
		fmt.Fprintf(out, "  ✓ Hooks path: %s\n", hooks)
	}
	if hooks != "" {
		b, err := os.ReadFile(filepath.Join(hooks, "pre-commit"))
		if err == nil && (strings.Contains(string(b), "git-cli hook run pre-commit") || precommit.ManagedHook(string(b))) {
			fmt.Fprintln(out, "  ✓ git-cli pre-commit hook")
		} else {
			fmt.Fprintln(out, "  ! git-cli pre-commit hook not installed\n    Fix: git-cli doctor --fix (existing custom hooks are preserved)")
		}
	}

	fmt.Fprintln(out, "\nSecurity")
	for _, tool := range []struct {
		name     string
		required bool
	}{{"gitleaks", true}, {"detect-secrets", true}, {"trufflehog", false}} {
		_, err := exec.LookPath(tool.name)
		if err == nil {
			fmt.Fprintf(out, "  ✓ %s\n", tool.name)
			continue
		}
		if tool.required {
			fmt.Fprintf(out, "  ✗ %s missing\n    Fix: %s\n", tool.name, suggestion(tool.name))
			problems++
		} else {
			fmt.Fprintf(out, "  ! %s optional / missing\n", tool.name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, config.DefaultFilename)); err == nil {
		fmt.Fprintf(out, "  ✓ %s\n", config.DefaultFilename)
	} else {
		fmt.Fprintf(out, "  ! %s not configured\n    Fix: git-cli doctor --fix\n", config.DefaultFilename)
	}

	fmt.Fprintln(out, "\nApplication")
	preset, err := precommit.Detect(root)
	if err != nil {
		fmt.Fprintf(out, "  ! Project type not detected: %v\n", err)
	} else {
		fmt.Fprintf(out, "  ✓ Type: %s\n", preset)
		for _, tool := range toolsFor(preset) {
			if _, err := exec.LookPath(tool); err == nil {
				fmt.Fprintf(out, "  ✓ %s\n", tool)
			} else {
				fmt.Fprintf(out, "  ✗ %s missing\n    Fix: %s\n", tool, suggestion(tool))
				problems++
			}
		}
	}
	if cfg, err := precommit.Load(root); err == nil {
		fmt.Fprintf(out, "  ✓ %s (preset=%s)\n", precommit.ConfigFilename, cfg.Preset)
	} else {
		fmt.Fprintf(out, "  ! %s not configured\n    Fix: git-cli doctor --fix (if a supported preset is detected)\n", precommit.ConfigFilename)
	}

	fmt.Fprintln(out)
	if problems > 0 {
		fmt.Fprintf(out, "Result: %d problem(s)\n", problems)
		return 1
	}
	fmt.Fprintln(out, "Result: OK")
	return 0
}

func hooksDir(root string) (string, error) {
	cmd := exec.Command("git", "config", "--get", "core.hooksPath")
	cmd.Dir = root
	if b, err := cmd.Output(); err == nil {
		v := strings.TrimSpace(string(b))
		if v != "" {
			if filepath.IsAbs(v) {
				return filepath.Clean(v), nil
			}
			return filepath.Join(root, v), nil
		}
	}
	gitDir, err := gitutil.GitDir(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(gitDir, "hooks"), nil
}

func toolsFor(preset string) []string {
	switch preset {
	case "python", "fastapi":
		if _, err := exec.LookPath("python3"); err == nil {
			return []string{"python3"}
		}
		return []string{"python"}
	case "django":
		if _, err := exec.LookPath("python3"); err == nil {
			return []string{"python3"}
		}
		return []string{"python"}
	case "laravel":
		return []string{"php"}
	default:
		return nil
	}
}
