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

func TestRunManifestGenerateWritesStarterManifestInCurrentDirectory(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		path     string
		snippets []string
	}{
		{
			name: "github repo",
			args: []string{
				"manifest", "generate", "github-repo", "sample-repo",
				"--owner", "emkaytec",
				"--visibility", "private",
				"--default-branch", "main",
			},
			path: filepath.Join(".forge", "sample-repo", "github-repo.yaml"),
			snippets: []string{
				"kind: GitHubRepository",
				"# visibility must be either public or private.",
				`name: "emkaytec-sample-repo"`,
				`owner: "emkaytec"`,
				`name: "sample-repo"`,
				"visibility: private",
				"default_branch: main",
			},
		},
		{
			name: "hcp workspace",
			args: []string{
				"manifest", "generate", "hcp-tf-workspace", "emkaytec/sample-repo",
				"--environment", "dev",
				"--account-id", "123456789012",
				"--organization", "emkaytec",
				"--project", "platform",
				"--vcs-repo", "emkaytec/sample-repo",
				"--execution-mode", "remote",
				"--terraform-version", "1.14.0",
			},
			path: filepath.Join(".forge", "sample-repo", "hcp-tf-workspace-dev.yml"),
			snippets: []string{
				"kind: HCPTerraformWorkspace",
				"# execution_mode must be remote, local, or agent.",
				`name: "emkaytec-sample-repo-dev"`,
				`name: "sample-repo-dev"`,
				`environment: "dev"`,
				`organization: "emkaytec"`,
				`account_id: "123456789012"`,
				"execution_mode: remote",
			},
		},
		{
			name: "launch agent",
			args: []string{
				"manifest", "generate", "launch-agent", "sample-repo",
				"--command", "/opt/homebrew/bin/brew update",
				"--schedule", "interval",
				"--interval-seconds", "86400",
				"--run-at-load",
			},
			path: filepath.Join(".forge", "sample-repo", "launch-agent.yaml"),
			snippets: []string{
				"kind: LaunchAgent",
				"# type must be interval or calendar.",
				`name: "sample-repo"`,
				`label: "dev.emkaytec.sample-repo"`,
				"interval_seconds: 86400",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			t.Chdir(tempDir)

			var stdout bytes.Buffer
			var stderr bytes.Buffer

			if err := Run(tt.args, &stdout, &stderr, "dev"); err != nil {
				t.Fatalf("Run returned error: %v", err)
			}

			rendered, err := os.ReadFile(filepath.Join(tempDir, tt.path))
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}

			for _, snippet := range tt.snippets {
				if !strings.Contains(string(rendered), snippet) {
					t.Fatalf("generated manifest did not contain %q: %q", snippet, string(rendered))
				}
			}

			if !strings.Contains(stdout.String(), "Wrote ") {
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

	if err := Run([]string{
		"manifest", "generate", "launch-agent", "brew-update",
		"--command", "/opt/homebrew/bin/brew update",
		"--schedule", "interval",
		"--interval-seconds", "86400",
		"--run-at-load",
		"--dir", "manifests/examples",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	path := filepath.Join(tempDir, "manifests", "examples", ".forge", "brew-update", "launch-agent.yaml")
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

func TestRunManifestGenerateGitHubRepoSupportsNonInteractiveFlags(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{
		"manifest", "generate", "github-repo",
		"--application", "forge",
		"--owner", "emkaytec",
		"--visibility", "private",
		"--description", "Forge CLI repo",
		"--topic", "platform",
		"--topic", "automation",
		"--default-branch", "main",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	path := filepath.Join(tempDir, ".forge", "forge", "github-repo.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	for _, snippet := range []string{
		`name: "emkaytec-forge"`,
		`owner: "emkaytec"`,
		`name: "forge"`,
		"visibility: private",
		`description: "Forge CLI repo"`,
		`- "platform"`,
		`- "automation"`,
		"default_branch: main",
	} {
		if !strings.Contains(contents, snippet) {
			t.Fatalf("generated manifest did not contain %q: %q", snippet, contents)
		}
	}

	if _, err := schema.DecodeManifest(rendered); err != nil {
		t.Fatalf("DecodeManifest() error = %v", err)
	}

	if !strings.Contains(stdout.String(), path) {
		t.Fatalf("expected success output to mention %q, got %q", path, stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestGenerateHCPTFWorkspaceSupportsNonInteractiveFlags(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{
		"manifest", "generate", "hcp-tf-workspace",
		"--vcs-repo", "emkaytec/forge",
		"--environment", "dev",
		"--account-id", "123456789012",
		"--organization", "emkaytec",
		"--project", "platform",
		"--execution-mode", "remote",
		"--terraform-version", "1.14.0",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	path := filepath.Join(tempDir, ".forge", "forge", "hcp-tf-workspace-dev.yml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	for _, snippet := range []string{
		`name: "emkaytec-forge-dev"`,
		`name: "forge-dev"`,
		`environment: "dev"`,
		`organization: "emkaytec"`,
		`project: "platform"`,
		`account_id: "123456789012"`,
		"execution_mode: remote",
		`terraform_version: "1.14.0"`,
	} {
		if !strings.Contains(contents, snippet) {
			t.Fatalf("generated manifest did not contain %q: %q", snippet, contents)
		}
	}

	if strings.Contains(contents, "vcs_repo") {
		t.Fatalf("generated manifest should not contain vcs_repo: %q", contents)
	}

	if _, err := schema.DecodeManifest(rendered); err != nil {
		t.Fatalf("DecodeManifest() error = %v", err)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestGenerateLaunchAgentSupportsCalendarSchedule(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{
		"manifest", "generate", "launch-agent",
		"--application", "nightly-report",
		"--command", "/usr/local/bin/report.sh",
		"--schedule", "calendar",
		"--hour", "2",
		"--minute", "15",
		"--run-at-load=false",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	path := filepath.Join(tempDir, ".forge", "nightly-report", "launch-agent.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	for _, snippet := range []string{
		`name: "nightly-report"`,
		`label: "dev.emkaytec.nightly-report"`,
		`command: "/usr/local/bin/report.sh"`,
		"type: calendar",
		"hour: 2",
		"minute: 15",
		"run_at_load: false",
	} {
		if !strings.Contains(contents, snippet) {
			t.Fatalf("generated manifest did not contain %q: %q", snippet, contents)
		}
	}

	if _, err := schema.DecodeManifest(rendered); err != nil {
		t.Fatalf("DecodeManifest() error = %v", err)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestManifestGeneratePromptsForGitHubRepoFieldsWhenNameOmitted(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	// Prevent the owner prompt from hitting the real GitHub API on a dev
	// machine that happens to have a token configured or a logged-in gh CLI.
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	restoreGH := github.SetLookupGHTokenForTesting(func() string { return "" })
	t.Cleanup(restoreGH)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := newRootCommand(&stdout, &stderr, "dev")
	root.SetIn(strings.NewReader(strings.Join([]string{
		"forge",          // application name
		"emkaytec",       // repository owner
		"1",              // visibility: Private (index 1)
		"Forge CLI repo", // description
		"platform",       // topics
		"",               // default branch (accept default "main")
	}, "\n") + "\n"))
	root.SetArgs([]string{"manifest", "generate", "github-repo"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	path := filepath.Join(tempDir, ".forge", "forge", "github-repo.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	for _, snippet := range []string{
		"kind: GitHubRepository",
		`name: "emkaytec-forge"`,
		`owner: "emkaytec"`,
		`name: "forge"`,
		"visibility: private",
		`description: "Forge CLI repo"`,
		`- "platform"`,
		"default_branch: main",
	} {
		if !strings.Contains(contents, snippet) {
			t.Fatalf("generated manifest did not contain %q: %q", snippet, contents)
		}
	}

	if !strings.Contains(stdout.String(), "Application name:") {
		t.Fatalf("expected application name prompt, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Repository owner:") {
		t.Fatalf("expected repository owner prompt, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Visibility:") {
		t.Fatalf("expected visibility selector, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestManifestGeneratePromptsForHCPTFWorkspaceFieldsWhenNameOmitted(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	configPath := filepath.Join(tempDir, "aws-config")
	if err := os.WriteFile(configPath, []byte(`
[profile dev-admin]
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
		"emkaytec/forge", // vcs repo (required)
		"2",              // environment: Development (index 2; 1 is "None")
		"1",              // AWS account: dev-admin (index 1)
		"emkaytec",       // organization (required)
		"platform",       // project
		"1",              // execution mode: Remote (index 1)
		"1.14.0",         // terraform version
	}, "\n") + "\n"))
	root.SetArgs([]string{"manifest", "generate", "hcp-tf-workspace"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	path := filepath.Join(tempDir, ".forge", "forge", "hcp-tf-workspace-dev.yml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	for _, snippet := range []string{
		"kind: HCPTerraformWorkspace",
		`name: "emkaytec-forge-dev"`,
		`name: "forge-dev"`,
		`environment: "dev"`,
		`organization: "emkaytec"`,
		`project: "platform"`,
		`account_id: "123456789012"`,
		"execution_mode: remote",
		`terraform_version: "1.14.0"`,
	} {
		if !strings.Contains(contents, snippet) {
			t.Fatalf("generated manifest did not contain %q: %q", snippet, contents)
		}
	}

	if !strings.Contains(stdout.String(), "VCS repo (owner/repo):") {
		t.Fatalf("expected vcs repo prompt, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Environment:") {
		t.Fatalf("expected environment selector, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "AWS account:") {
		t.Fatalf("expected AWS account selector, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "HCP TF organization") {
		t.Fatalf("expected HCP TF organization prompt, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "HCP TF project") {
		t.Fatalf("expected HCP TF project prompt, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Execution mode:") {
		t.Fatalf("expected execution mode selector, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestManifestGeneratePromptsPreferAWSAccountMatchingEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	configPath := filepath.Join(tempDir, "aws-config")
	if err := os.WriteFile(configPath, []byte(`
[default]
sso_account_id = 000000000000

[profile emkaytec-pre]
sso_account_id = 222222222222

[profile emkaytec-dev]
sso_account_id = 111111111111
`), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	t.Setenv("AWS_CONFIG_FILE", configPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(tempDir, "missing-credentials"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := newRootCommand(&stdout, &stderr, "dev")
	root.SetIn(strings.NewReader(strings.Join([]string{
		"emkaytec/forge", // vcs repo
		"2",              // environment: Development (index 2; 1 is "None")
		"",               // AWS account: accept default prioritized match
		"emkaytec",       // organization
		"platform",       // project
		"1",              // execution mode: Remote
		"1.14.0",         // terraform version
	}, "\n") + "\n"))
	root.SetArgs([]string{"manifest", "generate", "hcp-tf-workspace"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	path := filepath.Join(tempDir, ".forge", "forge", "hcp-tf-workspace-dev.yml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	if !strings.Contains(contents, `account_id: "111111111111"`) {
		t.Fatalf("generated manifest did not contain prioritized dev account_id: %q", contents)
	}

	renderedPrompts := stdout.String()
	devIndex := strings.Index(renderedPrompts, "emkaytec-dev (111111111111)")
	defaultIndex := strings.Index(renderedPrompts, "default (000000000000)")
	preIndex := strings.Index(renderedPrompts, "emkaytec-pre (222222222222)")
	if devIndex == -1 || defaultIndex == -1 || preIndex == -1 {
		t.Fatalf("expected prioritized AWS account options in prompt output, got %q", renderedPrompts)
	}
	if !(devIndex < defaultIndex && devIndex < preIndex) {
		t.Fatalf("expected dev profile to be listed before non-matching profiles, got %q", renderedPrompts)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestManifestGeneratePromptsForLaunchAgentFieldsWhenNameOmitted(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := newRootCommand(&stdout, &stderr, "dev")
	root.SetIn(strings.NewReader(strings.Join([]string{
		"brew-update",                   // application name
		"/opt/homebrew/bin/brew update", // command (required)
		"1",                             // schedule type: Interval (index 1)
		"86400",                         // interval seconds
		"1",                             // run at load: Yes (index 1)
	}, "\n") + "\n"))
	root.SetArgs([]string{"manifest", "generate", "launch-agent"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	path := filepath.Join(tempDir, ".forge", "brew-update", "launch-agent.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	for _, snippet := range []string{
		"kind: LaunchAgent",
		`name: "brew-update"`,
		`label: "dev.emkaytec.brew-update"`,
		`command: "/opt/homebrew/bin/brew update"`,
		"type: interval",
		"interval_seconds: 86400",
		"run_at_load: true",
	} {
		if !strings.Contains(contents, snippet) {
			t.Fatalf("generated manifest did not contain %q: %q", snippet, contents)
		}
	}

	if !strings.Contains(stdout.String(), "Application name:") {
		t.Fatalf("expected application name prompt, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Schedule type:") {
		t.Fatalf("expected schedule type selector, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestManifestGeneratePromptsForAWSIAMProvisionerFieldsWhenNameOmitted(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	configPath := filepath.Join(tempDir, "aws-config")
	if err := os.WriteFile(configPath, []byte(`
[profile prod-admin]
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
		"emkaytec/forge",
		"4", // environment: Prod (index 4; 1 is "None")
		"",
		"arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, "\n") + "\n"))
	root.SetArgs([]string{"manifest", "generate", "aws-iam-provisioner"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	manifestPath := filepath.Join(tempDir, ".forge", "forge", "aws-iam-provisioner-prod.yaml")
	rendered, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	if !strings.Contains(contents, `kind: AWSIAMProvisioner`) {
		t.Fatalf("generated manifest did not contain AWSIAMProvisioner kind: %q", contents)
	}

	if !strings.Contains(contents, `account_id: "123456789012"`) {
		t.Fatalf("generated manifest did not contain prompted account_id: %q", contents)
	}

	if !strings.Contains(contents, `name: "emkaytec-forge-prod"`) {
		t.Fatalf("generated manifest did not contain expected metadata name: %q", contents)
	}

	if !strings.Contains(contents, `name: "forge-prod-provisioner-role"`) {
		t.Fatalf("generated manifest did not contain expected role name: %q", contents)
	}

	if !strings.Contains(contents, `- oidc_provider: "token.actions.githubusercontent.com"`) {
		t.Fatalf("generated manifest did not contain GitHub Actions provider trust: %q", contents)
	}

	if !strings.Contains(contents, `oidc_subject: "repo:emkaytec/forge:*"`) {
		t.Fatalf("generated manifest did not contain expected GitHub subject: %q", contents)
	}

	if !strings.Contains(contents, `- oidc_provider: "app.terraform.io"`) {
		t.Fatalf("generated manifest did not contain HCP Terraform provider trust: %q", contents)
	}

	if !strings.Contains(contents, `oidc_subject: "organization:emkaytec:project:*:workspace:forge-prod:run_phase:*"`) {
		t.Fatalf("generated manifest did not contain expected HCP subject: %q", contents)
	}

	if !strings.Contains(stdout.String(), "AWS account:") {
		t.Fatalf("expected AWS account selector, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Environment:") {
		t.Fatalf("expected environment selector, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "VCS repo (owner/repo):") {
		t.Fatalf("expected VCS repo prompt, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Managed policy ARNs (comma-separated)") {
		t.Fatalf("expected managed policy prompt, got %q", stdout.String())
	}

	if strings.Contains(stdout.String(), "Application name:") {
		t.Fatalf("did not expect application name prompt, got %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "Provisioning systems:") {
		t.Fatalf("did not expect provisioning systems prompt, got %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "HCP TF workspace (organization/project/workspace):") {
		t.Fatalf("did not expect HCP TF workspace prompt, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestGenerateAWSIAMProvisionerSupportsNonInteractiveFlags(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{
		"manifest", "generate", "aws-iam-provisioner",
		"emkaytec/forge",
		"--environment", "dev",
		"--account-id", "123456789012",
		"--managed-policy", "arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	manifestPath := filepath.Join(tempDir, ".forge", "forge", "aws-iam-provisioner-dev.yaml")
	rendered, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	if !strings.Contains(contents, `name: "emkaytec-forge-dev"`) {
		t.Fatalf("generated manifest did not contain expected metadata name: %q", contents)
	}
	if !strings.Contains(contents, `name: "forge-dev-provisioner-role"`) {
		t.Fatalf("generated manifest did not contain expected role name: %q", contents)
	}
	if !strings.Contains(contents, `- oidc_provider: "token.actions.githubusercontent.com"`) {
		t.Fatalf("generated manifest did not contain GitHub Actions provider trust: %q", contents)
	}
	if !strings.Contains(contents, `oidc_subject: "repo:emkaytec/forge:*"`) {
		t.Fatalf("generated manifest did not contain expected GitHub subject: %q", contents)
	}
	if !strings.Contains(contents, `- oidc_provider: "app.terraform.io"`) {
		t.Fatalf("generated manifest did not contain HCP Terraform provider trust: %q", contents)
	}
	if !strings.Contains(contents, `oidc_subject: "organization:emkaytec:project:*:workspace:forge-dev:run_phase:*"`) {
		t.Fatalf("generated manifest did not contain expected HCP subject: %q", contents)
	}

	if !strings.Contains(stdout.String(), manifestPath) {
		t.Fatalf("expected success output to mention manifest path, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestGenerateAWSIAMProvisionerDerivesNamesFromVCSRepo(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{
		"manifest", "generate", "aws-iam-provisioner",
		"emkaytec/ForgeApp",
		"--environment", "dev",
		"--account-id", "123456789012",
		"--managed-policy", "arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	path := filepath.Join(tempDir, ".forge", "forge-app", "aws-iam-provisioner-dev.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	if !strings.Contains(contents, `name: "emkaytec-forge-app-dev"`) {
		t.Fatalf("generated manifest did not contain normalized metadata name: %q", contents)
	}

	if !strings.Contains(contents, `name: "forge-app-dev-provisioner-role"`) {
		t.Fatalf("generated manifest did not contain normalized role name: %q", contents)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestGenerateAWSIAMProvisionerWritesSingleManifestTrustedByBothProviders(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{
		"manifest", "generate", "aws-iam-provisioner",
		"emkaytec/forge",
		"--environment", "dev",
		"--account-id", "123456789012",
		"--managed-policy", "arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	manifestPath := filepath.Join(tempDir, ".forge", "forge", "aws-iam-provisioner-dev.yaml")
	rendered, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	if !strings.Contains(contents, `name: "emkaytec-forge-dev"`) {
		t.Fatalf("generated manifest did not contain suffixed metadata name: %q", contents)
	}

	if !strings.Contains(contents, `name: "forge-dev-provisioner-role"`) {
		t.Fatalf("generated manifest did not contain suffixed role name: %q", contents)
	}

	if !strings.Contains(contents, `- oidc_provider: "token.actions.githubusercontent.com"`) {
		t.Fatalf("generated manifest did not contain GitHub Actions trust: %q", contents)
	}

	if !strings.Contains(contents, `- oidc_provider: "app.terraform.io"`) {
		t.Fatalf("generated manifest did not contain HCP Terraform trust: %q", contents)
	}

	// Stale output files from the previous dual-manifest layout must not
	// reappear — a single manifest is the whole contract now.
	for _, stale := range []string{
		filepath.Join(tempDir, ".forge", "forge", "aws-iam-provisioner-dev-gha.yaml"),
		filepath.Join(tempDir, ".forge", "forge", "aws-iam-provisioner-dev-tfc.yaml"),
	} {
		if _, err := os.Stat(stale); err == nil {
			t.Fatalf("unexpected stale manifest written: %s", stale)
		}
	}

	if !strings.Contains(stdout.String(), manifestPath) {
		t.Fatalf("expected success output to mention manifest path, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestGenerateHelpDocumentsRepoDrivenSingleManifestFlow(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"manifest", "generate", "aws-iam-provisioner", "--help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "forge manifest generate aws-iam-provisioner [vcs-repo]") {
		t.Fatalf("expected usage for aws-iam-provisioner, got %q", stdout.String())
	}

	if strings.Contains(stdout.String(), "[application]") || strings.Contains(stdout.String(), "--application") {
		t.Fatalf("did not expect application argument help, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "the connected VCS repo") || !strings.Contains(stdout.String(), "single provisioner manifest whose IAM role trusts both") {
		t.Fatalf("expected prompt-flow help text, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "--account-profile") || !strings.Contains(stdout.String(), "--vcs-repo") || !strings.Contains(stdout.String(), "--environment") {
		t.Fatalf("expected aws provisioner flags in help output, got %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "--provider") || strings.Contains(stdout.String(), "--github-repo") || strings.Contains(stdout.String(), "--hcp-workspace") {
		t.Fatalf("did not expect provider-selection flags in help output, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "forge manifest generate aws-iam-provisioner emkaytec/forge --environment dev --account-id 123456789012") {
		t.Fatalf("expected non-interactive example usage, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "aws-iam-provisioner[-<env>].yaml") {
		t.Fatalf("expected single-manifest output note, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestGenerateAWSIAMProvisionerTruncatesRoleNameToAWSLimit(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	longApplication := "this-application-name-is-intentionally-long-enough-to-force-role-name-truncation"

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{
		"manifest", "generate", "aws-iam-provisioner",
		"emkaytec/" + longApplication,
		"--environment", "dev",
		"--account-id", "123456789012",
		"--managed-policy", "arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	path := filepath.Join(tempDir, ".forge", longApplication, "aws-iam-provisioner-dev.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	manifest, err := schema.DecodeManifest(rendered)
	if err != nil {
		t.Fatalf("DecodeManifest() error = %v", err)
	}

	spec, ok := manifest.Spec.(*schema.AWSIAMProvisionerSpec)
	if !ok {
		t.Fatalf("manifest spec type = %T, want *schema.AWSIAMProvisionerSpec", manifest.Spec)
	}

	if len([]rune(spec.Name)) != schema.AWSIAMRoleNameMaxLength {
		t.Fatalf("generated role name length = %d, want %d (%q)", len([]rune(spec.Name)), schema.AWSIAMRoleNameMaxLength, spec.Name)
	}

	if !strings.HasSuffix(spec.Name, "-dev-provisioner-role") {
		t.Fatalf("generated role name = %q, want dev suffix", spec.Name)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestManifestGeneratePromptsDeriveNamesFromVCSRepo(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	configPath := filepath.Join(tempDir, "aws-config")
	if err := os.WriteFile(configPath, []byte(`
[profile prod-admin]
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
		"emkaytec/ForgeApp",
		"4", // environment: Prod (index 4; 1 is "None")
		"",
		"arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, "\n") + "\n"))
	root.SetArgs([]string{"manifest", "generate", "aws-iam-provisioner"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	path := filepath.Join(tempDir, ".forge", "forge-app", "aws-iam-provisioner-prod.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	if !strings.Contains(contents, `name: "emkaytec-forge-app-prod"`) {
		t.Fatalf("generated manifest did not contain normalized metadata name: %q", contents)
	}

	if !strings.Contains(contents, `name: "forge-app-prod-provisioner-role"`) {
		t.Fatalf("generated manifest did not contain normalized role name: %q", contents)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}
