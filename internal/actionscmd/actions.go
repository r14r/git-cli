package actionscmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"git-cli/internal/patcmd"
)

const apiVersion = "2026-03-10"

var validStates = map[string]bool{
	"completed":       true,
	"action_required": true,
	"cancelled":       true,
	"failure":         true,
	"neutral":         true,
	"skipped":         true,
	"stale":           true,
	"success":         true,
	"timed_out":       true,
	"in_progress":     true,
	"queued":          true,
	"requested":       true,
	"waiting":         true,
	"pending":         true,
}

type workflowRunsResponse struct {
	TotalCount   int           `json:"total_count"`
	WorkflowRuns []workflowRun `json:"workflow_runs"`
}

type workflowRun struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	DisplayTitle string    `json:"display_title"`
	Event        string    `json:"event"`
	Status       string    `json:"status"`
	Conclusion   string    `json:"conclusion"`
	WorkflowID   int64     `json:"workflow_id"`
	RunNumber    int64     `json:"run_number"`
	RunAttempt   int64     `json:"run_attempt"`
	HeadBranch   string    `json:"head_branch"`
	HeadSHA      string    `json:"head_sha"`
	HTMLURL      string    `json:"html_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Actor        struct {
		Login string `json:"login"`
	} `json:"actor"`
}

type options struct {
	repository string
	state      string
	verbose    bool
	details    bool
	patName    string
}

func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		help(out)
		return 0
	}
	switch args[0] {
	case "list":
		return list(args[1:], out, errOut)
	case "delete":
		return deleteRuns(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown actions command: %s\n", args[0])
		help(errOut)
		return 2
	}
}

func help(w io.Writer) {
	io.WriteString(w, `git-cli actions - manage GitHub Actions workflow runs

Usage:
  git-cli actions list --repository OWNER/REPO [--state STATE] [--verbose] [--details] [--pat NAME]
  git-cli actions delete --repository OWNER/REPO --state STATE [--verbose] [--details] [--pat NAME]

Supported --state values:
  completed, action_required, cancelled, failure, neutral, skipped, stale,
  success, timed_out, in_progress, queued, requested, waiting, pending

Authentication:
  1. GITHUB_TOKEN environment variable
  2. --pat NAME from git-cli encrypted PAT storage
  3. automatically use the PAT if exactly one PAT is stored

