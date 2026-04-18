package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/emkaytec/forge/internal/ui"
	"github.com/spf13/cobra"
)

const bootstrapGroupID = "bootstrap"

func newRootCommand(stdout, stderr io.Writer, version string) *cobra.Command {
	_ = version

	root := &cobra.Command{
		Use:           "forge",
		Short:         "Forge is the umbrella CLI for imperative engineering automation.",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
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
	root.AddCommand(newHelpCommand(root))
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		renderHelp(cmd.OutOrStdout(), cmd, false)
	})

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
