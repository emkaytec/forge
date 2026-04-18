package cli

import "io"

// Run executes the Forge CLI using the Cobra-powered bootstrap shell.
func Run(args []string, stdout, stderr io.Writer, version string) error {
	root := newRootCommand(stdout, stderr, version)
	root.SetArgs(args)
	return root.Execute()
}
