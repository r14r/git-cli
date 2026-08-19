package ui

import (
	"fmt"
	"io"
	"strings"

	"git-cli/internal/model"
)

func PrintScan(w io.Writer, version, root, mode string, findings []model.Finding, results []model.ScanResult) {
	fmt.Fprintf(w, "Git Security %s\n\nRepository: %s\nMode:       %s\n\n", version, root, mode)
	fmt.Fprintln(w, "Scanner          Findings   Status")
	fmt.Fprintln(w, strings.Repeat("─", 38))
	for _, r := range results {
		status := "PASS"
		if r.Skipped {
			status = "SKIP"
		} else if r.Error != "" {
			status = "ERROR"
		} else if len(r.Findings) > 0 {
			status = "FAIL"
		}
		fmt.Fprintf(w, "%-16s %-10d %s\n", r.Scanner, len(r.Findings), status)
		if r.Error != "" {
			fmt.Fprintf(w, "  error: %s\n", r.Error)
		}
	}
	if len(findings) > 0 {
		fmt.Fprintln(w, "\nFindings")
		fmt.Fprintln(w, strings.Repeat("─", 38))
		for _, f := range findings {
			loc := f.File
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", f.File, f.Line)
			}
			if loc == "" {
				loc = "(staged changes)"
			}
			fmt.Fprintf(w, "%s  %s\n  %s\n  detected by: %s\n", f.Severity, f.Description, loc, strings.Join(f.Scanners, ", "))
		}
	}
	fmt.Fprintln(w)
	if len(findings) > 0 {
		fmt.Fprintf(w, "Result: BLOCKED (%d unique finding(s))\n", len(findings))
	} else {
		fmt.Fprintln(w, "Result: SAFE")
	}
}
