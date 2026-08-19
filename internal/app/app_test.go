package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestTopLevelRequiresSecurityNamespace(t *testing.T) {
	var out, errOut bytes.Buffer
	a := &App{Out: &out, Err: &errOut}

	code := a.Run([]string{"scanner", "list"})
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(errOut.String(), "unknown command: scanner") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}

func TestSecurityScannerList(t *testing.T) {
	var out, errOut bytes.Buffer
	a := &App{Out: &out, Err: &errOut}

	code := a.Run([]string{"security", "scanner", "list"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, errOut.String())
	}
	for _, name := range []string{"gitleaks", "detect-secrets", "trufflehog"} {
		if !strings.Contains(out.String(), name) {
			t.Fatalf("scanner %q missing from output: %s", name, out.String())
		}
	}
}

func TestHookUsesSecurityNamespace(t *testing.T) {
	got := hookContent()
	want := "exec git-cli security check-staged"
	if !strings.Contains(got, want) {
		t.Fatalf("hook does not contain %q: %s", want, got)
	}
	if !managedHook(got) {
		t.Fatal("generated hook should be recognized as managed")
	}
}
