package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emkaytec/forge/internal/github"
	"github.com/emkaytec/forge/pkg/schema"
)

func TestRunManifestComposeTerraformGitHubRepoSupportsNonInteractiveFlags(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{
		"manifest", "compose", "terraform-github-repo",
		"--application", "forge-test-repo",
		"--owner", "emkaytec",
		"--visibility", "private",
		"--description", "This is a comprehensive test.",
		"--topic", "aws",
		"--topic", "terraform",
		"--default-branch", "main",
		"--environment", "dev",
		"--environment", "pre",
		"--environment", "prod",
		"--account-id", "dev=502710547484",
		"--account-id", "pre=427606711885",
		"--account-id", "prod=133124153984",
		"--organization", "emkaytec",
		"--project", "platform",
		"--execution-mode", "remote",
		"--terraform-version", "1.14.0",
		"--managed-policy", "arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	expectedFiles := []string{
		filepath.Join(tempDir, "forge-test-repo", "github-repo.yaml"),
		filepath.Join(tempDir, "forge-test-repo", "hcp-tf-workspace-dev.yml"),
		filepath.Join(tempDir, "forge-test-repo", "hcp-tf-workspace-pre.yml"),
		filepath.Join(tempDir, "forge-test-repo", "hcp-tf-workspace-prod.yml"),
		filepath.Join(tempDir, "forge-test-repo", "aws-iam-provisioner-dev-gha.yaml"),
		filepath.Join(tempDir, "forge-test-repo", "aws-iam-provisioner-dev-tfc.yaml"),
		filepath.Join(tempDir, "forge-test-repo", "aws-iam-provisioner-pre-gha.yaml"),
		filepath.Join(tempDir, "forge-test-repo", "aws-iam-provisioner-pre-tfc.yaml"),
		filepath.Join(tempDir, "forge-test-repo", "aws-iam-provisioner-prod-gha.yaml"),
		filepath.Join(tempDir, "forge-test-repo", "aws-iam-provisioner-prod-tfc.yaml"),
	}

	for _, path := range expectedFiles {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}

		if _, err := schema.DecodeManifest(contents); err != nil {
			t.Fatalf("DecodeManifest(%q) error = %v", path, err)
		}
	}

	githubRepoContents, err := os.ReadFile(expectedFiles[0])
	if err != nil {
		t.Fatalf("ReadFile(github repo) error = %v", err)
	}
	for _, snippet := range []string{
		`name: "emkaytec-forge-test-repo"`,
		`owner: "emkaytec"`,
		`name: "forge-test-repo"`,
		"visibility: private",
		`description: "This is a comprehensive test."`,
		`- "aws"`,
		`- "terraform"`,
		"default_branch: main",
	} {
		if !strings.Contains(string(githubRepoContents), snippet) {
			t.Fatalf("github repo manifest missing %q: %q", snippet, string(githubRepoContents))
		}
	}

	hcpWorkspaceContents, err := os.ReadFile(filepath.Join(tempDir, "forge-test-repo", "hcp-tf-workspace-pre.yml"))
	if err != nil {
		t.Fatalf("ReadFile(hcp workspace) error = %v", err)
	}
	for _, snippet := range []string{
		`name: "emkaytec-forge-test-repo-pre"`,
		`name: "forge-test-repo-pre"`,
		`environment: "pre"`,
		`organization: "emkaytec"`,
		`project: "platform"`,
		`account_id: "427606711885"`,
		`vcs_repo: "emkaytec/forge-test-repo"`,
		"execution_mode: remote",
		`terraform_version: "1.14.0"`,
	} {
		if !strings.Contains(string(hcpWorkspaceContents), snippet) {
			t.Fatalf("hcp workspace manifest missing %q: %q", snippet, string(hcpWorkspaceContents))
		}
	}

	awsProvisionerContents, err := os.ReadFile(filepath.Join(tempDir, "forge-test-repo", "aws-iam-provisioner-prod-tfc.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(aws provisioner) error = %v", err)
	}
	for _, snippet := range []string{
		`name: "emkaytec-forge-test-repo-prod-tfc"`,
		`name: "forge-test-repo-prod-tfc-provisioner-role"`,
		`account_id: "133124153984"`,
		`oidc_provider: "app.terraform.io"`,
		`oidc_subject: "organization:emkaytec:project:*:workspace:forge-test-repo-prod:run_phase:*"`,
		`- "arn:aws:iam::aws:policy/ReadOnlyAccess"`,
	} {
		if !strings.Contains(string(awsProvisionerContents), snippet) {
			t.Fatalf("aws provisioner manifest missing %q: %q", snippet, string(awsProvisionerContents))
		}
	}

	for _, path := range expectedFiles {
		if !strings.Contains(stdout.String(), path) {
			t.Fatalf("expected success output to mention %q, got %q", path, stdout.String())
		}
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestManifestComposeTerraformGitHubRepoPromptsForBlueprintInputs(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	configPath := filepath.Join(tempDir, "aws-config")
	if err := os.WriteFile(configPath, []byte(`
[profile emkaytec-dev]
sso_account_id = 502710547484

[profile emkaytec-pre]
sso_account_id = 427606711885

[profile emkaytec-prod]
sso_account_id = 133124153984
`), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	t.Setenv("AWS_CONFIG_FILE", configPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(tempDir, "missing-credentials"))
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	restoreGH := github.SetLookupGHTokenForTesting(func() string { return "" })
	t.Cleanup(restoreGH)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := newRootCommand(&stdout, &stderr, "dev")
	root.SetIn(strings.NewReader(strings.Join([]string{
		"forge-test-repo",
		"emkaytec",
		"1",
		"This is a comprehensive test.",
		"aws,terraform",
		"",
		"1,2,3",
		"",
		"",
		"",
		"emkaytec",
		"platform",
		"1",
		"1.14.0",
		"arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, "\n") + "\n"))
	root.SetArgs([]string{"manifest", "compose", "terraform-github-repo"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, path := range []string{
		filepath.Join(tempDir, "forge-test-repo", "github-repo.yaml"),
		filepath.Join(tempDir, "forge-test-repo", "hcp-tf-workspace-dev.yml"),
		filepath.Join(tempDir, "forge-test-repo", "hcp-tf-workspace-pre.yml"),
		filepath.Join(tempDir, "forge-test-repo", "hcp-tf-workspace-prod.yml"),
		filepath.Join(tempDir, "forge-test-repo", "aws-iam-provisioner-dev-gha.yaml"),
		filepath.Join(tempDir, "forge-test-repo", "aws-iam-provisioner-dev-tfc.yaml"),
		filepath.Join(tempDir, "forge-test-repo", "aws-iam-provisioner-pre-gha.yaml"),
		filepath.Join(tempDir, "forge-test-repo", "aws-iam-provisioner-pre-tfc.yaml"),
		filepath.Join(tempDir, "forge-test-repo", "aws-iam-provisioner-prod-gha.yaml"),
		filepath.Join(tempDir, "forge-test-repo", "aws-iam-provisioner-prod-tfc.yaml"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated file %q: %v", path, err)
		}
	}

	devWorkspaceContents, err := os.ReadFile(filepath.Join(tempDir, "forge-test-repo", "hcp-tf-workspace-dev.yml"))
	if err != nil {
		t.Fatalf("ReadFile(dev workspace) error = %v", err)
	}
	if !strings.Contains(string(devWorkspaceContents), `account_id: "502710547484"`) {
		t.Fatalf("dev workspace did not use the prioritized dev account: %q", string(devWorkspaceContents))
	}

	preWorkspaceContents, err := os.ReadFile(filepath.Join(tempDir, "forge-test-repo", "hcp-tf-workspace-pre.yml"))
	if err != nil {
		t.Fatalf("ReadFile(pre workspace) error = %v", err)
	}
	if !strings.Contains(string(preWorkspaceContents), `account_id: "427606711885"`) {
		t.Fatalf("pre workspace did not use the prioritized pre account: %q", string(preWorkspaceContents))
	}

	prodWorkspaceContents, err := os.ReadFile(filepath.Join(tempDir, "forge-test-repo", "hcp-tf-workspace-prod.yml"))
	if err != nil {
		t.Fatalf("ReadFile(prod workspace) error = %v", err)
	}
	if !strings.Contains(string(prodWorkspaceContents), `account_id: "133124153984"`) {
		t.Fatalf("prod workspace did not use the prioritized prod account: %q", string(prodWorkspaceContents))
	}

	renderedPrompts := stdout.String()
	for _, snippet := range []string{
		"Application name:",
		"Repository owner:",
		"Visibility:",
		"Environments:",
		"Development AWS account:",
		"Pre-prod AWS account:",
		"Prod AWS account:",
		"HCP TF organization",
		"HCP TF project",
		"Execution mode:",
		"Terraform version",
		"Managed policy ARNs (comma-separated)",
	} {
		if !strings.Contains(renderedPrompts, snippet) {
			t.Fatalf("expected prompt output to contain %q, got %q", snippet, renderedPrompts)
		}
	}

	if strings.Contains(renderedPrompts, "VCS repo (owner/repo):") {
		t.Fatalf("did not expect a VCS repo prompt, got %q", renderedPrompts)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestComposeHelpListsBlueprintCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"manifest", "compose", "--help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "forge manifest compose [command]") {
		t.Fatalf("expected compose usage path, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Compose higher-level Forge manifest blueprints") {
		t.Fatalf("expected compose help text, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "terraform-github-repo") || !strings.Contains(stdout.String(), "github-repo, hcp-tf-workspace, and aws-iam-provisioner") {
		t.Fatalf("expected blueprint command listing, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestComposeWithoutBlueprintShowsHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"manifest", "compose"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "forge manifest compose [command]") {
		t.Fatalf("expected compose usage path, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "terraform-github-repo") {
		t.Fatalf("expected supported blueprint in help output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestComposeHelpAliasShowsHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"manifest", "compose", "help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "forge manifest compose [command]") {
		t.Fatalf("expected compose usage path, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "terraform-github-repo") {
		t.Fatalf("expected blueprint listing, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestComposeTerraformGitHubRepoHelpDescribesCLIFlow(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"manifest", "compose", "terraform-github-repo", "--help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "forge manifest compose terraform-github-repo [application]") {
		t.Fatalf("expected blueprint usage path, got %q", stdout.String())
	}

	for _, snippet := range []string{
		"Compose a repo stack into github-repo, hcp-tf-workspace, and aws-iam-provisioner manifests",
		"the application name and repository owner",
		"<application>/github-repo.yaml",
		"--environment",
		"--account-id",
	} {
		if !strings.Contains(stdout.String(), snippet) {
			t.Fatalf("expected blueprint help output to contain %q, got %q", snippet, stdout.String())
		}
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}
