package manifest

import (
	"io"
	"strings"
	"testing"

	"github.com/emkaytec/forge/internal/aws/accounts"
)

func TestResolveRepositoryNamePrefersFlag(t *testing.T) {
	p := newPromptSession(strings.NewReader(""), io.Discard)
	configureGitHubRepoFlow(p)

	name, err := resolveRepositoryName(p, nil, "sample-repo")
	if err != nil {
		t.Fatalf("resolveRepositoryName() error = %v", err)
	}
	if name != "sample-repo" {
		t.Fatalf("name = %q, want sample-repo", name)
	}
}

func TestResolveRepositoryNameRejectsUnsupportedCharacters(t *testing.T) {
	p := newPromptSession(strings.NewReader(""), io.Discard)
	configureGitHubRepoFlow(p)

	if _, err := resolveRepositoryName(p, []string{"owner/repo"}, ""); err == nil {
		t.Fatal("expected repository name error")
	}
}

func TestResolveTerraformRepoPromptsWhenNotSpecified(t *testing.T) {
	var out strings.Builder
	p := newPromptSession(strings.NewReader("1\n"), &out)
	configureGitHubRepoFlow(p)

	enabled, err := resolveTerraformRepo(p, false, gitHubRepoGenerateOptions{})
	if err != nil {
		t.Fatalf("resolveTerraformRepo() error = %v", err)
	}
	if !enabled {
		t.Fatal("expected Terraform repo selection")
	}
	if !strings.Contains(out.String(), "Is this a Terraform repo?") {
		t.Fatalf("expected Terraform prompt, got %q", out.String())
	}
}

func TestResolveTerraformRepoInfersFromTerraformFlags(t *testing.T) {
	p := newPromptSession(strings.NewReader(""), io.Discard)
	configureGitHubRepoFlow(p)

	enabled, err := resolveTerraformRepo(p, false, gitHubRepoGenerateOptions{accountID: "123456789012"})
	if err != nil {
		t.Fatalf("resolveTerraformRepo() error = %v", err)
	}
	if !enabled {
		t.Fatal("expected Terraform repo when Terraform-specific flags are set")
	}
}

func TestResolveTerraformEnvironmentAcceptsCustomLowercaseKey(t *testing.T) {
	p := newPromptSession(strings.NewReader(""), io.Discard)
	configureGitHubRepoFlow(p)

	environment, err := resolveTerraformEnvironment(p, "shared-services")
	if err != nil {
		t.Fatalf("resolveTerraformEnvironment() error = %v", err)
	}
	if environment != "shared-services" {
		t.Fatalf("environment = %q, want shared-services", environment)
	}
}

func TestResolveAWSAccountIDPrefersFlag(t *testing.T) {
	p := newPromptSession(strings.NewReader(""), io.Discard)
	configureGitHubRepoFlow(p)

	accountID, err := resolveAWSAccountID(p, "", "123456789012", "admin")
	if err != nil {
		t.Fatalf("resolveAWSAccountID() error = %v", err)
	}
	if accountID != "123456789012" {
		t.Fatalf("accountID = %q, want 123456789012", accountID)
	}
}

func TestPrioritizeAWSProfilesMovesEnvironmentMatchesToFront(t *testing.T) {
	t.Parallel()

	profiles := []accounts.Profile{
		{Name: "default", AccountID: "000000000000"},
		{Name: "emkaytec-pre", AccountID: "222222222222"},
		{Name: "emkaytec-dev", AccountID: "111111111111"},
		{Name: "emkaytec-prod", AccountID: "333333333333"},
	}

	ordered, defaultIndex := accounts.PrioritizeProfiles(profiles, "dev")
	if defaultIndex != 0 {
		t.Fatalf("defaultIndex = %d, want 0", defaultIndex)
	}

	got := []string{
		ordered[0].Name,
		ordered[1].Name,
		ordered[2].Name,
		ordered[3].Name,
	}
	want := []string{"emkaytec-dev", "default", "emkaytec-pre", "emkaytec-prod"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordered[%d] = %q, want %q (full order: %#v)", i, got[i], want[i], got)
		}
	}
}

func TestRenderAnvilManifestValidatesTerraformRequiredFields(t *testing.T) {
	manifest := anvilGitHubRepositoryManifest{
		APIVersion: anvilAPIVersion,
		Kind:       anvilGitHubRepositoryKind,
		Metadata:   anvilMetadata{Name: "complete-service"},
		Spec: anvilGitHubRepositorySpec{
			CreateTerraformWorkspaces: true,
			Repository:                anvilRepository{Name: "complete-service", Visibility: "private"},
		},
	}

	if _, err := renderAnvilManifest(manifest); err == nil {
		t.Fatal("expected missing environment validation error")
	}
}
