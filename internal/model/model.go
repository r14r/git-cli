package model

type Severity string

const (
	SeverityLow      Severity = "LOW"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

type Finding struct {
	Scanner     string   `json:"scanner"`
	Scanners    []string `json:"scanners,omitempty"`
	RuleID      string   `json:"rule_id,omitempty"`
	Description string   `json:"description"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Severity    Severity `json:"severity"`
	Verified    *bool    `json:"verified,omitempty"`
}

type ScanResult struct {
	Scanner  string    `json:"scanner"`
	Findings []Finding `json:"findings"`
	Error    string    `json:"error,omitempty"`
	Skipped  bool      `json:"skipped,omitempty"`
}
