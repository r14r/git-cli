package main

import (
	"fmt"
	"os"

	"git-cli/internal/app"
	"git-cli/internal/branchcmd"
	"git-cli/internal/cleancmd"
	"git-cli/internal/doctor"
	"git-cli/internal/gitignorecmd"
	"git-cli/internal/gitignorediag"
	"git-cli/internal/largefilescmd"
	"git-cli/internal/precommitcmd"
	"git-cli/internal/projectcmd"
	"git-cli/internal/repocmd"
)

var version = "dev"

func main() {
	app.Version = version
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(`git-cli - Git workflow utilities

Usage:
  git-cli security <command>       Secret scanning and commit protection
  git-cli precommit <options>      Configure/run application pre-commit checks
  git-cli gitignore <command>      Create/manage .gitignore from GitHub templates
  git-cli project detect           Detect the current application type
  git-cli project info             Show current project information
  git-cli repo health              Inspect repository health
  git-cli clean <command>          Safely preview/remove generated files
  git-cli large-files scan         Find large tracked files
  git-cli branch <command>         Inspect merged/stale branches
  git-cli doctor                   Diagnose repository/tooling setup
  git-cli version                  Show version
`)
		return
	}
	switch args[0] {
	case "precommit":
		os.Exit(precommitcmd.Run(args[1:], os.Stdout, os.Stderr))
	case "gitignore":
		if len(args) > 1 && (args[1] == "explain" || args[1] == "tracked") {
			os.Exit(gitignorediag.Run(args[1:], os.Stdout, os.Stderr))
		}
		os.Exit(gitignorecmd.Run(args[1:], os.Stdout, os.Stderr))
	case "project":
		os.Exit(projectcmd.Run(args[1:], os.Stdout, os.Stderr))
	case "repo":
		os.Exit(repocmd.Run(args[1:], os.Stdout, os.Stderr))
	case "clean":
		os.Exit(cleancmd.Run(args[1:], os.Stdout, os.Stderr))
	case "large-files":
		os.Exit(largefilescmd.Run(args[1:], os.Stdout, os.Stderr))
	case "branch":
		os.Exit(branchcmd.Run(args[1:], os.Stdout, os.Stderr))
	case "doctor":
		os.Exit(doctor.Run(os.Stdout, os.Stderr))
	case "hook":
		if len(args) == 3 && args[1] == "run" && args[2] == "pre-commit" {
			code := app.New().Run([]string{"security", "check-staged"})
			if code != 0 {
				os.Exit(code)
			}
			os.Exit(precommitcmd.Run([]string{"run"}, os.Stdout, os.Stderr))
		}
	}
	os.Exit(app.New().Run(args))
}
