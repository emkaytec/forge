package remote_test

import (
	"context"
	"errors"
	"testing"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/internal/reconcile/remote"
	"github.com/emkaytec/forge/pkg/schema"
)

func TestRemoteExecutorReportsTarget(t *testing.T) {
	exec := remote.NewExecutor()
	if exec.Target() != reconcile.TargetRemote {
		t.Fatalf("want TargetRemote, got %q", exec.Target())
	}
}

func TestRemoteExecutorStubsReturnNoOp(t *testing.T) {
	exec := remote.NewExecutor()

	cases := []schema.Kind{
		schema.KindGitHubRepo,
		schema.KindHCPTFWorkspace,
		schema.KindAWSIAMProvisioner,
	}

	for _, kind := range cases {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			m := &schema.Manifest{Kind: kind, Metadata: schema.Metadata{Name: "x"}}
			change, err := exec.DescribeChange(context.Background(), m, "x.yaml")
			if err != nil {
				t.Fatal(err)
			}
			if change.Action != reconcile.ActionNoOp {
				t.Fatalf("want ActionNoOp, got %q", change.Action)
			}
			if change.Note == "" {
				t.Fatal("stub handler did not set a note")
			}
		})
	}
}

func TestRemoteExecutorApplyReturnsNotImplemented(t *testing.T) {
	exec := remote.NewExecutor()
	plan := &reconcile.Plan{
		Target: reconcile.TargetRemote,
		Changes: []reconcile.ResourceChange{{
			Manifest: &schema.Manifest{Kind: schema.KindGitHubRepo, Metadata: schema.Metadata{Name: "sample"}},
			Action:   reconcile.ActionCreate,
		}},
	}

	result, err := exec.Apply(context.Background(), plan, reconcile.ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Failed) != 1 {
		t.Fatalf("want 1 failed change, got %d", len(result.Failed))
	}
	if !errors.Is(result.Failed[0].Err, reconcile.ErrNotImplemented) {
		t.Fatalf("want ErrNotImplemented, got %v", result.Failed[0].Err)
	}
}

func TestRemoteExecutorDryRunSkipsApply(t *testing.T) {
	exec := remote.NewExecutor()
	plan := &reconcile.Plan{
		Target: reconcile.TargetRemote,
		Changes: []reconcile.ResourceChange{{
			Manifest: &schema.Manifest{Kind: schema.KindGitHubRepo, Metadata: schema.Metadata{Name: "sample"}},
			Action:   reconcile.ActionCreate,
		}},
	}

	result, err := exec.Apply(context.Background(), plan, reconcile.ApplyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 1 || len(result.Failed) != 0 {
		t.Fatalf("dry run should succeed without calling handlers: %+v", result)
	}
}
