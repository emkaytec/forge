package oidc

import "testing"

func TestGitHubActionsSubject(t *testing.T) {
	provider, ok := Lookup("github-actions")
	if !ok {
		t.Fatal("github-actions provider not found")
	}

	subject, err := provider.BuildSubject("emkaytec/forge")
	if err != nil {
		t.Fatalf("BuildSubject() error = %v", err)
	}

	if subject != "repo:emkaytec/forge:*" {
		t.Fatalf("subject = %q, want repo:emkaytec/forge:*", subject)
	}
}

func TestHCPTerraformSubject(t *testing.T) {
	provider, ok := Lookup("hcp-terraform")
	if !ok {
		t.Fatal("hcp-terraform provider not found")
	}

	subject, err := provider.BuildSubject("emkaytec/platform/forge")
	if err != nil {
		t.Fatalf("BuildSubject() error = %v", err)
	}

	if subject != "organization:emkaytec:project:platform:workspace:forge:run_phase:*" {
		t.Fatalf("subject = %q, want organization:emkaytec:project:platform:workspace:forge:run_phase:*", subject)
	}
}
