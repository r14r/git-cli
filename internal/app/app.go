package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"git-cli/internal/config"
	gitutil "git-cli/internal/git"
	"git-cli/internal/model"
	"git-cli/internal/precommit"
	"git-cli/internal/scanner"
	"git-cli/internal/ui"
)

var Version = "dev"

type App struct{ Out, Err io.Writer }

func New() *App { return &App{Out: os.Stdout, Err: os.Stderr} }

func (a *App) Run(args []string) int {
	if len(args) == 0 {
		a.help()
		return 0
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintf(a.Out, "git-cli %s\n", Version)
		return 0
	case "help", "--help", "-h":
		a.help()
		return 0
	case "security":
		return a.securityCommand(args[1:])
	case "precommit":
		return a.precommitCommand(args[1:])
	default:
		fmt.Fprintf(a.Err, "unknown command: %s\n", args[0])
		a.help()
		return 2
	}
}

func (a *App) help() {
	fmt.Fprintln(a.Out, `git-cli - Git workflow utilities

Usage:
  git-cli security <command>       Secret scanning and commit protection
  git-cli precommit <options>      Configure/run application pre-commit checks
  git-cli version                  Show version
  git-cli help                     Show help

Run "git-cli security help" or "git-cli precommit help" for details.`)
}

func (a *App) precommitCommand(args []string) int {
	if len(args) == 0 {
		a.precommitHelp()
		return 0
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		a.precommitHelp()
		return 0
	}
	if args[0] == "run" {
		root, err := gitutil.Root()
		if err != nil {
			fmt.Fprintln(a.Err, err)
			return 2
		}
		if err := precommit.Run(root, a.Out, a.Err); err != nil {
			fmt.Fprintln(a.Err, err)
			return 1
		}
		return 0
	}
	if args[0] == "status" {
		return a.precommitStatus()
	}
	if args[0] == "list" {
		fmt.Fprintln(a.Out, strings.Join(precommit.Supported(), "\n"))
		return 0
	}

	fs := flag.NewFlagSet("precommit", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	setup := fs.Bool("setup", false, "install pre-commit configuration and hook")
	preset := fs.String("for", "", "application preset: python|fastapi|django|laravel")
	scan := fs.Bool("scan", false, "detect application preset from current repository")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !*setup {
		fmt.Fprintln(a.Err, "precommit requires --setup, or use run|status|list")
		return 2
	}
	if (*preset == "" && !*scan) || (*preset != "" && *scan) {
		fmt.Fprintln(a.Err, "use exactly one of --for <preset> or --scan")
		return 2
	}
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}
	selected := *preset
	if *scan {
		selected, err = precommit.Detect(root)
		if err != nil {
			fmt.Fprintln(a.Err, err)
			return 2
		}
		fmt.Fprintf(a.Out, "Detected application preset: %s\n", selected)
	}
	selected, err = precommit.NormalizePreset(selected)
	if err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}
	hook, err := precommit.Setup(root, selected)
	if err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}
	cfgPath := filepath.Join(root, config.DefaultFilename)
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		if err := config.WriteDefault(cfgPath); err != nil {
			fmt.Fprintln(a.Err, err)
			return 2
		}
	}
	fmt.Fprintf(a.Out, "Installed pre-commit preset: %s\nHook: %s\nConfig: %s\n", selected, hook, filepath.Join(root, precommit.ConfigFilename))
	return 0
}

func (a *App) precommitHelp() {
	fmt.Fprintln(a.Out, `git-cli precommit - application-aware pre-commit setup

Usage:
  git-cli precommit --setup --for python
  git-cli precommit --setup --for fastapi
  git-cli precommit --setup --for django
  git-cli precommit --setup --for laravel
  git-cli precommit --setup --scan
  git-cli precommit run
  git-cli precommit status
  git-cli precommit list

The managed hook runs secret scanning first, then application-specific checks.`)
}

