package main

import (
	"git-cli/internal/app"
	"os"
)

var version = "dev"

func main() {
	app.Version = version
	os.Exit(app.New().Run(os.Args[1:]))
}
