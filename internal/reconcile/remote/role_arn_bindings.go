package remote

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/emkaytec/forge/internal/aws/iamcli"
	ghapi "github.com/emkaytec/forge/internal/github"
	hcpapi "github.com/emkaytec/forge/internal/hcpterraform"
	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

const (
	gitHubActionsIssuer = "token.actions.githubusercontent.com"
	hcpTerraformIssuer  = "app.terraform.io"
)

type roleARNLookupClient interface {
	GetRole(ctx context.Context, roleName string) (*iamcli.Role, error)
}

type hcpWorkspaceVariableClient interface {
	GetWorkspace(ctx context.Context, organization string, name string) (*hcpapi.Workspace, error)
	ListVariables(ctx context.Context, workspaceID string) ([]hcpapi.WorkspaceVariable, error)
	CreateVariable(ctx context.Context, workspaceID string, variable hcpapi.WorkspaceVariable) error
	UpdateVariable(ctx context.Context, workspaceID string, variableID string, variable hcpapi.WorkspaceVariable) error
}

type githubRepositoryVariableClient interface {
	GetRepositoryVariable(ctx context.Context, owner string, repo string, name string) (*ghapi.RepositoryVariable, error)
	CreateRepositoryVariable(ctx context.Context, owner string, repo string, variable ghapi.RepositoryVariable) error
	UpdateRepositoryVariable(ctx context.Context, owner string, repo string, name string, variable ghapi.RepositoryVariable) error
}

type roleARNBindings struct {
	newRoleClient   func(accountID string) roleARNLookupClient
	newHCPClient    func() (hcpWorkspaceVariableClient, error)
	newGitHubClient func() (githubRepositoryVariableClient, error)
}

func defaultRoleARNBindings() roleARNBindings {
	return roleARNBindings{
		newRoleClient: func(accountID string) roleARNLookupClient {
			return iamcli.New().ForAccount(accountID)
		},
		newHCPClient: func() (hcpWorkspaceVariableClient, error) {
			return hcpapi.NewClientFromEnv()
		},
		newGitHubClient: func() (githubRepositoryVariableClient, error) {
			return ghapi.NewClientFromEnv()
		},
	}
}

func (e *Executor) bindAWSProvisionerRoleARNs(ctx context.Context, result *reconcile.ApplyResult, plan *reconcile.Plan) {
	if result == nil || plan == nil || result.DryRun {
		return
	}

	for _, change := range result.Applied {
		if change.Kind() != schema.KindAWSIAMProvisioner || change.Action == reconcile.ActionDelete {
			continue
		}

		if err := e.bindAWSProvisionerRoleARN(ctx, change, plan); err != nil {
			result.Failed = append(result.Failed, reconcile.FailedChange{Change: change, Err: err})
		}
	}
}

func (e *Executor) bindAWSProvisionerRoleARN(ctx context.Context, change reconcile.ResourceChange, plan *reconcile.Plan) error {
	spec, ok := change.Manifest.Spec.(*schema.AWSIAMProvisionerSpec)
	if !ok {
		return fmt.Errorf("AWSIAMProvisioner: unexpected spec type %T", change.Manifest.Spec)
	}

	switch spec.OIDCProvider {
	case hcpTerraformIssuer:
		workspace, ok := matchingHCPTFWorkspace(change, plan)
		if !ok {
			return nil
		}

		roleARN, err := e.lookupRoleARN(ctx, spec.AccountID, spec.Name)
		if err != nil {
			return err
		}

		return e.ensureHCPWorkspaceRoleARNVariable(ctx, workspace, roleARN)
	case gitHubActionsIssuer:
		repository, environment, ok := matchingGitHubRepository(change, plan)
		if !ok {
			return nil
		}

		roleARN, err := e.lookupRoleARN(ctx, spec.AccountID, spec.Name)
		if err != nil {
			return err
		}

		return e.ensureGitHubRepositoryRoleARNVariable(ctx, repository, environment, roleARN)
	default:
		return nil
	}
}

