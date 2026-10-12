package patcmd

import "testing"

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

func TestUsernameUsesNamespace(t *testing.T) {
	if got := username("deploy"); got != "git-cli-pat:deploy" {
		t.Fatalf("username() = %q", got)
	}
}
