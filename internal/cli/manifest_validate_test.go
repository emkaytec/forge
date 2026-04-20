package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunManifestValidateAcceptsSingleManifestFile(t *testing.T) {
	tempDir := t.TempDir()

	path := filepath.Join(tempDir, "brew-update.yaml")
	if err := os.WriteFile(path, []byte(`
apiVersion: forge/v1
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
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"manifest", "validate", path}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), path+" is valid") {
		t.Fatalf("expected success output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestValidateAcceptsManifestDirectory(t *testing.T) {
	tempDir := t.TempDir()
	manifestsDir := filepath.Join(tempDir, "manifests")
	if err := os.MkdirAll(manifestsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	files := map[string]string{
		"github.yaml": `
apiVersion: forge/v1
kind: GitHubRepository
metadata:
  name: sample-repo
spec:
  name: sample-repo
  visibility: public
`,
		"workspace.yml": `
apiVersion: forge/v1
kind: HCPTerraformWorkspace
metadata:
  name: platform
spec:
  name: platform
  organization: example-org
  execution_mode: remote
`,
	}

	for name, contents := range files {
		path := filepath.Join(manifestsDir, name)
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"manifest", "validate", manifestsDir}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), filepath.Join(manifestsDir, "github.yaml")+" is valid") {
		t.Fatalf("expected github.yaml success output, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), filepath.Join(manifestsDir, "workspace.yml")+" is valid") {
		t.Fatalf("expected workspace.yml success output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestValidateReportsActionableErrors(t *testing.T) {
	tempDir := t.TempDir()

	path := filepath.Join(tempDir, "broken.yaml")
	if err := os.WriteFile(path, []byte(`
apiVersion: forge/v1
kind: AWSIAMProvisioner
metadata:
  name: github-actions
spec:
  name: github-actions
  account_id: "123456789012"
  oidc_provider: token.actions.githubusercontent.com
  oidc_subject: repo:emkaytec/forge:ref:refs/heads/main
  assume_role_policy: {}
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"manifest", "validate", path}, &stdout, &stderr, "dev")
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "validation failed for 1 manifest") {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stderr.String(), "assume_role_policy") {
		t.Fatalf("expected unsupported field in stderr, got %q", stderr.String())
	}

	if !strings.Contains(stderr.String(), "remove unknown fields or rename them to a supported schema field") {
		t.Fatalf("expected actionable guidance in stderr, got %q", stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
}

func TestRunManifestValidateRejectsEmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"manifest", "validate", tempDir}, &stdout, &stderr, "dev")
	if err == nil {
		t.Fatal("expected empty directory error")
	}

	if !strings.Contains(err.Error(), "does not contain any .yaml or .yml manifest files") {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}
