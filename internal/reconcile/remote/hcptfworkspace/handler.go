// Package hcptfworkspace hosts the remote reconcile handler for the
// HCPTerraformWorkspace kind. MK-9 ships a stub seam; MK-14 replaces it
// with real delegation to anvil.
package hcptfworkspace

import (
	"context"
	"fmt"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

// Handler implements the HCPTerraformWorkspace remote handler contract.
type Handler struct{}

// New returns a new handler.
func New() *Handler { return &Handler{} }

// Kind reports schema.KindHCPTFWorkspace.
func (h *Handler) Kind() schema.Kind { return schema.KindHCPTFWorkspace }

// DescribeChange reports ActionNoOp with a note until MK-14 wires
// this handler into the real cloud runtime.
func (h *Handler) DescribeChange(_ context.Context, m *schema.Manifest, source string) (reconcile.ResourceChange, error) {
	return reconcile.ResourceChange{
		Source:   source,
		Manifest: m,
		Action:   reconcile.ActionNoOp,
		Note:     "HCPTerraformWorkspace remote handler is a stub; real reconciliation lands with the anvil carve-out",
	}, nil
}

// Apply always returns reconcile.ErrNotImplemented wrapped with the kind.
func (h *Handler) Apply(_ context.Context, _ reconcile.ResourceChange, _ reconcile.ApplyOptions) error {
	return fmt.Errorf("HCPTerraformWorkspace: %w", reconcile.ErrNotImplemented)
}
