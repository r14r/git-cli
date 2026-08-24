package gitignorediag

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"

	gitutil "git-cli/internal/git"
)

func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "gitignore diagnostics require explain <path...> or tracked")
		return 2
	}
	switch args[0] {
	case "explain":
		return explain(args[1:], out, errOut)
	case "tracked":
		return tracked(out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown gitignore diagnostic: %s\n", args[0])
		return 2
	}
}

func explain(paths []string, out, errOut io.Writer) int {
	if len(paths) == 0 {
		fmt.Fprintln(errOut, "usage: git-cli gitignore explain <path> [path...]")
		return 2
	}
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	code := 0
	for _, path := range paths {
		cmd := exec.Command("git", "check-ignore", "-v", "--no-index", "--", path)
		cmd.Dir = root
		b, err := cmd.CombinedOutput()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
				fmt.Fprintf(out, "%s: not ignored\n", path)
				continue
			}
			fmt.Fprintf(errOut, "%s: %v: %s\n", path, err, strings.TrimSpace(string(b)))
			code = 2
			continue
		}
		fmt.Fprintf(out, "%s: %s\n", path, strings.TrimSpace(string(b)))
	}
	return code
}

func tracked(out, errOut io.Writer) int {
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	cmd := exec.Command("git", "ls-files", "-ci", "--exclude-standard")
	cmd.Dir = root
	b, err := cmd.Output()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	var files []string
	s := bufio.NewScanner(strings.NewReader(string(b)))
	for s.Scan() {
		if v := strings.TrimSpace(s.Text()); v != "" {
			files = append(files, v)
		}
	}
	if err := s.Err(); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if len(files) == 0 {
		fmt.Fprintln(out, "No tracked files match ignore rules.")
		return 0
	}
	fmt.Fprintln(out, "Tracked files matching ignore rules:")
	for _, file := range files {
		fmt.Fprintf(out, "  %s\n", file)
	}
	fmt.Fprintf(out, "\n%d file(s). To stop tracking one safely, review then run: git rm --cached <path>\n", len(files))
	return 1
}
