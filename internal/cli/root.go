package cli

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/emkaytec/forge/internal/initcmd"
	"github.com/emkaytec/forge/internal/manifest"
	"github.com/emkaytec/forge/internal/reconcilecmd"
	"github.com/emkaytec/forge/internal/ui"
	selfupdate "github.com/emkaytec/forge/internal/update"
	"github.com/emkaytec/forge/internal/workstation"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	setupGroupID    = "setup"
	workflowGroupID = "workflow"
)

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

			renderHelp(cmd.OutOrStdout(), cmd, true, rootScreenUpdateNotice(cmd.Context(), version))
			return nil
		},
	}

	root.SetOut(stdout)
	root.SetErr(stderr)
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpCommand(&cobra.Command{Hidden: true})
	root.SetHelpCommandGroupID("")
	root.AddGroup(&cobra.Group{
		ID:    setupGroupID,
		Title: "Setup Commands",
	})
	root.AddGroup(&cobra.Group{
		ID:    workflowGroupID,
		Title: "Workflow Commands",
	})

	groupAssignments := []struct {
		command *cobra.Command
		groupID string
	}{
		{newHelpCommand(root), setupGroupID},
		{initcmd.Command(), setupGroupID},
		{newUpdateCommand(version), setupGroupID},
		{manifest.Command(), workflowGroupID},
		{reconcilecmd.Command(), workflowGroupID},
		{workstation.Command(), workflowGroupID},
	}
	for _, assignment := range groupAssignments {
		assignment.command.GroupID = assignment.groupID
		root.AddCommand(assignment.command)
	}
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		renderHelp(cmd.OutOrStdout(), cmd, false, "")
	})
	root.InitDefaultHelpFlag()
	root.Flags().BoolVarP(&versionRequested, "version", "v", false, "Print the Forge version")
	if helpFlag := root.Flags().Lookup("help"); helpFlag != nil {
		helpFlag.Usage = "Show help for forge"
	}
	root.PersistentFlags().BoolVar(&options.Verbose, "verbose", false, "Enable verbose output")
	root.PersistentFlags().BoolVar(&options.Debug, "debug", false, "Enable debug output")

	return root
}

func newHelpCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "help",
		Short: "Show this help output",
		RunE: func(cmd *cobra.Command, args []string) error {
			renderHelp(cmd.OutOrStdout(), root, false, "")
			return nil
		},
	}
}

func renderHelp(w io.Writer, cmd *cobra.Command, includeBanner bool, notice string) {
	if includeBanner {
		ui.Banner(w, ui.Profile())
	}

	fmt.Fprintln(w, cmd.Short)
	if longDescription := strings.TrimSpace(cmd.Long); longDescription != "" && longDescription != cmd.Short {
		fmt.Fprintln(w)
		fmt.Fprintln(w, longDescription)
	}
	if notice = strings.TrimSpace(notice); notice != "" {
		fmt.Fprintln(w)
		ui.Warn(w, notice)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.RenderHeading(w, "Usage"))
	fmt.Fprintf(w, "  %s\n", usageLine(cmd))
	fmt.Fprintln(w)

	colWidth := computeColumnWidth(cmd)
	writeAvailableCommands(w, cmd, colWidth)
	writeFlags(w, cmd, colWidth)
	writeExamples(w, cmd)
}

func rootScreenUpdateNotice(ctx context.Context, version string) string {
	if !isReleasedVersion(version) {
		return ""
	}

	runner := newUpdateRunner(version)
	result, err := runner.Run(ctx, selfupdate.Options{Check: true})
	if err != nil || result.UpToDate || strings.TrimSpace(result.TargetVersion) == "" {
		return ""
	}

	return fmt.Sprintf(
		"Update available: %s -> %s. Run `forge update` to install it.",
		displayVersion(result.CurrentVersion),
		result.TargetVersion,
	)
}

func isReleasedVersion(version string) bool {
	var major, minor, patch int
	_, err := fmt.Sscanf(strings.TrimSpace(version), "v%d.%d.%d", &major, &minor, &patch)
	return err == nil
}

func usageLine(cmd *cobra.Command) string {
	if cmd.HasAvailableSubCommands() {
		return cmd.CommandPath() + " [command]"
	}

	suffix := strings.TrimSpace(strings.TrimPrefix(cmd.Use, cmd.Name()))
	if suffix == "" {
		return cmd.CommandPath()
	}

	return cmd.CommandPath() + " " + suffix
}

