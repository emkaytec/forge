package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunManifestValidateAcceptsSingleAnvilManifestFile(t *testing.T) {
	tempDir := t.TempDir()

	path := filepath.Join(tempDir, "docs-site.yaml")
	if err := os.WriteFile(path, []byte(`
apiVersion: anvil.emkaytec.dev/v1alpha1
kind: GitHubRepository
metadata:
  name: docs-site
spec:
  repository:
    visibility: public
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
	manifestsDir := filepath.Join(tempDir, ".forge")
	if err := os.MkdirAll(manifestsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	files := map[string]string{
		"docs-site.yaml": `
apiVersion: anvil.emkaytec.dev/v1alpha1
kind: GitHubRepository
metadata:
  name: docs-site
spec:
  repository:
    visibility: public
`,
		"complete-service.yml": `
apiVersion: anvil.emkaytec.dev/v1alpha1
kind: GitHubRepository
metadata:
  name: complete-service
spec:
  createTerraformWorkspaces: true
  repository:
    visibility: private
  environments:
    admin:
      aws:
        accountId: "123456789012"
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

	if !strings.Contains(stdout.String(), filepath.Join(manifestsDir, "complete-service.yml")+" is valid") {
		t.Fatalf("expected complete-service.yml success output, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), filepath.Join(manifestsDir, "docs-site.yaml")+" is valid") {
		t.Fatalf("expected docs-site.yaml success output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestValidateReportsActionableErrors(t *testing.T) {
	tempDir := t.TempDir()

	path := filepath.Join(tempDir, "broken.yaml")
	if err := os.WriteFile(path, []byte(`
apiVersion: anvil.emkaytec.dev/v1alpha1
kind: GitHubRepository
metadata:
  name: complete-service
spec:
  createTerraformWorkspaces: true
  repository:
    visibility: private
  environments:
    admin:
      aws: {}
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

	if !strings.Contains(stderr.String(), "spec.environments.admin.aws.accountId") {
		t.Fatalf("expected accountId guidance in stderr, got %q", stderr.String())
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
