package cli

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/emkaytec/forge/internal/ui"
	"github.com/spf13/cobra"
)

const bootstrapGroupID = "bootstrap"
const demoGroupID = "demo"

var unknownCommandPattern = regexp.MustCompile(`unknown command "([^"]+)"`)

type Options struct {
	Verbose bool
	Debug   bool
}

func newRootCommand(stdout, stderr io.Writer, version string) *cobra.Command {
	options := &Options{}
	var versionRequested bool

	root := &cobra.Command{
		Use:           "forge",
		Short:         "Forge is the umbrella CLI for imperative engineering automation.",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if versionRequested {
				fmt.Fprintln(cmd.OutOrStdout(), version)
				return nil
			}

			renderHelp(cmd.OutOrStdout(), cmd, true)
			return nil
		},
	}

	root.SetOut(stdout)
	root.SetErr(stderr)
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddGroup(&cobra.Group{
		ID:    bootstrapGroupID,
		Title: "Bootstrap Commands",
	})
	root.AddGroup(&cobra.Group{
		ID:    demoGroupID,
		Title: "Demo Commands",
	})
	root.AddCommand(newHelpCommand(root))
	root.AddCommand(newDemoCommand())
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		renderHelp(cmd.OutOrStdout(), cmd, false)
	})
	root.Flags().BoolVarP(&versionRequested, "version", "v", false, "Print the Forge version")
	root.PersistentFlags().BoolVar(&options.Verbose, "verbose", false, "Enable verbose output")
	root.PersistentFlags().BoolVar(&options.Debug, "debug", false, "Enable debug output")

	return root
}

func newHelpCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:     "help",
		Short:   "Show this help output",
		GroupID: bootstrapGroupID,
		RunE: func(cmd *cobra.Command, args []string) error {
			renderHelp(cmd.OutOrStdout(), root, false)
			return nil
		},
	}
}

func renderHelp(w io.Writer, cmd *cobra.Command, includeBanner bool) {
	if includeBanner {
		ui.Banner(w, ui.Profile())
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, cmd.Short)
	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.RenderHeading(w, "Usage:"))
	fmt.Fprintln(w, "  forge [command]")
	fmt.Fprintln(w)

	writeGroupedCommands(w, cmd)
	writeFlags(w)
}

func writeGroupedCommands(w io.Writer, cmd *cobra.Command) {
	fmt.Fprintln(w, ui.RenderHeading(w, "Available commands:"))

	wroteGroup := false
	for _, group := range cmd.Groups() {
		commands := availableCommandsForGroup(cmd, group.ID)
		if len(commands) == 0 {
			continue
		}

		wroteGroup = true
		fmt.Fprintln(w, ui.RenderHeading(w, group.Title))
		for _, subcommand := range commands {
			fmt.Fprintf(
				w,
				"  %-16s %s\n",
				ui.RenderCommand(w, subcommand.Name()),
				ui.RenderMuted(w, subcommand.Short),
			)
		}
	}

	if !wroteGroup {
		fmt.Fprintln(w, "  (no commands registered)")
	}
}

func availableCommandsForGroup(cmd *cobra.Command, groupID string) []*cobra.Command {
	var commands []*cobra.Command

	for _, subcommand := range cmd.Commands() {
		if !subcommand.IsAvailableCommand() || subcommand.Hidden {
			continue
		}
		if strings.TrimSpace(subcommand.GroupID) != groupID {
			continue
		}

		commands = append(commands, subcommand)
	}

	return commands
}

func writeFlags(w io.Writer) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.RenderHeading(w, "Flags:"))

	flags := []struct {
		name        string
		description string
	}{
		{name: "-h, --help", description: "Show help for forge"},
		{name: "-v, --version", description: "Print the Forge version"},
		{name: "--verbose", description: "Enable verbose output"},
		{name: "--debug", description: "Enable debug output"},
	}

	for _, flag := range flags {
		fmt.Fprintf(
			w,
			"  %-16s %s\n",
			ui.RenderCommand(w, flag.name),
			ui.RenderMuted(w, flag.description),
		)
	}
}

func extractUnknownCommand(err error) string {
	if err == nil {
		return ""
	}

	matches := unknownCommandPattern.FindStringSubmatch(err.Error())
	if len(matches) != 2 {
		return ""
	}

	return matches[1]
}
