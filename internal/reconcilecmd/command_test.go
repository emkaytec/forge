package reconcilecmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

type fakeCommandExecutor struct {
	target     reconcile.Target
	change     reconcile.ResourceChange
	applyErr   error
	applyCalls int
}

func (f *fakeCommandExecutor) Target() reconcile.Target { return f.target }

func (f *fakeCommandExecutor) DescribeChange(context.Context, *schema.Manifest, string) (reconcile.ResourceChange, error) {
	if f.change.Action == "" {
		return reconcile.ResourceChange{Action: reconcile.ActionNoOp}, nil
	}
	return f.change, nil
}

func (f *fakeCommandExecutor) Apply(_ context.Context, plan *reconcile.Plan, opts reconcile.ApplyOptions) (*reconcile.ApplyResult, error) {
	f.applyCalls++
	if opts.Strict && len(plan.Skipped) > 0 {
		return nil, reconcile.ErrStrictSkipped
	}
	result := &reconcile.ApplyResult{
		Target:  f.target,
		DryRun:  opts.DryRun,
		Strict:  opts.Strict,
		Applied: plan.Changes,
		Skipped: plan.Skipped,
	}
	if f.applyErr != nil {
		result.Failed = append(result.Failed, reconcile.FailedChange{Change: plan.Changes[0], Err: f.applyErr})
	}
	return result, f.applyErr
}

func TestReconcileCommandPlansWithoutApply(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte(`
apiVersion: forge/v1
kind: LaunchAgent
metadata:
  name: brew-update
spec:
  name: brew-update
  label: com.emkaytec.brew-update
  command: /usr/bin/true
  schedule:
    type: interval
    interval_seconds: 3600
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	exec := &fakeCommandExecutor{target: reconcile.TargetLocal}
	previous := newLocalExecutor
	newLocalExecutor = func() (commandExecutor, error) { return exec, nil }
	defer func() { newLocalExecutor = previous }()

	cmd := Command()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"local", dir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if exec.applyCalls != 0 {
		t.Fatalf("expected no apply calls, got %d", exec.applyCalls)
	}
	if !strings.Contains(stdout.String(), "Plan (local)") {
		t.Fatalf("expected plan output, got %q", stdout.String())
	}
}

func TestReconcileCommandApplyReportsFailures(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "repo.yaml"), []byte(`
apiVersion: forge/v1
kind: GitHubRepository
metadata:
  name: sample
spec:
  name: sample
  visibility: private
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	exec := &fakeCommandExecutor{
		target:   reconcile.TargetRemote,
		change:   reconcile.ResourceChange{Action: reconcile.ActionCreate},
		applyErr: errors.New("boom"),
	}
	previous := newRemoteExecutor
	newRemoteExecutor = func() (commandExecutor, error) { return exec, nil }
	defer func() { newRemoteExecutor = previous }()

	cmd := Command()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"remote", "--apply", dir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected apply error")
	}
	if exec.applyCalls != 1 {
		t.Fatalf("expected one apply call, got %d", exec.applyCalls)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("expected stderr to include apply error, got %q", stderr.String())
	}
}