func (e *Executor) lookupRoleARN(ctx context.Context, accountID string, roleName string) (string, error) {
	role, err := e.bindings.newRoleClient(accountID).GetRole(ctx, roleName)
	if err != nil {
		return "", err
	}
	if role == nil || strings.TrimSpace(role.ARN) == "" {
		return "", fmt.Errorf("aws iam role %q did not include an ARN", roleName)
	}
	return role.ARN, nil
}

type hcpWorkspaceTarget struct {
	spec *schema.HCPTFWorkspaceSpec
}

func matchingHCPTFWorkspace(change reconcile.ResourceChange, plan *reconcile.Plan) (hcpWorkspaceTarget, bool) {
	awsSpec, ok := change.Manifest.Spec.(*schema.AWSIAMProvisionerSpec)
	if !ok {
		return hcpWorkspaceTarget{}, false
	}

	workspaceName, hasWorkspaceName := workspaceNameFromHCPTFSubject(awsSpec.OIDCSubject)
	for _, candidate := range plan.Changes {
		if candidate.Kind() != schema.KindHCPTFWorkspace || !sameManifestDirectory(change.Source, candidate.Source) {
			continue
		}

		spec, ok := candidate.Manifest.Spec.(*schema.HCPTFWorkspaceSpec)
		if !ok {
			continue
		}

		if hasWorkspaceName && spec.Name != workspaceName {
			continue
		}

		return hcpWorkspaceTarget{spec: spec}, true
	}

	return hcpWorkspaceTarget{}, false
}

func (e *Executor) ensureHCPWorkspaceRoleARNVariable(ctx context.Context, target hcpWorkspaceTarget, roleARN string) error {
	client, err := e.bindings.newHCPClient()
	if err != nil {
		return err
	}

	workspace, err := client.GetWorkspace(ctx, target.spec.Organization, target.spec.Name)
	if err != nil {
		return err
	}
	if workspace == nil || strings.TrimSpace(workspace.ID) == "" {
		return fmt.Errorf("hcp terraform workspace %q did not include an ID", target.spec.Name)
	}

	desired := hcpapi.WorkspaceVariable{
		Key:         roleARNVariableKey(target.spec.Environment),
		Value:       roleARN,
		Description: "Managed by Forge from an AWS IAM provisioner manifest.",
		Category:    "env",
	}

	variables, err := client.ListVariables(ctx, workspace.ID)
	if err != nil {
		return err
	}

	current, ok := findHCPWorkspaceVariable(variables, desired.Category, desired.Key)
	if !ok {
		if err := client.CreateVariable(ctx, workspace.ID, desired); !hcpapi.IsAlreadyExists(err) {
			return err
		}
		return e.updateExistingHCPWorkspaceRoleARNVariable(ctx, client, workspace.ID, desired)
	}
	if hcpWorkspaceVariableMatches(current, desired) {
		return nil
	}

	return client.UpdateVariable(ctx, workspace.ID, current.ID, desired)
}

func (e *Executor) updateExistingHCPWorkspaceRoleARNVariable(ctx context.Context, client hcpWorkspaceVariableClient, workspaceID string, desired hcpapi.WorkspaceVariable) error {
	variables, err := client.ListVariables(ctx, workspaceID)
	if err != nil {
		return err
	}

	current, ok := findHCPWorkspaceVariable(variables, desired.Category, desired.Key)
	if !ok {
		return fmt.Errorf("hcp terraform variable %q already exists but could not be read", desired.Key)
	}
	if hcpWorkspaceVariableMatches(current, desired) {
		return nil
	}

	return client.UpdateVariable(ctx, workspaceID, current.ID, desired)
}

type githubRepositoryTarget struct {
	spec *schema.GitHubRepoSpec
}

func matchingGitHubRepository(change reconcile.ResourceChange, plan *reconcile.Plan) (githubRepositoryTarget, string, bool) {
	awsSpec, ok := change.Manifest.Spec.(*schema.AWSIAMProvisionerSpec)
	if !ok {
		return githubRepositoryTarget{}, "", false
	}

	owner, repo, hasRepo := repositoryFromGitHubActionsSubject(awsSpec.OIDCSubject)
	environment := environmentFromProvisioner(change, awsSpec, "gha")

	for _, candidate := range plan.Changes {
		if candidate.Kind() != schema.KindGitHubRepo || !sameManifestDirectory(change.Source, candidate.Source) {
			continue
		}

		spec, ok := candidate.Manifest.Spec.(*schema.GitHubRepoSpec)
		if !ok {
			continue
		}

		if hasRepo && (spec.Owner != owner || spec.Name != repo) {
			continue
		}

		return githubRepositoryTarget{spec: spec}, environment, true
	}

	return githubRepositoryTarget{}, "", false
}

