package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunManifestGenerateWritesStarterManifestInCurrentDirectory(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		snippets []string
	}{
		{
			name:     "github repo",
			resource: "github-repo",
			snippets: []string{"kind: github-repo", "# visibility must be either public or private.", `name: "sample-repo"`},
		},
		{
			name:     "hcp workspace",
			resource: "hcp-tf-workspace",
			snippets: []string{"kind: hcp-tf-workspace", "# execution_mode must be remote, local, or agent.", `organization: "example-org"`},
		},
		{
			name:     "aws provisioner",
			resource: "aws-iam-provisioner",
			snippets: []string{"kind: aws-iam-provisioner", "# account_id is the 12-digit AWS account identifier.", `account_id: "123456789012"`},
		},
		{
			name:     "launch agent",
			resource: "launch-agent",
			snippets: []string{"kind: launch-agent", "# type must be interval or calendar.", "interval_seconds: 86400"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			t.Chdir(tempDir)

			var stdout bytes.Buffer
			var stderr bytes.Buffer

			if err := Run([]string{"manifest", "generate", tt.resource, "sample-repo"}, &stdout, &stderr, "dev"); err != nil {
				t.Fatalf("Run returned error: %v", err)
			}

			rendered, err := os.ReadFile(filepath.Join(tempDir, "sample-repo.yaml"))
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}

			for _, snippet := range tt.snippets {
				if !strings.Contains(string(rendered), snippet) {
					t.Fatalf("generated manifest did not contain %q: %q", snippet, string(rendered))
				}
			}

			if !strings.Contains(stdout.String(), "Wrote "+tt.resource+" manifest") {
				t.Fatalf("expected success output, got %q", stdout.String())
			}

			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", stderr.String())
			}
		})
	}
}

func TestRunManifestGenerateWritesStarterManifestInRelativeDirectory(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"manifest", "generate", "launch-agent", "brew-update", "--dir", "manifests/examples"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	path := filepath.Join(tempDir, "manifests", "examples", "brew-update.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if !strings.Contains(string(rendered), `label: "dev.emkaytec.brew-update"`) {
		t.Fatalf("generated manifest did not contain expected launch-agent label: %q", string(rendered))
	}

	if !strings.Contains(stdout.String(), path) {
		t.Fatalf("expected success output to mention %q, got %q", path, stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestGenerateRejectsAbsoluteDirectory(t *testing.T) {
	tempDir := t.TempDir()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"manifest", "generate", "github-repo", "sample-repo", "--dir", tempDir}, &stdout, &stderr, "dev")
	if err == nil {
		t.Fatal("expected absolute directory error")
	}

	if !strings.Contains(err.Error(), "output directory must be relative") {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestManifestGeneratePromptsForAWSIAMProvisionerFieldsWhenNameOmitted(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := newRootCommand(&stdout, &stderr, "dev")
	root.SetIn(strings.NewReader(strings.Join([]string{
		"github-actions",
		"github-actions",
		"123456789012",
		"token.actions.githubusercontent.com",
		"repo:emkaytec/forge:ref:refs/heads/main",
		"arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, "\n") + "\n"))
	root.SetArgs([]string{"manifest", "generate", "aws-iam-provisioner"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	rendered, err := os.ReadFile(filepath.Join(tempDir, "github-actions.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	if !strings.Contains(contents, `kind: aws-iam-provisioner`) {
		t.Fatalf("generated manifest did not contain aws-iam-provisioner kind: %q", contents)
	}

	if !strings.Contains(contents, `account_id: "123456789012"`) {
		t.Fatalf("generated manifest did not contain prompted account_id: %q", contents)
	}

	if !strings.Contains(stdout.String(), "Manifest name:") {
		t.Fatalf("expected manifest name prompt, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "AWS account ID [123456789012]:") {
		t.Fatalf("expected account ID prompt, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestGenerateHelpDocumentsOptionalNameArgument(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"manifest", "generate", "aws-iam-provisioner", "--help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "forge manifest generate aws-iam-provisioner [name]") {
		t.Fatalf("expected usage with optional name argument, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "If [name] is omitted, Forge prompts for the schema fields interactively.") {
		t.Fatalf("expected interactive help text, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "forge manifest generate aws-iam-provisioner example") {
		t.Fatalf("expected example usage, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}