func (a *App) precommitStatus() int {
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}
	cfg, cfgErr := precommit.Load(root)
	gitDir, _ := gitutil.GitDir(root)
	hook := filepath.Join(gitDir, "hooks", "pre-commit")
	managed := false
	if b, e := os.ReadFile(hook); e == nil {
		managed = precommit.ManagedHook(string(b))
	}
	fmt.Fprintf(a.Out, "Repository: %s\nManaged hook: %v\n", root, managed)
	if cfgErr != nil {
		fmt.Fprintf(a.Out, "Preset: not configured (%s)\n", precommit.ConfigFilename)
		return 0
	}
	fmt.Fprintf(a.Out, "Preset: %s\nConfig: %s\n", cfg.Preset, filepath.Join(root, precommit.ConfigFilename))
	return 0
}

func (a *App) securityCommand(args []string) int {
	if len(args) == 0 {
		a.securityHelp()
		return 0
	}

	switch args[0] {
	case "help", "--help", "-h":
		a.securityHelp()
		return 0
	case "check-staged":
		return a.scan(scanner.ModeStaged, args[1:])
	case "check":
		mode := scanner.ModeRepository
		for _, x := range args[1:] {
			if x == "--deep" {
				mode = scanner.ModeDeep
			}
		}
		return a.scan(mode, args[1:])
	case "check-history":
		return a.scan(scanner.ModeHistory, args[1:])
	case "install":
		return a.install(args[1:])
	case "uninstall":
		return a.uninstall()
	case "status":
		return a.status()
	case "scanner":
		return a.scannerCommand(args[1:])
	default:
		fmt.Fprintf(a.Err, "unknown security command: %s\n", args[0])
		a.securityHelp()
		return 2
	}
}

func (a *App) securityHelp() {
	fmt.Fprintln(a.Out, `git-cli security - local Git secret scanning

Usage:
  git-cli security install             Install repository pre-commit hook
  git-cli security uninstall           Remove managed pre-commit hook
  git-cli security status              Show repository security status
  git-cli security check-staged        Scan staged changes (pre-commit)
  git-cli security check               Scan current repository
  git-cli security check --deep        Deep scan using TruffleHog
  git-cli security check-history       Scan repository history
  git-cli security scanner list        List supported scanners
  git-cli security scanner status      Show scanner configuration/availability`)
}

func runners() []scanner.Runner {
	return []scanner.Runner{scanner.Gitleaks{}, scanner.DetectSecrets{}, scanner.TruffleHog{}}
}

func (a *App) scan(mode scanner.Mode, args []string) int {
	fs := flag.NewFlagSet("security scan", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	jsonOut := fs.Bool("json", false, "output JSON")
	timeout := fs.Duration("timeout", 2*time.Minute, "scanner timeout")
	_ = fs.Bool("deep", false, "deep scan")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}
	cfg, err := config.Load(filepath.Join(root, config.DefaultFilename))
	if err != nil {
		fmt.Fprintln(a.Err, "config:", err)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	var results []model.ScanResult
	for _, r := range runners() {
		sc := cfg.Scanners[r.Name()]
		if !sc.Enabled || !r.Supports(mode) {
			continue
		}
		if !r.Available() {
			res := model.ScanResult{Scanner: r.Name(), Skipped: true}
			if sc.Required {
				res.Skipped = false
				res.Error = "required scanner is not installed"
			}
			results = append(results, res)
			continue
		}
		results = append(results, r.Scan(ctx, root, mode, cfg))
	}

	findings := scanner.Deduplicate(results)
	if *jsonOut {
		b, _ := json.MarshalIndent(struct {
			Version  string             `json:"version"`
			Mode     scanner.Mode       `json:"mode"`
			Findings []model.Finding    `json:"findings"`
			Results  []model.ScanResult `json:"results"`
		}{Version, mode, findings, results}, "", "  ")
		fmt.Fprintln(a.Out, string(b))
	} else {
		ui.PrintScan(a.Out, Version, root, string(mode), findings, results)
	}

	for _, r := range results {
		if r.Error != "" {
			return 2
		}
	}
	if len(findings) > 0 {
		return 1
	}
	return 0
}

func hookContent() string {
	return "#!/usr/bin/env bash\nset -e\n\nexec git-cli security check-staged\n"
}

func managedHook(content string) bool {
	return strings.Contains(content, "git-cli security check-staged")
}

func (a *App) install(args []string) int {
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}
	gitDir, err := gitutil.GitDir(root)
	if err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}

	hooks := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}
	path := filepath.Join(hooks, "pre-commit")
	if b, err := os.ReadFile(path); err == nil && !managedHook(string(b)) {
		fmt.Fprintf(a.Err, "refusing to overwrite existing hook: %s\n", path)
		return 2
	}
	if err := os.WriteFile(path, []byte(hookContent()), 0o755); err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}

	cfgPath := filepath.Join(root, config.DefaultFilename)
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		if err := config.WriteDefault(cfgPath); err != nil {
			fmt.Fprintln(a.Err, err)
			return 2
		}
	}
	fmt.Fprintf(a.Out, "Installed pre-commit hook: %s\nConfig: %s\n", path, cfgPath)
	return 0
}

