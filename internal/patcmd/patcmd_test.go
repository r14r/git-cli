package patcmd

import (
	"bytes"
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

func TestCredentialPathNamespacesName(t *testing.T) {
	got := credentialPath("deploy token")
	if got != "git-cli/pat/deploy%20token" {
		t.Fatalf("credentialPath() = %q", got)
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

func TestParseNameArgsRejectsBoth(t *testing.T) {
	var errOut bytes.Buffer
	_, code := parseNameArgs("pat display", []string{"--name", "release", "other"}, &errOut)
	if code != 2 {
		t.Fatalf("code=%d err=%q", code, errOut.String())
	}
}
