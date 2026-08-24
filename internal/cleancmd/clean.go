package cleancmd

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	gitutil "git-cli/internal/git"
)

func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprintln(out, `git-cli clean - safe repository cleanup

Usage:
  git-cli clean preview
  git-cli clean ignored --preview
  git-cli clean ignored --apply`)
		return 0
	}
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	switch args[0] {
	case "preview":
		return run(root, []string{"clean", "-nd"}, out, errOut)
	case "ignored":
		apply := len(args) > 1 && args[1] == "--apply"
		preview := len(args) == 1 || (len(args) > 1 && args[1] == "--preview")
		if !apply && !preview {
			fmt.Fprintln(errOut, "use --preview or --apply")
			return 2
		}
		if apply {
			return run(root, []string{"clean", "-fdX"}, out, errOut)
		}
		return run(root, []string{"clean", "-ndX"}, out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown clean command: %s\n", args[0])
		return 2
	}
}

func run(root string, args []string, out, errOut io.Writer) int {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	b, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(b))
	if text == "" {
		fmt.Fprintln(out, "Nothing to clean.")
	} else {
		fmt.Fprintln(out, text)
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	return 0
}
