package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunManifestGenerateGitHubRepoWritesAnvilManifest(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{
		"manifest", "generate", "github-repo", "sample-repo",
		"--visibility", "private",
		"--description", "Sample repository.",
		"--topic", "terraform",
		"--default-branch", "main",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	path := filepath.Join(tempDir, ".forge", "sample-repo.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	for _, snippet := range []string{
		"apiVersion: anvil.emkaytec.dev/v1alpha1",
		"kind: GitHubRepository",
		"metadata:",
		"  name: sample-repo",
		"spec:",
		"  repository:",
		"    name: sample-repo",
		"    description: Sample repository.",
		"    visibility: private",
		"    topics:",
		"      - terraform",
		"    autoInit: true",
		"    defaultBranch: main",
		"    features:",
		"      hasIssues: true",
		"    mergePolicy:",
		"      allowSquashMerge: true",
	} {
		if !strings.Contains(contents, snippet) {
			t.Fatalf("generated manifest did not contain %q: %q", snippet, contents)
		}
	}

	if strings.Contains(contents, "createTerraformWorkspaces") {
		t.Fatalf("non-Terraform repo should not include Terraform workspace settings: %q", contents)
	}

	if !strings.Contains(stdout.String(), path) {
		t.Fatalf("expected success output to mention %q, got %q", path, stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestGenerateGitHubRepoWritesTerraformInputs(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{
		"manifest", "generate", "github-repo", "complete-service",
		"--terraform",
		"--environment", "admin",
		"--account-id", "123456789012",
		"--project-name", "platform",
		"--terraform-version", "1.14.8",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	path := filepath.Join(tempDir, ".forge", "complete-service.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	for _, snippet := range []string{
		"  createTerraformWorkspaces: true",
		"  aws:",
		"    region: us-east-1",
		"  environments:",
		"    admin:",
		"      aws:",
		"        accountId: \"123456789012\"",
		"  workspace:",
		"    projectName: platform",
		"    executionMode: remote",
		"    terraformVersion: 1.14.8",
	} {
		if !strings.Contains(contents, snippet) {
			t.Fatalf("generated manifest did not contain %q: %q", snippet, contents)
		}
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestManifestGeneratePromptsForMVPFields(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	configPath := filepath.Join(tempDir, "aws-config")
	if err := os.WriteFile(configPath, []byte(`
[profile admin]
sso_account_id = 123456789012
`), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	t.Setenv("AWS_CONFIG_FILE", configPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(tempDir, "missing-credentials"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := newRootCommand(&stdout, &stderr, "dev")
	root.SetIn(strings.NewReader(strings.Join([]string{
		"complete-service",
		"1",
		"1",
		"1",
	}, "\n") + "\n"))
	root.SetArgs([]string{"manifest", "generate", "github-repo"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	path := filepath.Join(tempDir, ".forge", "complete-service.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	for _, snippet := range []string{
		"Repository name:",
		"Is this a Terraform repo?",
		"Environment:",
		"AWS account:",
	} {
		if !strings.Contains(stdout.String(), snippet) {
			t.Fatalf("expected prompt %q, got %q", snippet, stdout.String())
		}
	}

	if !strings.Contains(contents, "accountId: \"123456789012\"") {
		t.Fatalf("generated manifest did not contain selected AWS account: %q", contents)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestGenerateWritesStarterManifestInRelativeDirectory(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{
		"manifest", "generate", "github-repo", "sample-repo",
		"--dir", "manifests/examples",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	path := filepath.Join(tempDir, "manifests", "examples", ".forge", "sample-repo.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected generated manifest at %q: %v", path, err)
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

func TestRunManifestGenerateOnlyExposesGitHubRepo(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"manifest", "generate"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "github-repo") {
		t.Fatalf("expected github-repo in generate help, got %q", out)
	}
	for _, retired := range []string{"hcp-tf-workspace", "aws-iam-provisioner", "launch-agent"} {
		if strings.Contains(out, retired) {
			t.Fatalf("did not expect retired generator %q in help output: %q", retired, out)
		}
	}
}
