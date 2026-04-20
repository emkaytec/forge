package reconcilecmd

import (
	"strings"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/internal/reconcile/local"
	"github.com/spf13/cobra"
)

func newLocalCommand() *cobra.Command {
	var (
		dryRun bool
		strict bool
	)

	cmd := &cobra.Command{
		Use:   "local <path>",
		Short: "Reconcile workstation-local manifests against this machine",
		Long: strings.TrimSpace(`Reconcile workstation-local manifests against this machine.

<path> may be a single manifest file or a directory of .yaml / .yml
manifests. Remote-only kinds in the path are skipped with a reason
unless --strict is set, in which case the command exits without
applying anything.`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor, err := local.NewExecutor()
			if err != nil {
				return err
			}

			return runReconcile(cmd, args[0], reconcile.ApplyOptions{
				DryRun: dryRun,
				Strict: strict,
			}, executor)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the plan without mutating live state")
	cmd.Flags().BoolVar(&strict, "strict", false, "Reject plans containing manifests that do not target local")

	return cmd
}
