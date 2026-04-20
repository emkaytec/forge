package reconcilecmd

import (
	"fmt"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/spf13/cobra"
)

// runReconcile drives BuildPlan + applyPlan for one target.
func runReconcile(cmd *cobra.Command, path string, opts reconcile.ApplyOptions, executor reconcile.Executor) error {
	plan, err := reconcile.BuildPlan(cmd.Context(), executor, path)
	if err != nil {
		return err
	}

	return applyPlan(cmd, plan, opts, executor)
}

// applyPlan renders the outcome of a pre-built plan. Dry-run
// short-circuits after planning; strict mode rejects plans containing
// skipped manifests before Apply runs.
func applyPlan(cmd *cobra.Command, plan *reconcile.Plan, opts reconcile.ApplyOptions, executor reconcile.Executor) error {
	stdout := cmd.OutOrStdout()

	if plan.HasBlockingErrors() {
		reconcile.RenderPlan(stdout, plan)
		return fmt.Errorf("reconcile failed for %d manifest(s)", len(plan.LoadErrors))
	}

	if opts.Strict && len(plan.Skipped) > 0 {
		reconcile.RenderPlan(stdout, plan)
		return reconcile.ErrStrictSkipped
	}

	if opts.DryRun {
		reconcile.RenderPlan(stdout, plan)
		return nil
	}

	result, err := executor.Apply(cmd.Context(), plan, opts)
	if err != nil {
		return err
	}

	reconcile.RenderApplyResult(stdout, result)

	if len(result.Failed) > 0 {
		return fmt.Errorf("reconcile failed for %d change(s)", len(result.Failed))
	}

	return nil
}
