package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emkaytec/forge/internal/reconcile"
)

const (
	launchAgentManifest = `apiVersion: forge/v1
kind: LaunchAgent
metadata:
  name: brew-update
spec:
  name: brew-update
  label: dev.emkaytec.brew-update
  command: /opt/homebrew/bin/brew update
  schedule:
    type: interval
    interval_seconds: 86400
`
	githubRepoManifest = `apiVersion: forge/v1
kind: GitHubRepository
metadata:
  name: sample-repo
spec:
  name: sample-repo
  visibility: private
`
)

// isolateHome points HOME at a temp dir so the launchagent handler does
// not touch the real ~/Library/LaunchAgents during tests.
func isolateHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func writeManifestFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", name, err)
	}
	return path
}

func TestRunReconcileLocalDryRunRendersPlan(t *testing.T) {
	isolateHome(t)

	dir := t.TempDir()
	writeManifestFile(t, dir, "agent.yaml", launchAgentManifest)
	writeManifestFile(t, dir, "repo.yaml", githubRepoManifest)

	var stdout, stderr bytes.Buffer
	if err := Run([]string{"reconcile", "local", "--dry-run", dir}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Plan (local)") {
		t.Fatalf("expected plan heading, got %q", out)
	}
	if !strings.Contains(out, "LaunchAgent brew-update") {
		t.Fatalf("expected LaunchAgent in plan, got %q", out)
	}
	if !strings.Contains(out, "Skipped") || !strings.Contains(out, "GitHubRepository sample-repo") {
		t.Fatalf("expected GitHubRepository in skipped section, got %q", out)
	}
}

func TestRunReconcileLocalStrictRejectsSkipped(t *testing.T) {
	isolateHome(t)

	dir := t.TempDir()
	writeManifestFile(t, dir, "agent.yaml", launchAgentManifest)
	writeManifestFile(t, dir, "repo.yaml", githubRepoManifest)

	var stdout, stderr bytes.Buffer
	err := Run([]string{"reconcile", "local", "--strict", "--dry-run", dir}, &stdout, &stderr, "dev")
	if !errors.Is(err, reconcile.ErrStrictSkipped) {
		t.Fatalf("want ErrStrictSkipped, got %v", err)
	}
}

func TestRunReconcileRemoteHelpShowsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"reconcile", "remote", "--help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "forge reconcile remote <path>") {
		t.Fatalf("expected remote usage path, got %q", out)
	}
	if !strings.Contains(out, "--apply") {
		t.Fatalf("expected remote apply flag in help output, got %q", out)
	}
}

func TestRunReconcileReportsBlockingLoadErrors(t *testing.T) {
	isolateHome(t)

	dir := t.TempDir()
	writeManifestFile(t, dir, "broken.yaml", "apiVersion: forge/v1\nkind: LaunchAgent\nmetadata: {}\nspec: {}\n")

	var stdout, stderr bytes.Buffer
	err := Run([]string{"reconcile", "local", "--dry-run", dir}, &stdout, &stderr, "dev")
	if err == nil {
		t.Fatal("expected error for malformed manifest")
	}
	if !strings.Contains(err.Error(), "1 manifest") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunReconcileRequiresPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run([]string{"reconcile", "local"}, &stdout, &stderr, "dev")
	if err == nil {
		t.Fatal("expected error when <path> is omitted")
	}
}

func TestRunReconcileShowsLocalAndRemoteSubcommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"reconcile"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "forge reconcile [command]") {
		t.Fatalf("expected reconcile usage path, got %q", out)
	}
	if !strings.Contains(out, "local") || !strings.Contains(out, "remote") {
		t.Fatalf("expected local and remote subcommands, got %q", out)
	}
}
