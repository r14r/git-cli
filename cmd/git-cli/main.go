package main

import (
	"fmt"
	"os"

	"git-cli/internal/app"
	"git-cli/internal/doctor"
	"git-cli/internal/precommitcmd"
	"git-cli/internal/projectcmd"
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
  git-cli project detect           Detect the current application type
  git-cli project info             Show current project information
  git-cli doctor                   Diagnose repository/tooling setup
  git-cli version                  Show version
`)
		return
	}
	switch args[0] {
	case "precommit":
		os.Exit(precommitcmd.Run(args[1:], os.Stdout, os.Stderr))
	case "project":
		os.Exit(projectcmd.Run(args[1:], os.Stdout, os.Stderr))
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
