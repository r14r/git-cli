package branchcmd

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	gitutil "git-cli/internal/git"
)

func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprintln(out, `git-cli branch - branch inspection

Usage:
  git-cli branch merged
  git-cli branch stale`)
		return 0
	}
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	switch args[0] {
	case "merged":
		return merged(root, out, errOut)
	case "stale":
		return stale(root, out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown branch command: %s\n", args[0])
		return 2
	}
}

func merged(root string, out, errOut io.Writer) int {
	cmd := exec.Command("git", "branch", "--merged", "HEAD")
	cmd.Dir = root
	b, err := cmd.Output()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	current := currentBranch(root)
	count := 0
	for _, line := range strings.Split(string(b), "\n") {
		name := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if name == "" || name == current || name == "main" || name == "master" {
			continue
		}
		fmt.Fprintln(out, name)
		count++
	}
	if count == 0 {
		fmt.Fprintln(out, "No merged local branches eligible for cleanup.")
	}
	return 0
}

func stale(root string, out, errOut io.Writer) int {
	cmd := exec.Command("git", "for-each-ref", "--sort=committerdate", "--format=%(committerdate:short) %(refname:short)", "refs/heads")
	cmd.Dir = root
	b, err := cmd.Output()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprint(out, string(b))
	return 0
}

func currentBranch(root string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = root
	b, _ := cmd.Output()
	return strings.TrimSpace(string(b))
}
