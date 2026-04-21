package reconcilecmd

import (
	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/spf13/cobra"
)

// GroupID is the cobra group that hosts reconcile subcommands in help output.
const GroupID = "reconcile"

type commandExecutor interface {
	reconcile.Executor
}

// Command returns the configured reconcile command group.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reconcile",
		Short:   "Plan and apply manifest reconciliation by execution target",
		GroupID: GroupID,
	}

	cmd.AddCommand(newLocalCommand())
	cmd.AddCommand(newRemoteCommand())

	return cmd
}
