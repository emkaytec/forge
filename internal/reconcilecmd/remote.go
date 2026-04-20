package reconcilecmd

import (
	"strings"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/internal/reconcile/remote"
	"github.com/spf13/cobra"
)

func newRemoteCommand() *cobra.Command {
	var (
		dryRun bool
		strict bool
	)

	cmd := &cobra.Command{
		Use:   "remote <path>",
		Short: "Reconcile cloud-capable manifests against their remote backends",
		Long: strings.TrimSpace(`Reconcile cloud-capable manifests against their remote backends.

<path> may be a single manifest file or a directory of .yaml / .yml
manifests. Local-only kinds in the path are skipped with a reason
unless --strict is set, in which case the command exits without
applying anything.

Remote kind handlers are stub seams today and report ErrNotImplemented
on apply; real delegation to anvil lands in a follow-up ticket.`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := remote.NewExecutor()

			return runReconcile(cmd, args[0], reconcile.ApplyOptions{
				DryRun: dryRun,
				Strict: strict,
			}, executor)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the plan without mutating live state")
	cmd.Flags().BoolVar(&strict, "strict", false, "Reject plans containing manifests that do not target remote")

	return cmd
}
