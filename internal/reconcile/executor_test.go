package reconcile_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

// fakeExecutor is a minimal Executor used to exercise the shared
// planner without pulling in the real local/remote packages.
type fakeExecutor struct {
	target    reconcile.Target
	describe  func(*schema.Manifest) (reconcile.ResourceChange, error)
	applyErr  map[string]error
}

func (f *fakeExecutor) Target() reconcile.Target { return f.target }

func (f *fakeExecutor) DescribeChange(_ context.Context, m *schema.Manifest, source string) (reconcile.ResourceChange, error) {
	if f.describe != nil {
		return f.describe(m)
	}
	return reconcile.ResourceChange{Action: reconcile.ActionNoOp}, nil
}

func (f *fakeExecutor) Apply(_ context.Context, plan *reconcile.Plan, opts reconcile.ApplyOptions) (*reconcile.ApplyResult, error) {
	if opts.Strict && len(plan.Skipped) > 0 {
		return nil, reconcile.ErrStrictSkipped
	}
	result := &reconcile.ApplyResult{
		Target:  f.target,
		DryRun:  opts.DryRun,
		Strict:  opts.Strict,
		Skipped: plan.Skipped,
	}
	for _, c := range plan.Changes {
		if err, ok := f.applyErr[c.Name()]; ok && err != nil {
			result.Failed = append(result.Failed, reconcile.FailedChange{Change: c, Err: err})
			continue
		}
		result.Applied = append(result.Applied, c)
	}
	return result, nil
}

const (
	githubRepoManifest = `apiVersion: forge/v1
kind: github-repo
metadata:
  name: sample
spec:
  name: sample
  visibility: private
`
	launchAgentManifest = `apiVersion: forge/v1
kind: launch-agent
metadata:
  name: brew-update
spec:
  name: brew-update
  label: com.emkaytec.brew-update
  command: /usr/bin/true
  schedule:
    type: interval
    interval_seconds: 3600
`
	invalidManifest = `apiVersion: forge/v1
kind: github-repo
metadata:
  name: broken
spec:
  visibility: private
`
)

func writeManifest(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPlanFiltersByTarget_Local(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "repo.yaml", githubRepoManifest)
	writeManifest(t, dir, "agent.yaml", launchAgentManifest)

	exec := &fakeExecutor{target: reconcile.TargetLocal}
	plan, err := reconcile.BuildPlan(context.Background(), exec, dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(plan.Changes) != 1 || plan.Changes[0].Kind() != schema.KindLaunchAgent {
		t.Fatalf("local plan changes: want [launch-agent], got %v", kinds(plan.Changes))
	}

	if len(plan.Skipped) != 1 || plan.Skipped[0].Kind() != schema.KindGitHubRepo {
		t.Fatalf("local plan skipped: want [github-repo], got %v", kinds(plan.Skipped))
	}

	if plan.Skipped[0].SkipReason == "" {
		t.Fatal("skipped change missing SkipReason")
	}
}

func TestPlanFiltersByTarget_Remote(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "repo.yaml", githubRepoManifest)
	writeManifest(t, dir, "agent.yaml", launchAgentManifest)

	exec := &fakeExecutor{target: reconcile.TargetRemote}
	plan, err := reconcile.BuildPlan(context.Background(), exec, dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(plan.Changes) != 1 || plan.Changes[0].Kind() != schema.KindGitHubRepo {
		t.Fatalf("remote plan changes: want [github-repo], got %v", kinds(plan.Changes))
	}

	if len(plan.Skipped) != 1 || plan.Skipped[0].Kind() != schema.KindLaunchAgent {
		t.Fatalf("remote plan skipped: want [launch-agent], got %v", kinds(plan.Skipped))
	}
}

func TestPlanCollectsLoadErrors(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "agent.yaml", launchAgentManifest)
	writeManifest(t, dir, "broken.yaml", invalidManifest)

	exec := &fakeExecutor{target: reconcile.TargetLocal}
	plan, err := reconcile.BuildPlan(context.Background(), exec, dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(plan.LoadErrors) != 1 {
		t.Fatalf("want 1 load error, got %d: %+v", len(plan.LoadErrors), plan.LoadErrors)
	}

	if len(plan.Changes) != 1 {
		t.Fatalf("want 1 change, got %d", len(plan.Changes))
	}
}

func TestPlanRequiresValidTarget(t *testing.T) {
	exec := &fakeExecutor{target: reconcile.Target("nope")}
	_, err := reconcile.BuildPlan(context.Background(), exec, t.TempDir())
	if err == nil {
		t.Fatal("want error for invalid target, got nil")
	}
}

func TestApplyStrictRejectsSkipped(t *testing.T) {
	exec := &fakeExecutor{target: reconcile.TargetLocal}
	plan := &reconcile.Plan{
		Target: reconcile.TargetLocal,
		Skipped: []reconcile.ResourceChange{
			{Action: reconcile.ActionNoOp, SkipReason: "not compatible"},
		},
	}

	_, err := exec.Apply(context.Background(), plan, reconcile.ApplyOptions{Strict: true})
	if !errors.Is(err, reconcile.ErrStrictSkipped) {
		t.Fatalf("want ErrStrictSkipped, got %v", err)
	}
}

func kinds(changes []reconcile.ResourceChange) []schema.Kind {
	out := make([]schema.Kind, 0, len(changes))
	for _, c := range changes {
		out = append(out, c.Kind())
	}
	return out
}
