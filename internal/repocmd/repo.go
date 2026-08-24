package repocmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gitutil "git-cli/internal/git"
)

func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] != "health" {
		fmt.Fprintln(errOut, "usage: git-cli repo health")
		return 2
	}
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	problems := 0
	fmt.Fprintln(out, "Git Repository Health")
	fmt.Fprintf(out, "Repository: %s\n\n", root)

	branch := gitOutput(root, "branch", "--show-current")
	if branch == "" {
		fmt.Fprintln(out, "! branch: detached HEAD")
		problems++
	} else {
		fmt.Fprintf(out, "✓ branch: %s\n", branch)
	}

	if gitOutput(root, "status", "--porcelain") == "" {
		fmt.Fprintln(out, "✓ working tree clean")
	} else {
		fmt.Fprintln(out, "! working tree has changes")
		problems++
	}

	upstream := gitOutput(root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if upstream == "" {
		fmt.Fprintln(out, "! no upstream configured")
		problems++
	} else {
		fmt.Fprintf(out, "✓ upstream: %s\n", upstream)
		ahead := gitOutput(root, "rev-list", "--count", upstream+"..HEAD")
		behind := gitOutput(root, "rev-list", "--count", "HEAD.."+upstream)
		fmt.Fprintf(out, "  ahead=%s behind=%s\n", defaultZero(ahead), defaultZero(behind))
	}

	if _, err := os.Stat(filepath.Join(root, ".gitignore")); err == nil {
		fmt.Fprintln(out, "✓ .gitignore present")
	} else {
		fmt.Fprintln(out, "! .gitignore missing")
		problems++
	}

	ignoredTracked := gitOutput(root, "ls-files", "-ci", "--exclude-standard")
	if ignoredTracked == "" {
		fmt.Fprintln(out, "✓ no tracked files match ignore rules")
	} else {
		count := len(strings.Split(strings.TrimSpace(ignoredTracked), "\n"))
		fmt.Fprintf(out, "! %d tracked file(s) match ignore rules\n", count)
		problems++
	}

	fmt.Fprintln(out)
	if problems == 0 {
		fmt.Fprintln(out, "Result: OK")
		return 0
	}
	fmt.Fprintf(out, "Result: %d issue(s)\n", problems)
	return 1
}

func gitOutput(root string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	b, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func defaultZero(v string) string {
	if v == "" {
		return "0"
	}
	return v
}
