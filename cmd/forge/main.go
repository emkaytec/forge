package main

import (
	"os"

	"github.com/emkaytec/forge/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr, version); err != nil {
		os.Exit(1)
	}
}
