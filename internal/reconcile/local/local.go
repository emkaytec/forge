// Package local hosts the forge reconcile local executor and the
// workstation-only handlers behind it.
package local

import (
	"context"
	"fmt"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/internal/reconcile/local/launchagent"
	"github.com/emkaytec/forge/pkg/schema"
)

// Handler is the per-kind contract local dispatch routes to.
type Handler interface {
	Kind() schema.Kind
	DescribeChange(ctx context.Context, m *schema.Manifest, source string) (reconcile.ResourceChange, error)
	Apply(ctx context.Context, change reconcile.ResourceChange, opts reconcile.ApplyOptions) error
}

// Executor routes local-capable manifests to their per-kind Handler.
type Executor struct {
	handlers map[schema.Kind]Handler
}

// NewExecutor returns a local executor wired with the built-in
// LaunchAgent handler. No init side effects.
func NewExecutor() (*Executor, error) {
	la, err := launchagent.New()
	if err != nil {
		return nil, err
	}

	return newExecutor(la), nil
}

func newExecutor(handlers ...Handler) *Executor {
	m := make(map[schema.Kind]Handler, len(handlers))
	for _, h := range handlers {
		m[h.Kind()] = h
	}
	return &Executor{handlers: m}
}

// Target reports TargetLocal.
func (e *Executor) Target() reconcile.Target { return reconcile.TargetLocal }

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

		if change.Action == reconcile.ActionNoOp {
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
		return nil, fmt.Errorf("reconcile: no local handler registered for kind %q", string(kind))
	}
	return h, nil
}
