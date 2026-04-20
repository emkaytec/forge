package reconcilecmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
	"github.com/spf13/cobra"
)

type fakeExecutor struct {
	target    reconcile.Target
	applyFunc func(plan *reconcile.Plan, opts reconcile.ApplyOptions) (*reconcile.ApplyResult, error)
	called    bool
}

func (f *fakeExecutor) Target() reconcile.Target { return f.target }

func (f *fakeExecutor) DescribeChange(_ context.Context, _ *schema.Manifest, _ string) (reconcile.ResourceChange, error) {
	return reconcile.ResourceChange{Action: reconcile.ActionNoOp}, nil
}

func (f *fakeExecutor) Apply(_ context.Context, plan *reconcile.Plan, opts reconcile.ApplyOptions) (*reconcile.ApplyResult, error) {
	f.called = true
	if f.applyFunc != nil {
		return f.applyFunc(plan, opts)
	}
	return &reconcile.ApplyResult{
		Target:  f.target,
		DryRun:  opts.DryRun,
		Strict:  opts.Strict,
		Applied: plan.Changes,
		Skipped: plan.Skipped,
	}, nil
}

func newTestCommand() (*cobra.Command, *strings.Builder) {
	var buf strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	return cmd, &buf
}

func TestApplyPlanDryRunShortCircuitsBeforeApply(t *testing.T) {
	exec := &fakeExecutor{target: reconcile.TargetLocal}
	plan := &reconcile.Plan{
		Target:  reconcile.TargetLocal,
		Changes: []reconcile.ResourceChange{{Action: reconcile.ActionCreate}},
	}
	cmd, buf := newTestCommand()

	if err := applyPlan(cmd, plan, reconcile.ApplyOptions{DryRun: true}, exec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.called {
		t.Fatal("Apply should not run during dry-run")
	}
	if !strings.Contains(buf.String(), "Plan (local)") {
		t.Fatalf("expected plan heading, got %q", buf.String())
	}
}

func TestApplyPlanBlockingErrorsFailBeforeApply(t *testing.T) {
	exec := &fakeExecutor{target: reconcile.TargetLocal}
	plan := &reconcile.Plan{
		Target: reconcile.TargetLocal,
		LoadErrors: []reconcile.LoadError{
			{Source: "bad.yaml", Err: errors.New("boom")},
		},
	}
	cmd, _ := newTestCommand()

	err := applyPlan(cmd, plan, reconcile.ApplyOptions{}, exec)
	if err == nil {
		t.Fatal("expected error for blocking load errors")
	}
	if !strings.Contains(err.Error(), "1 manifest") {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.called {
		t.Fatal("Apply should not run when plan has blocking errors")
	}
}

func TestApplyPlanStrictRejectsSkipped(t *testing.T) {
	exec := &fakeExecutor{target: reconcile.TargetLocal}
	plan := &reconcile.Plan{
		Target: reconcile.TargetLocal,
		Skipped: []reconcile.ResourceChange{
			{Action: reconcile.ActionNoOp, SkipReason: "wrong target"},
		},
	}
	cmd, _ := newTestCommand()

	err := applyPlan(cmd, plan, reconcile.ApplyOptions{Strict: true}, exec)
	if !errors.Is(err, reconcile.ErrStrictSkipped) {
		t.Fatalf("want ErrStrictSkipped, got %v", err)
	}
	if exec.called {
		t.Fatal("Apply should not run when strict rejects skipped manifests")
	}
}

func TestApplyPlanAppliesAndRendersResult(t *testing.T) {
	exec := &fakeExecutor{target: reconcile.TargetLocal}
	plan := &reconcile.Plan{
		Target:  reconcile.TargetLocal,
		Changes: []reconcile.ResourceChange{{Action: reconcile.ActionCreate}},
	}
	cmd, buf := newTestCommand()

	if err := applyPlan(cmd, plan, reconcile.ApplyOptions{}, exec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exec.called {
		t.Fatal("Apply should run when neither dry-run nor strict-skipped")
	}
	if !strings.Contains(buf.String(), "Applied (local)") {
		t.Fatalf("expected applied heading, got %q", buf.String())
	}
}

func TestApplyPlanSurfacesFailedChanges(t *testing.T) {
	exec := &fakeExecutor{
		target: reconcile.TargetRemote,
		applyFunc: func(plan *reconcile.Plan, _ reconcile.ApplyOptions) (*reconcile.ApplyResult, error) {
			return &reconcile.ApplyResult{
				Target: reconcile.TargetRemote,
				Failed: []reconcile.FailedChange{
					{Change: plan.Changes[0], Err: reconcile.ErrNotImplemented},
				},
			}, nil
		},
	}
	plan := &reconcile.Plan{
		Target:  reconcile.TargetRemote,
		Changes: []reconcile.ResourceChange{{Action: reconcile.ActionCreate}},
	}
	cmd, _ := newTestCommand()

	err := applyPlan(cmd, plan, reconcile.ApplyOptions{}, exec)
	if err == nil {
		t.Fatal("expected non-zero exit when changes fail")
	}
	if !strings.Contains(err.Error(), "1 change") {
		t.Fatalf("unexpected error: %v", err)
	}
}
