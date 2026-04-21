package reconcilecmd

import (
	"fmt"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/internal/reconcile/local"
	"github.com/emkaytec/forge/internal/reconcile/remote"
	"github.com/emkaytec/forge/internal/ui"
	"github.com/spf13/cobra"
)

// GroupID is the cobra group that hosts reconcile subcommands in help output.
const GroupID = "reconcile"

type commandExecutor interface {
	reconcile.Executor
}

var newLocalExecutor = func() (commandExecutor, error) {
	return local.NewExecutor()
}

var newRemoteExecutor = func() (commandExecutor, error) {
	return remote.NewExecutor(), nil
}

// Command returns the configured reconcile command group.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reconcile",
		Short:   "Plan and apply manifest reconciliation by execution target",
		GroupID: GroupID,
	}

	cmd.AddCommand(
		newTargetCommand("local", "Plan or apply local-only manifest kinds", newLocalExecutor),
		newTargetCommand("remote", "Plan or apply remote-capable manifest kinds", newRemoteExecutor),
	)

	return cmd
}

func newTargetCommand(name, short string, factory func() (commandExecutor, error)) *cobra.Command {
	var apply bool
	var strict bool

	cmd := &cobra.Command{
		Use:   name + " <file-or-dir>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor, err := factory()
			if err != nil {
				return err
			}

			var plan *reconcile.Plan
			spinner := ui.NewSpinner(fmt.Sprintf("Building %s reconcile plan...", name))
			err = spinner.RunWhile(cmd.OutOrStdout(), func() error {
				builtPlan, buildErr := reconcile.BuildPlan(cmd.Context(), executor, args[0])
				plan = builtPlan
				return buildErr
			})
			if err != nil {
				ui.Error(cmd.ErrOrStderr(), err.Error())
				return err
			}

			reconcile.RenderPlan(cmd.OutOrStdout(), plan)

			if plan.HasBlockingErrors() {
				return fmt.Errorf("reconcile plan has %d load error(s)", len(plan.LoadErrors))
			}
			if strict && len(plan.Skipped) > 0 {
				return reconcile.ErrStrictSkipped
			}
			if !apply {
				return nil
			}

			var result *reconcile.ApplyResult
			spinner = ui.NewSpinner(fmt.Sprintf("Applying %s reconcile plan...", name))
			err = spinner.RunWhile(cmd.OutOrStdout(), func() error {
				applied, applyErr := executor.Apply(cmd.Context(), plan, reconcile.ApplyOptions{
					DryRun: false,
					Strict: strict,
				})
				result = applied
				return applyErr
			})
			if err != nil {
				ui.Error(cmd.ErrOrStderr(), err.Error())
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout())
			reconcile.RenderApplyResult(cmd.OutOrStdout(), result)

			if len(result.Failed) > 0 {
				return fmt.Errorf("reconcile apply failed for %d change(s)", len(result.Failed))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the planned changes instead of running a dry plan")
	cmd.Flags().BoolVar(&strict, "strict", false, "Fail when manifests incompatible with the chosen target are present")

	return cmd
}
