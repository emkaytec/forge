package reconcilecmd

import (
	"strings"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/internal/reconcile/local"
	"github.com/spf13/cobra"
)

var newLocalExecutor = func() (commandExecutor, error) {
	return local.NewExecutor()
}

func newLocalCommand() *cobra.Command {
	var (
		apply  bool
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
applying anything.

The command prints the plan by default. Pass --apply to mutate local
state after the plan is rendered.`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor, err := newLocalExecutor()
			if err != nil {
				return err
			}

			return runReconcile(cmd, args[0], reconcile.ApplyOptions{
				DryRun: !apply || dryRun,
				Strict: strict,
			}, executor)
		},
	}

	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the planned changes instead of running a dry plan")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the plan without mutating live state")
	cmd.Flags().BoolVar(&strict, "strict", false, "Reject plans containing manifests that do not target local")
	_ = cmd.Flags().MarkHidden("dry-run")

	return cmd
}
