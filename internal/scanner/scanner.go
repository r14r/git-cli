package scanner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"git-cli/internal/config"
	"git-cli/internal/git"
	"git-cli/internal/model"
)

type Mode string

const (
	ModeStaged     Mode = "staged"
	ModeRepository Mode = "repository"
	ModeHistory    Mode = "history"
	ModeDeep       Mode = "deep"
)

type Runner interface {
	Name() string
	Available() bool
	Supports(Mode) bool
	Scan(context.Context, string, Mode, config.Config) model.ScanResult
}

func commandExists(name string) bool { _, err := exec.LookPath(name); return err == nil }

func fingerprint(file string, line int, desc string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", file, line, desc)))
	return hex.EncodeToString(h[:8])
}

func Deduplicate(results []model.ScanResult) []model.Finding {
	m := map[string]model.Finding{}
	for _, r := range results {
		for _, f := range r.Findings {
			key := fmt.Sprintf("%s:%d:%s", f.File, f.Line, strings.ToLower(f.Description))
			if f.Fingerprint != "" {
				key = f.Fingerprint
			}
			if old, ok := m[key]; ok {
				seen := map[string]bool{}
				for _, s := range old.Scanners {
					seen[s] = true
				}
				if old.Scanner != "" {
					seen[old.Scanner] = true
				}
				seen[f.Scanner] = true
				old.Scanners = old.Scanners[:0]
				for s := range seen {
					old.Scanners = append(old.Scanners, s)
				}
				sort.Strings(old.Scanners)
				m[key] = old
				continue
			}
			f.Scanners = []string{f.Scanner}
			m[key] = f
		}
	}
	out := make([]model.Finding, 0, len(m))
	for _, f := range m {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File == out[j].File {
			return out[i].Line < out[j].Line
		}
		return out[i].File < out[j].File
	})
	return out
}

// Gitleaks

type Gitleaks struct{}

func (Gitleaks) Name() string    { return "gitleaks" }
func (Gitleaks) Available() bool { return commandExists("gitleaks") }
func (Gitleaks) Supports(m Mode) bool {
	return m == ModeStaged || m == ModeRepository || m == ModeHistory
}

type gitleaksFinding struct {
	RuleID      string `json:"RuleID"`
	Description string `json:"Description"`
	File        string `json:"File"`
	StartLine   int    `json:"StartLine"`
	Fingerprint string `json:"Fingerprint"`
}

func (Gitleaks) Scan(ctx context.Context, root string, mode Mode, cfg config.Config) model.ScanResult {
	r := model.ScanResult{Scanner: "gitleaks"}
	tmp, err := os.CreateTemp("", "git-cli-gitleaks-*.json")
	if err != nil {
		r.Error = err.Error()
		return r
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)
	args := []string{"git", "--redact", "--report-format", "json", "--report-path", path}
	if mode == ModeStaged {
		args = append(args, "--pre-commit", "--staged")
	}
	if cfg.GitleaksConfig != "" {
		args = append(args, "--config", cfg.GitleaksConfig)
	}
	args = append(args, root)
	cmd := exec.CommandContext(ctx, "gitleaks", args...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	// Exit code 1 means findings; parse report before treating it as execution error.
	b, readErr := os.ReadFile(path)
	if readErr == nil && len(bytes.TrimSpace(b)) > 0 {
		var raw []gitleaksFinding
		if json.Unmarshal(b, &raw) == nil {
			for _, x := range raw {
				d := x.Description
				if d == "" {
					d = x.RuleID
				}
				r.Findings = append(r.Findings, model.Finding{Scanner: "gitleaks", RuleID: x.RuleID, Description: d, File: x.File, Line: x.StartLine, Fingerprint: x.Fingerprint, Severity: model.SeverityHigh})
			}
		}
	}
	if err != nil && len(r.Findings) == 0 {
		r.Error = strings.TrimSpace(stderr.String())
		if r.Error == "" {
			r.Error = err.Error()
		}
	}
	return r
}

// detect-secrets

type DetectSecrets struct{}

func (DetectSecrets) Name() string { return "detect-secrets" }
func (DetectSecrets) Available() bool {
	return commandExists("detect-secrets-hook") && commandExists("detect-secrets")
}
func (DetectSecrets) Supports(m Mode) bool { return m == ModeStaged || m == ModeRepository }

func (DetectSecrets) Scan(ctx context.Context, root string, mode Mode, cfg config.Config) model.ScanResult {
	r := model.ScanResult{Scanner: "detect-secrets"}
	if mode == ModeStaged {
		files, err := git.StagedFiles(root)
		if err != nil {
			r.Error = err.Error()
			return r
		}
		if len(files) == 0 {
			return r
		}
		args := []string{}
		baseline := filepath.Join(root, cfg.Baseline)
		if _, err := os.Stat(baseline); err == nil {
			args = append(args, "--baseline", baseline)
		}
		args = append(args, files...)
		cmd := exec.CommandContext(ctx, "detect-secrets-hook", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err == nil {
			return r
		}
		// detect-secrets-hook emits human-readable findings. Preserve them as one blocking finding.
		desc := strings.TrimSpace(string(out))
		if desc == "" {
			desc = "potential secret detected"
		}
		r.Findings = append(r.Findings, model.Finding{Scanner: "detect-secrets", Description: desc, Severity: model.SeverityHigh, Fingerprint: fingerprint("staged", 0, desc)})
		return r
	}
	cmd := exec.CommandContext(ctx, "detect-secrets", "scan")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		r.Error = err.Error()
		return r
	}
	var doc struct {
		Results map[string][]struct {
			Type   string `json:"type"`
			Line   int    `json:"line_number"`
			Hashed string `json:"hashed_secret"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		r.Error = err.Error()
		return r
	}
	for file, xs := range doc.Results {
		for _, x := range xs {
			r.Findings = append(r.Findings, model.Finding{Scanner: "detect-secrets", Description: x.Type, File: file, Line: x.Line, Fingerprint: x.Hashed, Severity: model.SeverityHigh})
		}
	}
	return r
}

// TruffleHog

type TruffleHog struct{}

func (TruffleHog) Name() string         { return "trufflehog" }
func (TruffleHog) Available() bool      { return commandExists("trufflehog") }
func (TruffleHog) Supports(m Mode) bool { return m == ModeDeep || m == ModeHistory }
func (TruffleHog) Scan(ctx context.Context, root string, mode Mode, cfg config.Config) model.ScanResult {
	r := model.ScanResult{Scanner: "trufflehog"}
	args := []string{"git", "file://" + root, "--json", "--no-update"}
	cmd := exec.CommandContext(ctx, "trufflehog", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		r.Error = err.Error()
		return r
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var x struct {
			DetectorName   string `json:"DetectorName"`
			Verified       bool   `json:"Verified"`
			SourceMetadata struct {
				Data struct {
					Git struct {
						File string `json:"file"`
						Line int    `json:"line"`
					} `json:"Git"`
				} `json:"Data"`
			} `json:"SourceMetadata"`
		}
		if err := dec.Decode(&x); err != nil {
			break
		}
		v := x.Verified
		sev := model.SeverityHigh
		if v {
			sev = model.SeverityCritical
		}
		f := model.Finding{Scanner: "trufflehog", Description: x.DetectorName, File: x.SourceMetadata.Data.Git.File, Line: x.SourceMetadata.Data.Git.Line, Severity: sev, Verified: &v}
		f.Fingerprint = fingerprint(f.File, f.Line, f.Description)
		r.Findings = append(r.Findings, f)
	}
	return r
}
