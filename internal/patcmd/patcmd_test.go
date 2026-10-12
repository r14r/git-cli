package patcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeName(t *testing.T) {
	got, err := normalizeName(" release ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "release" {
		t.Fatalf("normalizeName() = %q", got)
	}
}

func TestNormalizeNameRejectsEmpty(t *testing.T) {
	if _, err := normalizeName("   "); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseNameArgsPositional(t *testing.T) {
	var errOut bytes.Buffer
	got, code := parseNameArgs("pat display", []string{"release"}, &errOut)
	if code != 0 || got != "release" {
		t.Fatalf("got=%q code=%d err=%q", got, code, errOut.String())
	}
}

func TestParseNameArgsFlag(t *testing.T) {
	var errOut bytes.Buffer
	got, code := parseNameArgs("pat display", []string{"--name", "release"}, &errOut)
	if code != 0 || got != "release" {
		t.Fatalf("got=%q code=%d err=%q", got, code, errOut.String())
	}
}

func TestConfigPathForExecutable(t *testing.T) {
	got := configPathForExecutable("/usr/local/bin/git-cli")
	want := "/usr/local/etc/git-config/pat.yaml"
	if got != want {
		t.Fatalf("configPathForExecutable() = %q, want %q", got, want)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(configDirEnv, dir)
	path := filepath.Join(dir, "pat.yaml")

	s := store{PATs: map[string]string{
		"github-selfhosted-runner": "github_pat_abc:123",
		"release":                  "token with spaces",
	}}
	if err := saveStore(path, s); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %04o, want 0600", got)
	}

	got, gotPath, err := loadStore()
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if got.PATs["github-selfhosted-runner"] != "github_pat_abc:123" {
		t.Fatalf("unexpected token: %q", got.PATs["github-selfhosted-runner"])
	}
	if got.PATs["release"] != "token with spaces" {
		t.Fatalf("unexpected token: %q", got.PATs["release"])
	}
}
