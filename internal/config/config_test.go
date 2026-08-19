package config

import "testing"

func TestDefault(t *testing.T) {
	c := Default()
	if !c.Scanners["gitleaks"].Required {
		t.Fatal("gitleaks must be required")
	}
	if c.Scanners["trufflehog"].Required {
		t.Fatal("trufflehog should be optional")
	}
}
