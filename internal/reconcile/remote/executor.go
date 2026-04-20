package remote

import (
	"context"
	"fmt"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/internal/reconcile/remote/awsiamprovisioner"
	"github.com/emkaytec/forge/internal/reconcile/remote/githubrepo"
	"github.com/emkaytec/forge/internal/reconcile/remote/hcptfworkspace"
	"github.com/emkaytec/forge/pkg/schema"
)

// Executor routes remote-capable manifests to their per-kind Handler.
type Executor struct {
	handlers map[schema.Kind]Handler
}

// NewExecutor returns a remote executor wired with the built-in stub
// handlers. No init side effects — the CLI shell in reconcilecmd
// constructs this explicitly during command wiring.
func NewExecutor() *Executor {
	return newExecutor(
		githubrepo.New(),
		hcptfworkspace.New(),
		awsiamprovisioner.New(),
	)
}

func newExecutor(handlers ...Handler) *Executor {
	m := make(map[schema.Kind]Handler, len(handlers))
	for _, h := range handlers {
		m[h.Kind()] = h
	}
	return &Executor{handlers: m}
}

// Target reports TargetRemote.
func (e *Executor) Target() reconcile.Target { return reconcile.TargetRemote }

// DescribeChange routes to the per-kind handler.
func (e *Executor) DescribeChange(ctx context.Context, m *schema.Manifest, source string) (reconcile.ResourceChange, error) {
	handler, err := e.handlerFor(m.Kind)
	if err != nil {
		return reconcile.ResourceChange{}, err
	}

	return handler.DescribeChange(ctx, m, source)
}

// Apply executes the compatible changes in plan.
func (e *Executor) Apply(ctx context.Context, plan *reconcile.Plan, opts reconcile.ApplyOptions) (*reconcile.ApplyResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("reconcile: plan is required")
	}

	if opts.Strict && len(plan.Skipped) > 0 {
		return nil, reconcile.ErrStrictSkipped
	}

	result := &reconcile.ApplyResult{
		Target:  e.Target(),
		DryRun:  opts.DryRun,
		Strict:  opts.Strict,
		Skipped: plan.Skipped,
	}

	for _, change := range plan.Changes {
		if opts.DryRun {
			result.Applied = append(result.Applied, change)
			continue
		}

		handler, err := e.handlerFor(change.Kind())
		if err != nil {
			result.Failed = append(result.Failed, reconcile.FailedChange{Change: change, Err: err})
			continue
		}

		if err := handler.Apply(ctx, change, opts); err != nil {
			result.Failed = append(result.Failed, reconcile.FailedChange{Change: change, Err: err})
			continue
		}

		result.Applied = append(result.Applied, change)
	}

	return result, nil
}

func (e *Executor) handlerFor(kind schema.Kind) (Handler, error) {
	h, ok := e.handlers[kind]
	if !ok {
		return nil, fmt.Errorf("reconcile: no remote handler registered for kind %q", string(kind))
	}
	return h, nil
}
