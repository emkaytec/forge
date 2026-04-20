package reconcile

import (
	"errors"
	"fmt"

	"github.com/emkaytec/forge/pkg/schema"
)

// Action is the intended mutation for a resource.
type Action string

const (
	ActionNoOp   Action = "no-op"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

// DriftField captures a single field mismatch between desired and
// observed state. Path uses dotted spec notation (e.g. "spec.command").
type DriftField struct {
	Path     string
	Desired  string
	Observed string
}

// ResourceChange is a planned change for one manifest.
type ResourceChange struct {
	Source     string
	Manifest   *schema.Manifest
	Action     Action
	Drift      []DriftField
	Note       string
	SkipReason string
}

// Kind returns the manifest kind for the change.
func (c ResourceChange) Kind() schema.Kind {
	if c.Manifest == nil {
		return ""
	}
	return c.Manifest.Kind
}

// Name returns the manifest metadata.name for the change.
func (c ResourceChange) Name() string {
	if c.Manifest == nil {
		return ""
	}
	return c.Manifest.Metadata.Name
}

// LoadError is a per-file failure surfaced by the planner. It is kept
// separate from ResourceChange so callers can distinguish "plan built
// with issues" from "execution failed".
type LoadError struct {
	Source string
	Err    error
}

func (e *LoadError) Error() string {
	return fmt.Sprintf("%s: %s", e.Source, e.Err.Error())
}

func (e *LoadError) Unwrap() error { return e.Err }

// Plan is the result of the shared front half.
type Plan struct {
	Target     Target
	Changes    []ResourceChange
	Skipped    []ResourceChange
	LoadErrors []LoadError
}

// HasBlockingErrors reports whether the plan has load errors that
// prevent meaningful execution.
func (p *Plan) HasBlockingErrors() bool {
	return len(p.LoadErrors) > 0
}

// ApplyOptions configures how an Executor applies a Plan.
type ApplyOptions struct {
	// DryRun reports the plan without mutating live state.
	DryRun bool
	// Strict rejects plans that contain skipped manifests.
	Strict bool
}

// ApplyResult summarises the outcome of Executor.Apply.
type ApplyResult struct {
	Target   Target
	Applied  []ResourceChange
	Failed   []FailedChange
	Skipped  []ResourceChange
	DryRun   bool
	Strict   bool
}

// FailedChange is a ResourceChange that a handler returned an error for.
type FailedChange struct {
	Change ResourceChange
	Err    error
}

// ErrStrictSkipped is returned by Apply when Strict is true and the
// plan contains skipped manifests.
var ErrStrictSkipped = errors.New("reconcile: strict mode rejects plans with skipped manifests")

// ErrNotImplemented is returned by per-kind handlers that have not
// yet been wired to a live backend. Callers may test with errors.Is.
var ErrNotImplemented = errors.New("reconcile: handler not yet implemented")
