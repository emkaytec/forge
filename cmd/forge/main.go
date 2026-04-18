package main

import (
	"fmt"
	"os"

	"github.com/emkaytec/forge/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr, version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
