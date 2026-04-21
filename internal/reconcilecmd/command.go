package reconcilecmd

import (
	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/spf13/cobra"
)

type commandExecutor interface {
	reconcile.Executor
}

// Command returns the configured reconcile command group.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Plan and apply manifest reconciliation by execution target",
	}

	cmd.AddCommand(newLocalCommand())
	cmd.AddCommand(newRemoteCommand())

	return cmd
}
