package cli

import (
	"fmt"
	"io"
)

// Run executes the Forge CLI using a lightweight command handler during bootstrap.
func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		writeHelp(stdout)
		return nil
	}

	switch args[0] {
	case "-h", "--help", "help":
		writeHelp(stdout)
		return nil
	default:
		writeHelp(stderr)
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func writeHelp(w io.Writer) {
	fmt.Fprintln(w, "Forge is the umbrella CLI for imperative engineering automation.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  forge <command>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Available commands:")
	fmt.Fprintln(w, "  help    Show this help output")
}