func computeColumnWidth(cmd *cobra.Command) int {
	width := 0
	consider := func(name string) {
		if n := len(name); n > width {
			width = n
		}
	}

	for _, subcommand := range cmd.Commands() {
		if !subcommand.IsAvailableCommand() || subcommand.Hidden {
			continue
		}
		consider(subcommand.Name())
	}

	for _, f := range visibleFlags(cmd) {
		consider(f.name)
	}

	// Minimum width keeps short command lists from feeling cramped.
	if width < 10 {
		width = 10
	}

	return width
}

func writeAvailableCommands(w io.Writer, cmd *cobra.Command, colWidth int) {
	if writeGroupedCommands(w, cmd, colWidth) {
		return
	}

	commands := availableUngroupedCommands(cmd)
	if len(commands) == 0 {
		fmt.Fprintln(w, ui.RenderHeading(w, "Commands"))
		fmt.Fprintln(w, "  (no commands registered)")
		return
	}

	fmt.Fprintln(w, ui.RenderHeading(w, "Commands"))
	writeCommandList(w, commands, colWidth)
}

func writeGroupedCommands(w io.Writer, cmd *cobra.Command, colWidth int) bool {
	wroteGroup := false
	for _, group := range cmd.Groups() {
		commands := availableCommandsForGroup(cmd, group.ID)
		if len(commands) == 0 {
			continue
		}

		if wroteGroup {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, ui.RenderHeading(w, strings.TrimSuffix(group.Title, " Commands")))
		writeCommandList(w, commands, colWidth)
		wroteGroup = true
	}

	return wroteGroup
}

func writeCommandList(w io.Writer, commands []*cobra.Command, colWidth int) {
	for _, subcommand := range commands {
		name := subcommand.Name()
		pad := strings.Repeat(" ", colWidth-len(name))
		fmt.Fprintf(
			w,
			"  %s%s   %s\n",
			ui.RenderCommand(w, name),
			pad,
			ui.RenderMuted(w, subcommand.Short),
		)
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

func availableUngroupedCommands(cmd *cobra.Command) []*cobra.Command {
	var commands []*cobra.Command

	for _, subcommand := range cmd.Commands() {
		if !subcommand.IsAvailableCommand() || subcommand.Hidden {
			continue
		}
		if strings.TrimSpace(subcommand.GroupID) != "" {
			continue
		}

		commands = append(commands, subcommand)
	}

	return commands
}

func writeFlags(w io.Writer, cmd *cobra.Command, colWidth int) {
	flags := visibleFlags(cmd)
	if len(flags) == 0 {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.RenderHeading(w, "Flags"))

	for _, flag := range flags {
		pad := strings.Repeat(" ", colWidth-len(flag.name))
		fmt.Fprintf(
			w,
			"  %s%s   %s\n",
			ui.RenderCommand(w, flag.name),
			pad,
			ui.RenderMuted(w, flag.description),
		)
	}
}

func writeExamples(w io.Writer, cmd *cobra.Command) {
	example := strings.TrimSpace(cmd.Example)
	if example == "" {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.RenderHeading(w, "Examples"))
	for _, line := range strings.Split(example, "\n") {
		fmt.Fprintln(w, line)
	}
}

func visibleFlags(cmd *cobra.Command) []struct {
	name        string
	description string
} {
	flagMap := map[string]struct {
		name        string
		description string
	}{}

	recordFlag := func(flag *pflag.Flag) {
		if flag == nil || flag.Hidden {
			return
		}

		name := "--" + flag.Name
		if flag.Shorthand != "" {
			name = "-" + flag.Shorthand + ", " + name
		}

		flagMap[name] = struct {
			name        string
			description string
		}{
			name:        name,
			description: flag.Usage,
		}
	}

	cmd.NonInheritedFlags().VisitAll(recordFlag)
	cmd.InheritedFlags().VisitAll(recordFlag)

	names := make([]string, 0, len(flagMap))
	for name := range flagMap {
		names = append(names, name)
	}
	sort.Strings(names)

	flags := make([]struct {
		name        string
		description string
	}, 0, len(names))
	for _, name := range names {
		flags = append(flags, flagMap[name])
	}

	return flags
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
