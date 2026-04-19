package manifest

import "github.com/spf13/cobra"

// GroupID is the cobra group that hosts manifest subcommands in help output.
const GroupID = "manifest"

// Command returns the configured manifest command group.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "manifest",
		Short:   "Generate and validate Forge manifests",
		GroupID: GroupID,
	}

	cmd.AddCommand(newGenerateCommand())

	return cmd
}