list can access public repositories without authentication.
delete requires a token with GitHub Actions write permission.
`)
}

func parseOptions(command string, args []string, requireState bool, errOut io.Writer) (options, int) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	var o options
	fs.StringVar(&o.repository, "repository", "", "GitHub repository as OWNER/REPO or GitHub URL")
	fs.StringVar(&o.state, "state", "", "workflow run status or conclusion")
	fs.BoolVar(&o.verbose, "verbose", false, "show workflow name and state")
	fs.BoolVar(&o.details, "details", false, "show full workflow run details")
	fs.StringVar(&o.patName, "pat", "", "PAT name stored by git-cli")
	if err := fs.Parse(args); err != nil {
		return o, 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(errOut, "unexpected positional arguments")
		return o, 2
	}
	repo, err := normalizeRepository(o.repository)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return o, 2
	}
	o.repository = repo
	o.state = strings.TrimSpace(o.state)
	if o.state != "" && !validStates[o.state] {
		fmt.Fprintf(errOut, "invalid --state %q\n", o.state)
		fmt.Fprintln(errOut, "use: completed, action_required, cancelled, failure, neutral, skipped, stale, success, timed_out, in_progress, queued, requested, waiting, pending")
		return o, 2
	}
	if requireState && o.state == "" {
		fmt.Fprintln(errOut, "--state is required for actions delete")
		return o, 2
	}
	return o, 0
}

func normalizeRepository(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("--repository is required")
	}
	value = strings.TrimSuffix(value, ".git")
	if strings.HasPrefix(value, "https://github.com/") || strings.HasPrefix(value, "http://github.com/") {
		u, err := url.Parse(value)
		if err != nil {
			return "", err
		}
		value = strings.Trim(u.Path, "/")
	}
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid repository %q; expected OWNER/REPO", value)
	}
	return parts[0] + "/" + parts[1], nil
}

func authToken(patName string) (string, string, error) {
	if patName != "" {
		token, err := patcmd.ResolveToken(patName)
		if err != nil {
			return "", "", err
		}
		return token, "pat:" + patName, nil
	}
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token, "GITHUB_TOKEN", nil
	}
	names, err := patcmd.TokenNames()
	if err != nil {
		return "", "", err
	}
	if len(names) == 1 {
		token, err := patcmd.ResolveToken(names[0])
		if err != nil {
			return "", "", err
		}
		return token, "pat:" + names[0], nil
	}
	if len(names) > 1 {
		return "", "", errors.New("multiple PATs are stored; select one with --pat NAME or set GITHUB_TOKEN")
	}
	return "", "anonymous", nil
}

func list(args []string, out, errOut io.Writer) int {
	o, code := parseOptions("actions list", args, false, errOut)
	if code != 0 {
		return code
	}
	token, authSource, err := authToken(o.patName)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if o.details {
		fmt.Fprintf(out, "Repository: %s\nAuthentication: %s\n\n", o.repository, authSource)
	}
	runs, err := fetchRuns(o.repository, o.state, token)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if len(runs) == 0 {
		fmt.Fprintln(out, "No workflow runs found.")
		return 0
	}
	for _, run := range runs {
		printRun(out, run, o.verbose, o.details)
	}
	fmt.Fprintf(out, "Runs: %d\n", len(runs))
	return 0
}

func deleteRuns(args []string, out, errOut io.Writer) int {
	o, code := parseOptions("actions delete", args, true, errOut)
	if code != 0 {
		return code
	}
	token, authSource, err := authToken(o.patName)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if o.details {
		fmt.Fprintf(out, "Repository: %s\nAuthentication: %s\n\n", o.repository, authSource)
	}
	if token == "" {
		fmt.Fprintln(errOut, "authentication is required for actions delete; set GITHUB_TOKEN or use --pat NAME")
		return 2
	}
	runs, err := fetchRuns(o.repository, o.state, token)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if len(runs) == 0 {
		fmt.Fprintln(out, "No workflow runs matched.")
		return 0
	}
	deleted := 0
	for _, run := range runs {
		if err := deleteRun(o.repository, run.ID, token); err != nil {
			fmt.Fprintf(errOut, "delete run %d: %v\n", run.ID, err)
			return 2
		}
		deleted++
		if o.details {
			fmt.Fprintf(out, "Deleted run %d (%s, %s, %s)\n", run.ID, run.Name, run.HeadBranch, effectiveState(run))
		} else if o.verbose {
			fmt.Fprintf(out, "%s\t%s\n", run.Name, effectiveState(run))
		}
	}
	fmt.Fprintf(out, "Deleted %d workflow run(s) with state %q from %s.\n", deleted, o.state, o.repository)
	return 0
}

func fetchRuns(repository, state, token string) ([]workflowRun, error) {
	var all []workflowRun
	for page := 1; ; page++ {
		u := fmt.Sprintf("https://api.github.com/repos/%s/actions/runs?per_page=100&page=%d", repository, page)
		if state != "" {
			u += "&status=" + url.QueryEscape(state)
		}
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		setHeaders(req, token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode != http.StatusOK {
			return nil, githubHTTPError("GitHub Actions list", resp.StatusCode, body, true)
		}
		var doc workflowRunsResponse
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, err
		}
		all = append(all, doc.WorkflowRuns...)
		if len(doc.WorkflowRuns) < 100 || len(all) >= 1000 {
			break
		}
	}
	return all, nil
}

func deleteRun(repository string, runID int64, token string) error {
	u := fmt.Sprintf("https://api.github.com/repos/%s/actions/runs/%d", repository, runID)
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	setHeaders(req, token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return githubHTTPError("GitHub Actions delete", resp.StatusCode, body, false)
	}
	return nil
}

func githubHTTPError(operation string, status int, body []byte, readOnly bool) error {
	var apiErr struct {
		Message          string `json:"message"`
		DocumentationURL string `json:"documentation_url"`
	}
	_ = json.Unmarshal(body, &apiErr)
	msg := strings.TrimSpace(apiErr.Message)
	if msg == "" {
		msg = http.StatusText(status)
	}

	var hint string
	if status == http.StatusForbidden || status == http.StatusUnauthorized {
		if readOnly {
			hint = "private repositories require a token with Actions: read (fine-grained PAT) or repo scope (classic PAT)"
		} else {
			hint = "deleting workflow runs requires Actions: write (fine-grained PAT) or repo scope with write access (classic PAT)"
		}
	}
	if hint != "" {
		return fmt.Errorf("%s failed: HTTP %d: %s; %s", operation, status, msg, hint)
	}
	return fmt.Errorf("%s failed: HTTP %d: %s", operation, status, msg)
}

func setHeaders(req *http.Request, token string) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	req.Header.Set("User-Agent", "git-cli")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func effectiveState(run workflowRun) string {
	if run.Conclusion != "" {
		return run.Conclusion
	}
	return run.Status
}

func printRun(w io.Writer, run workflowRun, verbose, details bool) {
	if details {
		fmt.Fprintf(w, "Run %d\n", run.ID)
		fmt.Fprintf(w, "  Workflow:   %s\n", run.Name)
		if run.DisplayTitle != "" && run.DisplayTitle != run.Name {
			fmt.Fprintf(w, "  Title:      %s\n", run.DisplayTitle)
		}
		fmt.Fprintf(w, "  State:      %s\n", effectiveState(run))
		fmt.Fprintf(w, "  Status:     %s\n", run.Status)
		fmt.Fprintf(w, "  Conclusion: %s\n", run.Conclusion)
		fmt.Fprintf(w, "  Branch:     %s\n", run.HeadBranch)
		fmt.Fprintf(w, "  Event:      %s\n", run.Event)
		fmt.Fprintf(w, "  Commit:     %s\n", run.HeadSHA)
		fmt.Fprintf(w, "  Run:        %s attempt %s\n", strconv.FormatInt(run.RunNumber, 10), strconv.FormatInt(run.RunAttempt, 10))
		fmt.Fprintf(w, "  Actor:      %s\n", run.Actor.Login)
		fmt.Fprintf(w, "  Created:    %s\n", run.CreatedAt.Format(time.RFC3339))
		fmt.Fprintf(w, "  Updated:    %s\n", run.UpdatedAt.Format(time.RFC3339))
		fmt.Fprintf(w, "  URL:        %s\n\n", run.HTMLURL)
		return
	}
	if verbose {
		fmt.Fprintf(w, "%s\t%s\n", run.Name, effectiveState(run))
		return
	}
	fmt.Fprintf(w, "%d\t%-12s\t%-20s\t%s\n", run.ID, effectiveState(run), run.HeadBranch, run.Name)
	return
}
