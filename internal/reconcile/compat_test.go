package reconcile_test

import (
	"testing"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

func TestIsCompatible(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind   schema.Kind
		target reconcile.Target
		want   bool
	}{
		{schema.KindGitHubRepo, reconcile.TargetRemote, true},
		{schema.KindGitHubRepo, reconcile.TargetLocal, false},
		{schema.KindHCPTFWorkspace, reconcile.TargetRemote, true},
		{schema.KindHCPTFWorkspace, reconcile.TargetLocal, false},
		{schema.KindAWSIAMProvisioner, reconcile.TargetRemote, true},
		{schema.KindAWSIAMProvisioner, reconcile.TargetLocal, false},
		{schema.KindLaunchAgent, reconcile.TargetRemote, false},
		{schema.KindLaunchAgent, reconcile.TargetLocal, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.kind)+"/"+string(tt.target), func(t *testing.T) {
			t.Parallel()

			if got := reconcile.IsCompatible(tt.kind, tt.target); got != tt.want {
				t.Fatalf("IsCompatible(%q, %q) = %t, want %t", tt.kind, tt.target, got, tt.want)
			}
		})
	}
}

func TestKindsForTarget(t *testing.T) {
	t.Parallel()

	remote := reconcile.KindsForTarget(reconcile.TargetRemote)
	if len(remote) != 3 {
		t.Fatalf("remote kinds: want 3, got %d (%v)", len(remote), remote)
	}

	local := reconcile.KindsForTarget(reconcile.TargetLocal)
	if len(local) != 1 || local[0] != schema.KindLaunchAgent {
		t.Fatalf("local kinds: want [LaunchAgent], got %v", local)
	}
}

func TestTargetValidate(t *testing.T) {
	t.Parallel()

	if err := reconcile.TargetRemote.Validate(); err != nil {
		t.Fatalf("TargetRemote.Validate: %v", err)
	}
	if err := reconcile.TargetLocal.Validate(); err != nil {
		t.Fatalf("TargetLocal.Validate: %v", err)
	}
	if err := reconcile.Target("garbage").Validate(); err == nil {
		t.Fatal("Target(garbage).Validate: want error, got nil")
	}
}
