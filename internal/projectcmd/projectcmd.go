package projectcmd

import (
	"encoding/json"
	"fmt"
	"io"

	gitutil "git-cli/internal/git"
	"git-cli/internal/precommit"
)

func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(out, `git-cli project - project inspection

Usage:
  git-cli project detect [--json]
  git-cli project info [--json]`)
		return 0
	}
	root, err := gitutil.Root()
	if err != nil { fmt.Fprintln(errOut, err); return 2 }
	preset, detectErr := precommit.Detect(root)
	jsonOut := len(args) > 1 && args[1] == "--json"
	switch args[0] {
	case "detect", "info":
		if detectErr != nil {
			if jsonOut {
				b, _ := json.MarshalIndent(map[string]any{"repository": root, "detected": false, "error": detectErr.Error()}, "", "  ")
				fmt.Fprintln(out, string(b))
			} else {
				fmt.Fprintln(errOut, detectErr)
			}
			return 1
		}
		if jsonOut {
			b, _ := json.MarshalIndent(map[string]any{"repository": root, "detected": true, "preset": preset}, "", "  ")
			fmt.Fprintln(out, string(b))
		} else {
			fmt.Fprintf(out, "Repository: %s\nDetected project: %s\n", root, preset)
		}
		return 0
	default:
		fmt.Fprintf(errOut, "unknown project command: %s\n", args[0])
		return 2
	}
}
