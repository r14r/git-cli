package actionscmd

import (
	"bytes"
	"testing"
)

func TestNormalizeRepository(t *testing.T) {
	cases := map[string]string{
		"r14r/git-cli":                    "r14r/git-cli",
		"https://github.com/r14r/git-cli": "r14r/git-cli",
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
