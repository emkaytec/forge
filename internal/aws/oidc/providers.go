package oidc

import (
	"fmt"
	"strings"
)

// Provider describes one built-in OIDC trust shape Forge knows how to model.
type Provider struct {
	Key          string
	Label        string
	NameSuffix   string
	Issuer       string
	TargetFlag   string
	TargetLabel  string
	TargetHelp   string
	BuildSubject func(target string) (string, error)
}

var providers = []Provider{
	{
		Key:          "github-actions",
		Label:        "GitHub Actions",
		NameSuffix:   "github-actions",
		Issuer:       "token.actions.githubusercontent.com",
		TargetFlag:   "github-repo",
		TargetLabel:  "GitHub repository (owner/repo)",
		TargetHelp:   "Repository path to trust for GitHub Actions, such as emkaytec/forge",
		BuildSubject: buildGitHubActionsSubject,
	},
	{
		Key:          "hcp-terraform",
		Label:        "HCP Terraform",
		NameSuffix:   "hcp-terraform",
		Issuer:       "app.terraform.io",
		TargetFlag:   "hcp-workspace",
		TargetLabel:  "HCP Terraform workspace (organization/project/workspace)",
		TargetHelp:   "Workspace path to trust for HCP Terraform, such as emkaytec/platform/forge",
		BuildSubject: buildHCPTerraformSubject,
	},
}

// Providers returns the built-in provider registry.
func Providers() []Provider {
	out := make([]Provider, len(providers))
	copy(out, providers)
	return out
}

// Lookup resolves a provider by key.
func Lookup(key string) (Provider, bool) {
	for _, provider := range providers {
		if provider.Key == key {
			return provider, true
		}
	}

	return Provider{}, false
}

func buildGitHubActionsSubject(target string) (string, error) {
	parts := strings.Split(strings.TrimSpace(target), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("GitHub repository must look like owner/repo")
	}

	return fmt.Sprintf("repo:%s:*", strings.Join(parts, "/")), nil
}

func buildHCPTerraformSubject(target string) (string, error) {
	parts := strings.Split(strings.TrimSpace(target), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", fmt.Errorf("HCP Terraform workspace must look like organization/project/workspace")
	}

	return fmt.Sprintf(
		"organization:%s:project:%s:workspace:%s:run_phase:*",
		parts[0],
		parts[1],
		parts[2],
	), nil
}
