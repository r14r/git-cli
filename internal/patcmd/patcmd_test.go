package patcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

func TestParseNameArgsPositional(t *testing.T) {
	var errOut bytes.Buffer
	got, code := parseNameArgs("pat display", []string{"release"}, &errOut)
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

func TestEncryptDecrypt(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	encrypted, err := encryptToken(key, "github_pat_secret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encrypted, encPrefix) {
		t.Fatalf("missing encryption prefix: %q", encrypted)
	}
	if strings.Contains(encrypted, "github_pat_secret") {
		t.Fatal("ciphertext contains plaintext token")
	}
	plain, err := decryptToken(key, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "github_pat_secret" {
		t.Fatalf("plain = %q", plain)
	}
}

func TestEncryptedStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(configDirEnv, dir)
	path := filepath.Join(dir, "pat.yaml")

	key, err := loadOrCreateKey()
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := encryptToken(key, "github_pat_abc123")
	if err != nil {
		t.Fatal(err)
	}
	s := store{PATs: map[string]string{"runner": encrypted}}
	if err := saveStore(path, s); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "github_pat_abc123") {
		t.Fatal("pat.yaml contains plaintext token")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("pat mode = %04o", info.Mode().Perm())
	}
	keyInfo, err := os.Stat(filepath.Join(dir, "pat.key"))
	if err != nil {
		t.Fatal(err)
	}
	if keyInfo.Mode().Perm() != 0o600 {
		t.Fatalf("key mode = %04o", keyInfo.Mode().Perm())
	}

	got, _, err := loadStore()
	if err != nil {
		t.Fatal(err)
	}
	plain, err := decryptToken(key, got.PATs["runner"])
	if err != nil {
		t.Fatal(err)
	}
	if plain != "github_pat_abc123" {
		t.Fatalf("plain = %q", plain)
	}
}

func TestSecureMigratesPlaintext(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(configDirEnv, dir)
	path := filepath.Join(dir, "pat.yaml")
	if err := os.WriteFile(path, []byte("pats:\n  \"runner\": \"github_pat_old\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := secure(&out, &errOut); code != 0 {
		t.Fatalf("secure code=%d err=%q", code, errOut.String())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "github_pat_old") {
		t.Fatal("plaintext token remained after secure")
	}
	if !strings.Contains(string(b), encPrefix) {
		t.Fatal("encrypted value missing")
	}
}
