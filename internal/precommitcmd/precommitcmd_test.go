package precommitcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHooksDirRespectsCoreHooksPath(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "core.hooksPath", ".githooks")
	got, err := hooksDir(root)
	if err != nil { t.Fatal(err) }
	want := filepath.Join(root, ".githooks")
	if got != want { t.Fatalf("hooksDir() = %q, want %q", got, want) }
}

func TestInstallHookUsesDispatcher(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "core.hooksPath", ".githooks")
	path, err := installHook(root)
	if err != nil { t.Fatal(err) }
	b, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	if !strings.Contains(string(b), "exec git-cli hook run pre-commit") { t.Fatalf("unexpected hook: %s", b) }
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil { t.Fatalf("git %v: %v: %s", args, err, out) }
}
