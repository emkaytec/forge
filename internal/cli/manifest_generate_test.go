package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emkaytec/forge/pkg/schema"
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
		"forge",
		"1",
		"1",
		"emkaytec/forge",
		"arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, "\n") + "\n"))
	root.SetArgs([]string{"manifest", "generate", "aws-iam-provisioner"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	rendered, err := os.ReadFile(filepath.Join(tempDir, "forge", "aws-iam-provisioner-gha.yaml"))
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

	if !strings.Contains(contents, `name: "forge-gha-provisioner-role"`) {
		t.Fatalf("generated manifest did not contain expected role name: %q", contents)
	}

	if !strings.Contains(contents, `oidc_provider: "token.actions.githubusercontent.com"`) {
		t.Fatalf("generated manifest did not contain expected provider: %q", contents)
	}

	if !strings.Contains(contents, `oidc_subject: "repo:emkaytec/forge:*"`) {
		t.Fatalf("generated manifest did not contain expected GitHub subject: %q", contents)
	}

	if !strings.Contains(stdout.String(), "Application name:") {
		t.Fatalf("expected application name prompt, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "AWS account:") {
		t.Fatalf("expected AWS account selector, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Provisioning systems:") {
		t.Fatalf("expected provisioning systems selector, got %q", stdout.String())
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
		"--application", "forge",
		"--account-id", "123456789012",
		"--provider", "hcp-terraform",
		"--hcp-workspace", "emkaytec/platform/forge",
		"--managed-policy", "arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	path := filepath.Join(tempDir, "forge", "aws-iam-provisioner-tfc.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	if !strings.Contains(contents, `name: "forge-tfc"`) {
		t.Fatalf("generated manifest did not contain expected metadata name: %q", contents)
	}

	if !strings.Contains(contents, `name: "forge-tfc-provisioner-role"`) {
		t.Fatalf("generated manifest did not contain expected role name: %q", contents)
	}

	if !strings.Contains(contents, `oidc_provider: "app.terraform.io"`) {
		t.Fatalf("generated manifest did not contain expected HCP provider: %q", contents)
	}

	if !strings.Contains(contents, `oidc_subject: "organization:emkaytec:project:platform:workspace:forge:run_phase:*"`) {
		t.Fatalf("generated manifest did not contain expected HCP subject: %q", contents)
	}

	if !strings.Contains(stdout.String(), path) {
		t.Fatalf("expected success output to mention %q, got %q", path, stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestGenerateAWSIAMProvisionerNormalizesApplicationNameFromFlag(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{
		"manifest", "generate", "aws-iam-provisioner",
		"--application", "  Forge App  ",
		"--account-id", "123456789012",
		"--provider", "github-actions",
		"--github-repo", "emkaytec/forge",
		"--managed-policy", "arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	path := filepath.Join(tempDir, "forge-app", "aws-iam-provisioner-gha.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	if !strings.Contains(contents, `name: "forge-app-gha"`) {
		t.Fatalf("generated manifest did not contain normalized metadata name: %q", contents)
	}

	if !strings.Contains(contents, `name: "forge-app-gha-provisioner-role"`) {
		t.Fatalf("generated manifest did not contain normalized role name: %q", contents)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestGenerateAWSIAMProvisionerSupportsMultipleProviders(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{
		"manifest", "generate", "aws-iam-provisioner",
		"--application", "forge",
		"--account-id", "123456789012",
		"--provider", "github-actions",
		"--provider", "hcp-terraform",
		"--github-repo", "emkaytec/forge",
		"--hcp-workspace", "emkaytec/platform/forge",
		"--managed-policy", "arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	githubPath := filepath.Join(tempDir, "forge", "aws-iam-provisioner-gha.yaml")
	githubRendered, err := os.ReadFile(githubPath)
	if err != nil {
		t.Fatalf("ReadFile(github) error = %v", err)
	}

	githubContents := string(githubRendered)
	if !strings.Contains(githubContents, `name: "forge-gha"`) {
		t.Fatalf("generated GitHub manifest did not contain suffixed metadata name: %q", githubContents)
	}

	if !strings.Contains(githubContents, `name: "forge-gha-provisioner-role"`) {
		t.Fatalf("generated GitHub manifest did not contain suffixed role name: %q", githubContents)
	}

	hcpPath := filepath.Join(tempDir, "forge", "aws-iam-provisioner-tfc.yaml")
	hcpRendered, err := os.ReadFile(hcpPath)
	if err != nil {
		t.Fatalf("ReadFile(hcp) error = %v", err)
	}

	hcpContents := string(hcpRendered)
	if !strings.Contains(hcpContents, `name: "forge-tfc"`) {
		t.Fatalf("generated HCP manifest did not contain suffixed metadata name: %q", hcpContents)
	}

	if !strings.Contains(hcpContents, `name: "forge-tfc-provisioner-role"`) {
		t.Fatalf("generated HCP manifest did not contain suffixed role name: %q", hcpContents)
	}

	if !strings.Contains(stdout.String(), githubPath) || !strings.Contains(stdout.String(), hcpPath) {
		t.Fatalf("expected success output to mention both manifest paths, got %q", stdout.String())
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

	if !strings.Contains(stdout.String(), "forge manifest generate aws-iam-provisioner [application]") {
		t.Fatalf("expected usage with optional application argument, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "the application name") {
		t.Fatalf("expected prompt-flow help text, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "--account-profile") || !strings.Contains(stdout.String(), "--provider") {
		t.Fatalf("expected aws provisioner flags in help output, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "forge manifest generate aws-iam-provisioner --application forge --account-id 123456789012") {
		t.Fatalf("expected non-interactive example usage, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "aws-iam-provisioner-gha.yaml") || !strings.Contains(stdout.String(), "aws-iam-provisioner-tfc.yaml") {
		t.Fatalf("expected provider-specific output note, got %q", stdout.String())
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
		"--application", longApplication,
		"--account-id", "123456789012",
		"--provider", "github-actions",
		"--github-repo", "emkaytec/forge",
		"--managed-policy", "arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	path := filepath.Join(tempDir, longApplication, "aws-iam-provisioner-gha.yaml")
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

	if !strings.HasSuffix(spec.Name, "-gha-provisioner-role") {
		t.Fatalf("generated role name = %q, want gha suffix", spec.Name)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestManifestGeneratePromptsNormalizeApplicationName(t *testing.T) {
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
		"  Forge App  ",
		"1",
		"1",
		"emkaytec/forge",
		"arn:aws:iam::aws:policy/ReadOnlyAccess",
	}, "\n") + "\n"))
	root.SetArgs([]string{"manifest", "generate", "aws-iam-provisioner"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	path := filepath.Join(tempDir, "forge-app", "aws-iam-provisioner-gha.yaml")
	rendered, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	contents := string(rendered)
	if !strings.Contains(contents, `name: "forge-app-gha"`) {
		t.Fatalf("generated manifest did not contain normalized metadata name: %q", contents)
	}

	if !strings.Contains(contents, `name: "forge-app-gha-provisioner-role"`) {
		t.Fatalf("generated manifest did not contain normalized role name: %q", contents)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}
