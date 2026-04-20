package reconcile

import (
	"context"
	"fmt"

	"github.com/emkaytec/forge/pkg/schema"
)

// Planner is the per-target contract that turns a validated manifest
// into a ResourceChange. The shared planner calls this during the
// filter / plan phase.
type Planner interface {
	// Target reports which execution target this planner serves.
	Target() Target
	// DescribeChange inspects the manifest (and, for local kinds,
	// the live filesystem) and returns the planned change.
	DescribeChange(ctx context.Context, m *schema.Manifest, source string) (ResourceChange, error)
}

// Executor applies a plan for a specific target.
type Executor interface {
	Planner
	// Apply executes the compatible changes in plan. Implementations
	// honour ApplyOptions.DryRun and ApplyOptions.Strict.
	Apply(ctx context.Context, plan *Plan, opts ApplyOptions) (*ApplyResult, error)
}

// BuildPlan runs the shared front half: discover files, decode +
// validate each manifest, filter by target compatibility, then hand
// compatible manifests to the executor's DescribeChange.
//
// Per-file decode / validate failures are collected in Plan.LoadErrors
// rather than short-circuiting, so operators see every issue at once.
// A compatible kind whose DescribeChange returns an error is also
// recorded as a LoadError tagged with its source path.
func BuildPlan(ctx context.Context, executor Executor, path string) (*Plan, error) {
	if executor == nil {
		return nil, fmt.Errorf("reconcile: executor is required")
	}

	target := executor.Target()
	if err := target.Validate(); err != nil {
		return nil, err
	}

	paths, err := DiscoverManifests(path)
	if err != nil {
		return nil, err
	}

	plan := &Plan{Target: target}

	for _, source := range paths {
		manifest, loadErr := loadManifest(source)
		if loadErr != nil {
			plan.LoadErrors = append(plan.LoadErrors, LoadError{Source: source, Err: loadErr})
			continue
		}

		if !IsCompatible(manifest.Kind, target) {
			plan.Skipped = append(plan.Skipped, ResourceChange{
				Source:     source,
				Manifest:   manifest,
				Action:     ActionNoOp,
				SkipReason: incompatibleReason(manifest.Kind, target),
			})
			continue
		}

		change, err := executor.DescribeChange(ctx, manifest, source)
		if err != nil {
			plan.LoadErrors = append(plan.LoadErrors, LoadError{Source: source, Err: err})
			continue
		}

		change.Source = source
		change.Manifest = manifest
		plan.Changes = append(plan.Changes, change)
	}

	return plan, nil
}

func incompatibleReason(kind schema.Kind, target Target) string {
	valid := TargetsForKind(kind)
	if len(valid) == 0 {
		return fmt.Sprintf("kind %q has no compatible execution target", string(kind))
	}

	names := make([]string, 0, len(valid))
	for _, t := range valid {
		names = append(names, string(t))
	}

	return fmt.Sprintf("kind %q is not compatible with target %q (valid: %s)", string(kind), string(target), joinQuoted(names))
}

func joinQuoted(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("%q", values[0])
	}

	out := ""
	for i, v := range values {
		if i > 0 {
			if i == len(values)-1 {
				out += " and "
			} else {
				out += ", "
			}
		}
		out += fmt.Sprintf("%q", v)
	}

	return out
}
