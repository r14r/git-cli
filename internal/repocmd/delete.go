package repocmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const defaultOwner = "r14r"

var (
	repoPart       = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	httpStatusLine = regexp.MustCompile(`^HTTP/\S+\s+([0-9]{3})(?:\s|$)`)
)

type deleteClient interface {
	check(repo string) (int, error)
	delete(repo string, yes bool, out, errOut io.Writer) error
}

type ghDeleteClient struct{}

func (ghDeleteClient) check(repo string) (int, error) {
	// GitHub can return 404 for a private repository that is not accessible.
	output, err := exec.Command("gh", "api", "--include", "repos/"+repo).CombinedOutput()
	status := parseHTTPStatus(output)
	if status == 0 {
		return 0, fmt.Errorf("gh api: %s", strings.TrimSpace(string(output)))
	}
	return status, err
}

func (ghDeleteClient) delete(repo string, yes bool, out, errOut io.Writer) error {
	args := []string{"repo", "delete", repo}
	if yes {
		args = append(args, "--yes")
	}
	cmd := exec.Command("gh", args...)
	if yes {
		output, err := cmd.CombinedOutput()
		if err != nil {
			if msg := strings.TrimSpace(string(output)); msg != "" {
				fmt.Fprintln(errOut, msg)
			}
			return err
		}
		return nil
	}
	// Without --yes, keep GitHub CLI's interactive confirmation.
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
}

func parseHTTPStatus(data []byte) int {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		match := httpStatusLine.FindStringSubmatch(strings.TrimSpace(scanner.Text()))
		if len(match) == 2 {
			var status int
			if _, err := fmt.Sscanf(match[1], "%d", &status); err == nil {
				return status
			}
		}
	}
	return 0
}

func normalizeRepo(input string) (string, error) {
	if input == "" || strings.TrimSpace(input) != input || strings.Count(input, "/") > 1 {
		return "", fmt.Errorf("invalid repository %q (expected repo or owner/repo)", input)
	}
	owner, name := defaultOwner, input
	if strings.Contains(input, "/") {
		parts := strings.SplitN(input, "/", 2)
		owner, name = parts[0], parts[1]
	}
	for _, part := range []string{owner, name} {
		if !repoPart.MatchString(part) || part == "." || part == ".." {
			return "", fmt.Errorf("invalid repository %q (expected repo or owner/repo)", input)
		}
	}
	return owner + "/" + name, nil
}

func deleteHelp(out io.Writer) {
	fmt.Fprintln(out, `Usage: git-cli repo delete <repo> [--yes]

Delete a GitHub repository and check the result.
A short name such as "test" resolves to "r14r/test".

Options:
  --yes       Skip GitHub CLI's interactive confirmation
  -h, --help  Show this help

Examples:
  git-cli repo delete test
  git-cli repo delete test --yes
  git-cli repo delete r14r/test --yes

Requires GitHub CLI authorization (gh auth login; gh auth refresh -s delete_repo).
WARNING: Deleting a repository removes its GitHub-hosted data.`)
}

func runDelete(args []string, out, errOut io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		deleteHelp(out)
		return 0
	}
	if _, err := exec.LookPath("gh"); err != nil {
		fmt.Fprintln(errOut, "GitHub CLI (gh) is not installed or not in PATH")
		return 2
	}
	return runDeleteWithClient(args, out, errOut, ghDeleteClient{})
}

func statusLine(out io.Writer, step int, label, status string) {
	fmt.Fprintf(out, "%d. %-18s %s\n", step, label, status)
}

func runDeleteWithClient(args []string, out, errOut io.Writer, client deleteClient) int {
	var input string
	yes := false
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			deleteHelp(out)
			return 0
		case "--yes":
			yes = true
		default:
			if strings.HasPrefix(arg, "-") || input != "" {
				fmt.Fprintln(errOut, "usage: git-cli repo delete <repo> [--yes]")
				return 2
			}
			input = arg
		}
	}
	if input == "" {
		fmt.Fprintln(errOut, "usage: git-cli repo delete <repo> [--yes]")
		return 2
	}
	repo, err := normalizeRepo(input)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}

	fmt.Fprintf(out, "Repository: %s\n\n", repo)

	before, checkErr := client.check(repo)
	switch {
	case before == 200 && checkErr == nil:
		statusLine(out, 1, "Checking Repo:", "EXISTS")
	case before == 404:
		statusLine(out, 1, "Checking Repo:", "NOT FOUND (or inaccessible)")
		statusLine(out, 2, "Deleting Repo:", "SKIPPED")
		statusLine(out, 3, "Checking Repo:", "NOT FOUND (or inaccessible)")
		return 1
	default:
		statusLine(out, 1, "Checking Repo:", "ERROR")
		if before > 0 {
			fmt.Fprintf(errOut, "Cannot verify repository (HTTP %d); deletion aborted.\n", before)
		} else {
			fmt.Fprintln(errOut, "Cannot verify repository (API or authentication error); deletion aborted.")
		}
		return 1
	}

	statusLine(out, 2, "Deleting Repo:", "DELETING")
	deleteErr := client.delete(repo, yes, out, errOut)
	if deleteErr != nil {
		statusLine(out, 2, "Deleting Repo:", "FAILED")
	}

	after, checkErr := client.check(repo)
	switch {
	case after == 404:
		if deleteErr == nil {
			statusLine(out, 3, "Checking Repo:", "DELETED")
			return 0
		}
		statusLine(out, 3, "Checking Repo:", "NOT FOUND (or inaccessible)")
	case after == 200 && checkErr == nil:
		statusLine(out, 3, "Checking Repo:", "STILL EXISTS")
	default:
		statusLine(out, 3, "Checking Repo:", "ERROR")
	}
	if deleteErr == nil {
		fmt.Fprintln(errOut, "Deletion could not be verified.")
	}
	return 1
}
