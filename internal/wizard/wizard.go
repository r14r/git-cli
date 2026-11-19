package wizard

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func Run(in io.Reader, out, errOut io.Writer) int {
	items := []struct {
		label string
		args []string
	}{
		{"Repository health", []string{"repo", "health"}},
		{"Doctor", []string{"doctor"}},
		{"Doctor: safe fixes", []string{"doctor", "--fix"}},
		{"Branch cleanup preview", []string{"branch", "--clean", "--dry-run"}},
		{"Branch cleanup (confirm)", []string{"branch", "--clean"}},
		{"Project information", []string{"project", "info"}},
		{"Security status", []string{"security", "status"}},
	}
	reader := bufio.NewReader(in)
	for {
		fmt.Fprintln(out, "\nGit CLI Wizard")
		for i, item := range items {
			fmt.Fprintf(out, "%d. %s\n", i+1, item.label)
		}
		fmt.Fprint(out, "0. Exit\nSelect: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return 0
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || n < 0 || n > len(items) {
			fmt.Fprintln(errOut, "Invalid selection")
			continue
		}
		if n == 0 {
			return 0
		}
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 2
		}
		cmd := exec.Command(exe, items[n-1].args...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, out, errOut
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		return 0
	}
}
