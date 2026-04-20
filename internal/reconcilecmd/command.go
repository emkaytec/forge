package reconcilecmd

import "github.com/spf13/cobra"

// GroupID is the cobra group that hosts reconcile subcommands in help output.
const GroupID = "reconcile"

// Command returns the configured reconcile command group.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reconcile",
		Short:   "Reconcile Forge manifests against their execution targets",
		GroupID: GroupID,
	}

	cmd.AddCommand(newLocalCommand())
	cmd.AddCommand(newRemoteCommand())

	return cmd
}
