package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func Root() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return filepath.Clean(strings.TrimSpace(out.String())), nil
}

func StagedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--staged", "--name-only", "--diff-filter=ACMR")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}

func GitDir(root string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(string(out))
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	return filepath.Join(root, p), nil
}
