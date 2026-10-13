package actionscmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestNormalizeRepository(t *testing.T) {
	cases := map[string]string{
		"r14r/git-cli":                        "r14r/git-cli",
		"https://github.com/r14r/git-cli":     "r14r/git-cli",
		"https://github.com/r14r/git-cli.git": "r14r/git-cli",
	}
	for input, want := range cases {
		got, err := normalizeRepository(input)
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if got != want {
			t.Fatalf("%s => %q, want %q", input, got, want)
		}
	}
}

func TestInvalidState(t *testing.T) {
	var errOut bytes.Buffer
	_, code := parseOptions("actions list", []string{"--repository", "r14r/git-cli", "--state", "banana"}, false, &errOut)
	if code != 2 {
		t.Fatalf("code=%d err=%q", code, errOut.String())
	}
}

func TestDeleteRequiresState(t *testing.T) {
	var errOut bytes.Buffer
	_, code := parseOptions("actions delete", []string{"--repository", "r14r/git-cli"}, true, &errOut)
	if code != 2 {
		t.Fatalf("code=%d err=%q", code, errOut.String())
	}
}

func TestEffectiveState(t *testing.T) {
	if got := effectiveState(workflowRun{Status: "completed", Conclusion: "failure"}); got != "failure" {
		t.Fatalf("got %q", got)
	}
	if got := effectiveState(workflowRun{Status: "in_progress"}); got != "in_progress" {
		t.Fatalf("got %q", got)
	}
}

func TestVerbosePrintsOnlyWorkflowAndState(t *testing.T) {
	var out bytes.Buffer
	printRun(&out, workflowRun{Name: "CI", Status: "completed", Conclusion: "success", HeadBranch: "main"}, true, false)
	if got, want := out.String(), "CI\tsuccess\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestDetailsPrintsFullInformation(t *testing.T) {
	var out bytes.Buffer
	run := workflowRun{
		ID:         42,
		Name:       "CI",
		Status:     "completed",
		Conclusion: "success",
		HeadBranch: "main",
		HeadSHA:    "abc123",
		HTMLURL:    "https://github.com/example/repo/actions/runs/42",
	}
	printRun(&out, run, false, true)
	got := out.String()
	for _, want := range []string{"Run 42", "Workflow:   CI", "State:      success", "Branch:     main", "Commit:     abc123", "URL:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("details output missing %q: %q", want, got)
		}
	}
}
