package local_test

import (
	"context"
	"testing"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/internal/reconcile/local"
)

func TestLocalExecutorReportsTarget(t *testing.T) {
	exec, err := local.NewExecutor()
	if err != nil {
		t.Fatal(err)
	}

	if exec.Target() != reconcile.TargetLocal {
		t.Fatalf("want TargetLocal, got %q", exec.Target())
	}
}

func TestLocalExecutorStrictRejectsSkipped(t *testing.T) {
	exec, err := local.NewExecutor()
	if err != nil {
		t.Fatal(err)
	}

	plan := &reconcile.Plan{
		Target: reconcile.TargetLocal,
		Skipped: []reconcile.ResourceChange{
			{Action: reconcile.ActionNoOp, SkipReason: "kind not compatible"},
		},
	}

	_, err = exec.Apply(context.Background(), plan, reconcile.ApplyOptions{Strict: true})
	if err != reconcile.ErrStrictSkipped {
		t.Fatalf("want ErrStrictSkipped, got %v", err)
	}
}
