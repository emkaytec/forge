package reconcilecmd

import (
	"fmt"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/internal/ui"
	"github.com/spf13/cobra"
)

// runReconcile drives BuildPlan + applyPlan for one target.
func runReconcile(cmd *cobra.Command, path string, opts reconcile.ApplyOptions, executor reconcile.Executor) error {
	var plan *reconcile.Plan
	spinner := ui.NewSpinner(fmt.Sprintf("Building %s reconcile plan...", executor.Target()))
	err := spinner.RunWhile(cmd.OutOrStdout(), func() error {
		builtPlan, buildErr := reconcile.BuildPlan(cmd.Context(), executor, path)
		plan = builtPlan
		return buildErr
	})
	if err != nil {
		ui.Error(cmd.ErrOrStderr(), err.Error())
		return err
	}

	return applyPlan(cmd, plan, opts, executor)
}

// applyPlan renders the plan first, then optionally applies it.
func applyPlan(cmd *cobra.Command, plan *reconcile.Plan, opts reconcile.ApplyOptions, executor reconcile.Executor) error {
	stdout := cmd.OutOrStdout()
	reconcile.RenderPlan(stdout, plan)

	if plan.HasBlockingErrors() {
		return fmt.Errorf("reconcile failed for %d manifest(s)", len(plan.LoadErrors))
	}

	if opts.Strict && len(plan.Skipped) > 0 {
		return reconcile.ErrStrictSkipped
	}

	if opts.DryRun {
		return nil
	}

	var result *reconcile.ApplyResult
	spinner := ui.NewSpinner(fmt.Sprintf("Applying %s reconcile plan...", executor.Target()))
	err := spinner.RunWhile(stdout, func() error {
		applied, applyErr := executor.Apply(cmd.Context(), plan, opts)
		result = applied
		return applyErr
	})
	if err != nil {
		ui.Error(cmd.ErrOrStderr(), err.Error())
		return err
	}
	if result == nil {
		return fmt.Errorf("reconcile: apply returned no result")
	}

	fmt.Fprintln(stdout)
	reconcile.RenderApplyResult(stdout, result)

	if len(result.Failed) > 0 {
		return fmt.Errorf("reconcile failed for %d change(s)", len(result.Failed))
	}

	return nil
}
