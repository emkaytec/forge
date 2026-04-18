package cli

import (
	"fmt"
	"io"
)

// Run executes the Forge CLI using the Cobra-powered bootstrap shell.
func Run(args []string, stdout, stderr io.Writer, version string) error {
	root := newRootCommand(stdout, stderr, version)
	root.SetArgs(args)
	err := root.Execute()
	if extractUnknownCommand(err) == "" {
		return err
	}

	fmt.Fprintln(stderr, err)
	fmt.Fprintln(stderr)
	renderHelp(stderr, root, false)

	return err
}
