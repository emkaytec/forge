// Package remote hosts the forge reconcile remote executor and the
// per-kind handler seams that MK-14 will replace with real anvil
// delegation.
package remote

import (
	"context"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

// Handler is the per-kind contract remote dispatch routes to.
//
// Implementations should be stateless; the dispatcher creates at most
// one instance per kind at NewExecutor time.
type Handler interface {
	// Kind reports which manifest kind this handler serves.
	Kind() schema.Kind
	// DescribeChange inspects the manifest and returns the planned
	// change. Remote handlers talk to their respective APIs to
	// compute drift; today every built-in remote handler is a stub
	// that reports ActionNoOp and defers the real work.
	DescribeChange(ctx context.Context, m *schema.Manifest, source string) (reconcile.ResourceChange, error)
	// Apply executes the change. Stubs return reconcile.ErrNotImplemented.
	Apply(ctx context.Context, change reconcile.ResourceChange, opts reconcile.ApplyOptions) error
}
