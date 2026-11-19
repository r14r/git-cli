package doctor

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"git-cli/internal/config"
	"git-cli/internal/precommit"
)

func suggestion(tool string) string {
	switch tool {
	case "detect-secrets":
		return "pipx install detect-secrets"
	case "gitleaks":
		return "brew install gitleaks (macOS) or install from github.com/gitleaks/gitleaks/releases (Linux/Windows)"
	case "python", "python3":
		return "install Python 3 using your OS package manager"
	case "php":
		return "install PHP using your OS package manager"
	default:
		return "install " + tool + " using your OS package manager"
	}
}

func createOnly(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func applyFixes(root string, out, errOut io.Writer) {
	fmt.Fprintln(out, "Applying safe fixes")
	cfg := filepath.Join(root, config.DefaultFilename)
	if _, err := os.Lstat(cfg); errors.Is(err, os.ErrNotExist) {
		// Generate defaults in a temporary directory, then create exclusively.
		dir, e := os.MkdirTemp("", "git-cli-doctor-")
		if e == nil {
			defer os.RemoveAll(dir)
			temp := filepath.Join(dir, config.DefaultFilename)
			e = config.WriteDefault(temp)
			if e == nil {
				var data []byte
				data, e = os.ReadFile(temp)
				if e == nil {
					e = createOnly(cfg, data, 0644)
				}
			}
		}
		if e != nil {
			fmt.Fprintln(errOut, "FAILED", cfg, e)
		} else {
			fmt.Fprintln(out, "FIXED", cfg)
		}
	}
	preset, detectErr := precommit.Detect(root)
	pc := filepath.Join(root, precommit.ConfigFilename)
	if loaded, err := precommit.Load(root); err == nil {
		preset, detectErr = loaded.Preset, nil
	} else if _, statErr := os.Lstat(pc); !errors.Is(statErr, os.ErrNotExist) {
		fmt.Fprintln(errOut, "MANUAL: existing pre-commit configuration needs review")
		return
	} else if detectErr == nil {
		if err := createOnly(pc, []byte("preset: "+preset+"\n"), 0644); err != nil {
			fmt.Fprintln(errOut, "FAILED", pc, err)
			return
		}
		fmt.Fprintln(out, "FIXED", pc)
	}
	hooks, err := hooksDir(root)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return
	}
	// A custom core.hooksPath can be shared by multiple repositories.
	cmd := exec.Command("git", "config", "--get", "core.hooksPath")
	cmd.Dir = root
	if b, e := cmd.Output(); e == nil && len(b) > 0 {
		fmt.Fprintln(out, "MANUAL: custom hooksPath; no hook modified")
		return
	}
	path := filepath.Join(hooks, "pre-commit")
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		content := "#!/usr/bin/env bash\nset -e\nexec git-cli security check-staged\n"
		if detectErr == nil {
			content = precommit.HookContent()
		}
		if err := os.MkdirAll(hooks, 0755); err == nil {
			err = createOnly(path, []byte(content), 0755)
			if err == nil {
				fmt.Fprintln(out, "FIXED", path)
			} else {
				fmt.Fprintln(errOut, "FAILED", path, err)
			}
		} else {
			fmt.Fprintln(errOut, "FAILED", hooks, err)
		}
	}
	fmt.Fprintln(out, "External packages and existing hooks/configuration require manual review.")
}