func (e *Executor) ensureGitHubRepositoryRoleARNVariable(ctx context.Context, target githubRepositoryTarget, environment string, roleARN string) error {
	client, err := e.bindings.newGitHubClient()
	if err != nil {
		return err
	}

	name := roleARNVariableKey(environment)
	desired := ghapi.RepositoryVariable{
		Name:  name,
		Value: roleARN,
	}

	current, err := client.GetRepositoryVariable(ctx, target.spec.Owner, target.spec.Name, name)
	switch {
	case ghapi.IsNotFound(err):
		if err := client.CreateRepositoryVariable(ctx, target.spec.Owner, target.spec.Name, desired); !ghapi.IsAlreadyExists(err) {
			return err
		}
		return client.UpdateRepositoryVariable(ctx, target.spec.Owner, target.spec.Name, name, desired)
	case err != nil:
		return err
	}

	if current != nil && current.Value == roleARN {
		return nil
	}

	return client.UpdateRepositoryVariable(ctx, target.spec.Owner, target.spec.Name, name, desired)
}

func workspaceNameFromHCPTFSubject(subject string) (string, bool) {
	parts := strings.Split(subject, ":")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "workspace" && strings.TrimSpace(parts[i+1]) != "" {
			return parts[i+1], true
		}
	}

	return "", false
}

func repositoryFromGitHubActionsSubject(subject string) (string, string, bool) {
	parts := strings.Split(subject, ":")
	if len(parts) < 2 || parts[0] != "repo" {
		return "", "", false
	}

	repoParts := strings.Split(parts[1], "/")
	if len(repoParts) != 2 || repoParts[0] == "" || repoParts[1] == "" {
		return "", "", false
	}

	return repoParts[0], repoParts[1], true
}

func environmentFromProvisioner(change reconcile.ResourceChange, spec *schema.AWSIAMProvisionerSpec, providerSuffix string) string {
	for _, name := range []string{change.Name(), spec.Name} {
		if environment := environmentFromProvisionerName(name, providerSuffix); environment != "" {
			return environment
		}
	}

	return ""
}

func environmentFromProvisionerName(name string, providerSuffix string) string {
	for _, suffix := range []string{
		"-" + providerSuffix + "-provisioner-role",
		"-" + providerSuffix,
	} {
		if !strings.HasSuffix(name, suffix) {
			continue
		}

		base := strings.TrimSuffix(name, suffix)
		if idx := strings.LastIndex(base, "-"); idx >= 0 && idx < len(base)-1 {
			return base[idx+1:]
		}
	}

	return ""
}

func roleARNVariableKey(environment string) string {
	environment = strings.TrimSpace(environment)
	if environment == "" {
		return "AWS_PROVISIONER_ROLE_ARN"
	}

	environment = strings.ToUpper(strings.ReplaceAll(environment, "-", "_"))
	return "AWS_PROVISIONER_ROLE_ARN_" + environment
}

func sameManifestDirectory(a string, b string) bool {
	return filepath.Clean(filepath.Dir(a)) == filepath.Clean(filepath.Dir(b))
}

func findHCPWorkspaceVariable(variables []hcpapi.WorkspaceVariable, category, key string) (hcpapi.WorkspaceVariable, bool) {
	for _, variable := range variables {
		if variable.Category == category && variable.Key == key {
			return variable, true
		}
	}

	for _, variable := range variables {
		if variable.Key == key {
			return variable, true
		}
	}

	return hcpapi.WorkspaceVariable{}, false
}

func hcpWorkspaceVariableMatches(current, desired hcpapi.WorkspaceVariable) bool {
	return current.Key == desired.Key &&
		current.Value == desired.Value &&
		current.Description == desired.Description &&
		current.Category == desired.Category &&
		current.HCL == desired.HCL &&
		current.Sensitive == desired.Sensitive
}
