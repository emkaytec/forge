package remote

import (
	"context"
	"errors"
	"testing"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

type fakeHandler struct {
	kind         schema.Kind
	change       reconcile.ResourceChange
	describeErr  error
	applyErr     error
	describeSeen int
	applySeen    int
}

func (f *fakeHandler) Kind() schema.Kind { return f.kind }

func (f *fakeHandler) DescribeChange(context.Context, *schema.Manifest, string) (reconcile.ResourceChange, error) {
	f.describeSeen++
	return f.change, f.describeErr
}

func (f *fakeHandler) Apply(context.Context, reconcile.ResourceChange, reconcile.ApplyOptions) error {
	f.applySeen++
	return f.applyErr
}

func TestRemoteExecutorReportsTarget(t *testing.T) {
	exec := NewExecutor()
	if exec.Target() != reconcile.TargetRemote {
		t.Fatalf("want TargetRemote, got %q", exec.Target())
	}
}

func TestRemoteExecutorDescribeRoutesToHandler(t *testing.T) {
	handler := &fakeHandler{
		kind:   schema.KindGitHubRepo,
		change: reconcile.ResourceChange{Action: reconcile.ActionUpdate},
	}
	exec := newExecutor(handler)

	change, err := exec.DescribeChange(context.Background(), &schema.Manifest{Kind: schema.KindGitHubRepo}, "x.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if change.Action != reconcile.ActionUpdate {
		t.Fatalf("want ActionUpdate, got %q", change.Action)
	}
	if handler.describeSeen != 1 {
		t.Fatalf("expected one DescribeChange call, got %d", handler.describeSeen)
	}
}

func TestRemoteExecutorApplyCollectsHandlerFailure(t *testing.T) {
	handler := &fakeHandler{
		kind:     schema.KindGitHubRepo,
		applyErr: errors.New("boom"),
	}
	exec := newExecutor(handler)
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
	if !errors.Is(result.Failed[0].Err, handler.applyErr) {
		t.Fatalf("want handler error, got %v", result.Failed[0].Err)
	}
}

func TestRemoteExecutorDryRunSkipsApply(t *testing.T) {
	handler := &fakeHandler{kind: schema.KindGitHubRepo}
	exec := newExecutor(handler)
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
	if handler.applySeen != 0 {
		t.Fatalf("expected dry run to skip handler.Apply, got %d calls", handler.applySeen)
	}
}
