package reconcile

import (
	"fmt"
	"sort"

	"github.com/emkaytec/forge/pkg/schema"
)

// Target identifies where a reconciliation executes.
type Target string

const (
	TargetRemote Target = "remote"
	TargetLocal  Target = "local"
)

// Validate reports whether t is a supported target.
func (t Target) Validate() error {
	switch t {
	case TargetRemote, TargetLocal:
		return nil
	default:
		return fmt.Errorf("reconcile: unsupported target %q", string(t))
	}
}

// compatibility captures which targets each manifest kind supports.
var compatibility = map[schema.Kind]map[Target]bool{
	schema.KindGitHubRepo:        {TargetRemote: true},
	schema.KindHCPTFWorkspace:    {TargetRemote: true},
	schema.KindAWSIAMProvisioner: {TargetRemote: true},
	schema.KindLaunchAgent:       {TargetLocal: true},
}

// IsCompatible reports whether the given kind is compatible with target.
func IsCompatible(kind schema.Kind, target Target) bool {
	return compatibility[kind][target]
}

// KindsForTarget returns the sorted set of kinds compatible with target.
func KindsForTarget(target Target) []schema.Kind {
	var kinds []schema.Kind
	for kind, targets := range compatibility {
		if targets[target] {
			kinds = append(kinds, kind)
		}
	}

	sort.Slice(kinds, func(i, j int) bool {
		return kinds[i] < kinds[j]
	})

	return kinds
}

// TargetsForKind returns the sorted set of targets a kind can run on.
func TargetsForKind(kind schema.Kind) []Target {
	var targets []Target
	for target, ok := range compatibility[kind] {
		if ok {
			targets = append(targets, target)
		}
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i] < targets[j]
	})

	return targets
}
