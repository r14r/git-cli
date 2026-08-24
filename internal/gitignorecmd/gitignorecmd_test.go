package gitignorecmd

import (
	"strings"
	"testing"
)

func TestUpsertBlockPreservesUserRules(t *testing.T) {
	existing := "# user rules\n.env.local\n"
	got := upsertBlock(existing, "django", "__pycache__/\n*.log\n")
	if !strings.Contains(got, "# user rules\n.env.local") {
		t.Fatalf("user rules were not preserved: %q", got)
	}
	if !strings.Contains(got, "# >>> git-cli gitignore: django") {
		t.Fatalf("managed block missing: %q", got)
	}

	updated := upsertBlock(got, "django", "__pycache__/\n*.sqlite3\n")
	if strings.Count(updated, "# >>> git-cli gitignore: django") != 1 {
		t.Fatalf("expected one managed block: %q", updated)
	}
	if !strings.Contains(updated, "*.sqlite3") || strings.Contains(updated, "*.log") {
		t.Fatalf("managed block was not replaced: %q", updated)
	}
}

func TestRemoveBlock(t *testing.T) {
	in := "keep.me\n\n" + managedBlock("python", "__pycache__/\n")
	got, removed := removeBlock(in, "python")
	if !removed {
		t.Fatal("expected block removal")
	}
	if strings.Contains(got, "git-cli gitignore") || !strings.Contains(got, "keep.me") {
		t.Fatalf("unexpected result: %q", got)
	}
}

func TestNormalizeTemplatePath(t *testing.T) {
	cases := map[string]string{
		"Python":       "Python.gitignore",
		"Python.gitignore": "Python.gitignore",
		"Global/macOS": "Global/macOS.gitignore",
	}
	for in, want := range cases {
		if got := normalizeTemplatePath(in); got != want {
			t.Fatalf("normalizeTemplatePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolvePresets(t *testing.T) {
	for _, name := range []string{"python", "fastapi", "django", "go", "node", "laravel"} {
		sel, err := resolveSelection(name, "")
		if err != nil {
			t.Fatalf("resolveSelection(%q): %v", name, err)
		}
		if sel.Key == "" {
			t.Fatalf("preset %q has empty key", name)
		}
	}
}

func TestManagedSections(t *testing.T) {
	content := managedBlock("python", "a") + "\n" + managedBlock("global-macos", "b")
	got := managedSections(content)
	if len(got) != 2 || got[0] != "global-macos" || got[1] != "python" {
		t.Fatalf("unexpected sections: %#v", got)
	}
}