func (a *App) uninstall() int {
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}
	gitDir, _ := gitutil.GitDir(root)
	p := filepath.Join(gitDir, "hooks", "pre-commit")
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(a.Out, "No git-cli security hook installed.")
		return 0
	}
	if err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}
	if !managedHook(string(b)) {
		fmt.Fprintln(a.Err, "pre-commit hook is not managed by git-cli security")
		return 2
	}
	if err := os.Remove(p); err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}
	fmt.Fprintln(a.Out, "Removed git-cli security pre-commit hook.")
	return 0
}

func (a *App) status() int {
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}
	fmt.Fprintf(a.Out, "Git CLI Security %s\nRepository: %s\nOS/Arch: %s/%s\n\n", Version, root, runtime.GOOS, runtime.GOARCH)
	gitDir, _ := gitutil.GitDir(root)
	hook := filepath.Join(gitDir, "hooks", "pre-commit")
	managed := false
	if b, e := os.ReadFile(hook); e == nil {
		managed = managedHook(string(b))
	}
	fmt.Fprintf(a.Out, "Pre-commit hook: %v\nConfig:          %v (%s)\n\n", managed, fileExists(filepath.Join(root, config.DefaultFilename)), config.DefaultFilename)
	return a.scannerStatus(root)
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func (a *App) scannerCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(a.Err, "usage: git-cli security scanner <list|status>")
		return 2
	}

	switch args[0] {
	case "list":
		fmt.Fprintln(a.Out, "Scanner          Modes")
		fmt.Fprintln(a.Out, "-------------------------------------------")
		for _, r := range runners() {
			var modes []string
			for _, mode := range []scanner.Mode{scanner.ModeStaged, scanner.ModeRepository, scanner.ModeDeep, scanner.ModeHistory} {
				if r.Supports(mode) {
					modes = append(modes, string(mode))
				}
			}
			fmt.Fprintf(a.Out, "%-16s %s\n", r.Name(), strings.Join(modes, ", "))
		}
		return 0
	case "status":
		root, err := gitutil.Root()
		if err != nil {
			return a.scannerStatus("")
		}
		return a.scannerStatus(root)
	default:
		fmt.Fprintln(a.Err, "usage: git-cli security scanner <list|status>")
		return 2
	}
}

func (a *App) scannerStatus(root string) int {
	cfg := config.Default()
	configPath := "defaults"
	if root != "" {
		configPath = filepath.Join(root, config.DefaultFilename)
		loaded, err := config.Load(configPath)
		if err != nil {
			fmt.Fprintln(a.Err, "config:", err)
			return 2
		}
		cfg = loaded
	}

	fmt.Fprintf(a.Out, "Scanner configuration: %s\n", configPath)
	fmt.Fprintln(a.Out, "Scanner          Enabled  Required  Installed")
	fmt.Fprintln(a.Out, "----------------------------------------------")
	for _, r := range runners() {
		sc := cfg.Scanners[r.Name()]
		fmt.Fprintf(a.Out, "%-16s %-8v %-9v %v\n", r.Name(), sc.Enabled, sc.Required, r.Available())
	}
	return 0
}
