package scanner

import (
	"git-cli/internal/model"
	"testing"
)

func TestDeduplicate(t *testing.T) {
	in := []model.ScanResult{
		{Scanner: "gitleaks", Findings: []model.Finding{{Scanner: "gitleaks", Description: "API key", File: "a.go", Line: 10, Severity: model.SeverityHigh}}},
		{Scanner: "detect-secrets", Findings: []model.Finding{{Scanner: "detect-secrets", Description: "API key", File: "a.go", Line: 10, Severity: model.SeverityHigh}}},
	}
	out := Deduplicate(in)
	if len(out) != 1 {
		t.Fatalf("got %d findings", len(out))
	}
	if len(out[0].Scanners) != 2 {
		t.Fatalf("expected 2 scanners, got %#v", out[0].Scanners)
	}
}
