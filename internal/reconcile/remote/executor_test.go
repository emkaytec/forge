package remote

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/emkaytec/forge/internal/aws/iamcli"
	ghapi "github.com/emkaytec/forge/internal/github"
	hcpapi "github.com/emkaytec/forge/internal/hcpterraform"
	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

type fakeHandler struct {
	kind         schema.Kind
	change       reconcile.ResourceChange
	describeErr  error
	applyErr     error
	describeSeen int
	applySeen    int
}

func (f *fakeHandler) Kind() schema.Kind { return f.kind }

func (f *fakeHandler) DescribeChange(context.Context, *schema.Manifest, string) (reconcile.ResourceChange, error) {
	f.describeSeen++
	return f.change, f.describeErr
}

func (f *fakeHandler) Apply(context.Context, reconcile.ResourceChange, reconcile.ApplyOptions) error {
	f.applySeen++
	return f.applyErr
}

func TestRemoteExecutorReportsTarget(t *testing.T) {
	exec := NewExecutor()
	if exec.Target() != reconcile.TargetRemote {
		t.Fatalf("want TargetRemote, got %q", exec.Target())
	}
}

func TestRemoteExecutorDescribeRoutesToHandler(t *testing.T) {
	handler := &fakeHandler{
		kind:   schema.KindGitHubRepo,
		change: reconcile.ResourceChange{Action: reconcile.ActionUpdate},
	}
	exec := newExecutor(handler)

	change, err := exec.DescribeChange(context.Background(), &schema.Manifest{Kind: schema.KindGitHubRepo}, "x.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if change.Action != reconcile.ActionUpdate {
		t.Fatalf("want ActionUpdate, got %q", change.Action)
	}
	if handler.describeSeen != 1 {
		t.Fatalf("expected one DescribeChange call, got %d", handler.describeSeen)
	}
}

func TestRemoteExecutorApplyCollectsHandlerFailure(t *testing.T) {
	handler := &fakeHandler{
		kind:     schema.KindGitHubRepo,
		applyErr: errors.New("boom"),
	}
	exec := newExecutor(handler)
	plan := &reconcile.Plan{
		Target: reconcile.TargetRemote,
		Changes: []reconcile.ResourceChange{{
			Manifest: &schema.Manifest{Kind: schema.KindGitHubRepo, Metadata: schema.Metadata{Name: "sample"}},
			Action:   reconcile.ActionCreate,
		}},
	}

	result, err := exec.Apply(context.Background(), plan, reconcile.ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Failed) != 1 {
		t.Fatalf("want 1 failed change, got %d", len(result.Failed))
	}
	if !errors.Is(result.Failed[0].Err, handler.applyErr) {
		t.Fatalf("want handler error, got %v", result.Failed[0].Err)
	}
}

func TestRemoteExecutorDryRunSkipsApply(t *testing.T) {
	handler := &fakeHandler{kind: schema.KindGitHubRepo}
	exec := newExecutor(handler)
	plan := &reconcile.Plan{
		Target: reconcile.TargetRemote,
		Changes: []reconcile.ResourceChange{{
			Manifest: &schema.Manifest{Kind: schema.KindGitHubRepo, Metadata: schema.Metadata{Name: "sample"}},
			Action:   reconcile.ActionCreate,
		}},
	}

	result, err := exec.Apply(context.Background(), plan, reconcile.ApplyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 1 || len(result.Failed) != 0 {
		t.Fatalf("dry run should succeed without calling handlers: %+v", result)
	}
	if handler.applySeen != 0 {
		t.Fatalf("expected dry run to skip handler.Apply, got %d calls", handler.applySeen)
	}
}

type fakeRoleBindingClient struct {
	roles map[string]*iamcli.Role
}

func (f *fakeRoleBindingClient) GetRole(_ context.Context, roleName string) (*iamcli.Role, error) {
	role, ok := f.roles[roleName]
	if !ok {
		return nil, errors.New("role not found")
	}
	return role, nil
}

type fakeHCPBindingClient struct {
	workspace         *hcpapi.Workspace
	variables         []hcpapi.WorkspaceVariable
	variableLists     [][]hcpapi.WorkspaceVariable
	createErr         error
	createdVariable   *hcpapi.WorkspaceVariable
	updatedVariable   *hcpapi.WorkspaceVariable
	updatedVariableID string
}

func (f *fakeHCPBindingClient) GetWorkspace(context.Context, string, string) (*hcpapi.Workspace, error) {
	return f.workspace, nil
}

func (f *fakeHCPBindingClient) ListVariables(context.Context, string) ([]hcpapi.WorkspaceVariable, error) {
	if len(f.variableLists) > 0 {
		variables := f.variableLists[0]
		f.variableLists = f.variableLists[1:]
		return variables, nil
	}
	return f.variables, nil
}

func (f *fakeHCPBindingClient) CreateVariable(_ context.Context, _ string, variable hcpapi.WorkspaceVariable) error {
	copied := variable
	f.createdVariable = &copied
	return f.createErr
}

func (f *fakeHCPBindingClient) UpdateVariable(_ context.Context, _ string, variableID string, variable hcpapi.WorkspaceVariable) error {
	copied := variable
	f.updatedVariable = &copied
	f.updatedVariableID = variableID
	return nil
}

type fakeGitHubBindingClient struct {
	variable        *ghapi.RepositoryVariable
	createErr       error
	createdVariable *ghapi.RepositoryVariable
	updatedVariable *ghapi.RepositoryVariable
}

func (f *fakeGitHubBindingClient) GetRepositoryVariable(context.Context, string, string, string) (*ghapi.RepositoryVariable, error) {
	if f.variable == nil {
		return nil, &ghapi.APIError{StatusCode: 404, Method: "GET", Path: "/actions/variables/AWS_PROVISIONER_ROLE_ARN_DEV"}
	}
	return f.variable, nil
}

func (f *fakeGitHubBindingClient) CreateRepositoryVariable(_ context.Context, _ string, _ string, variable ghapi.RepositoryVariable) error {
	copied := variable
	f.createdVariable = &copied
	return f.createErr
}

func (f *fakeGitHubBindingClient) UpdateRepositoryVariable(_ context.Context, _ string, _ string, _ string, variable ghapi.RepositoryVariable) error {
	copied := variable
	f.updatedVariable = &copied
	return nil
}

func TestRemoteExecutorWiresTFCProvisionerRoleARNToWorkspaceVariable(t *testing.T) {
	roleClient := &fakeRoleBindingClient{roles: map[string]*iamcli.Role{
		"sample-dev-tfc-provisioner-role": {ARN: "arn:aws:iam::123456789012:role/sample-dev-tfc-provisioner-role"},
	}}
	hcpClient := &fakeHCPBindingClient{
		workspace: &hcpapi.Workspace{ID: "ws-dev", Name: "sample-dev"},
	}

	exec := newExecutor(
		&fakeHandler{kind: schema.KindAWSIAMProvisioner},
		&fakeHandler{kind: schema.KindHCPTFWorkspace},
	)
	exec.bindings.newRoleClient = func(string) roleARNLookupClient { return roleClient }
	exec.bindings.newHCPClient = func() (hcpWorkspaceVariableClient, error) { return hcpClient, nil }

	plan := &reconcile.Plan{
		Target: reconcile.TargetRemote,
		Changes: []reconcile.ResourceChange{
			{
				Source: "stack/aws-iam-provisioner-dev-tfc.yaml",
				Action: reconcile.ActionCreate,
				Manifest: &schema.Manifest{
					Kind:     schema.KindAWSIAMProvisioner,
					Metadata: schema.Metadata{Name: "sample-dev-tfc"},
					Spec: &schema.AWSIAMProvisionerSpec{
						Name:         "sample-dev-tfc-provisioner-role",
						OIDCProvider: "app.terraform.io",
						OIDCSubject:  "organization:emkaytec:project:*:workspace:sample-dev:run_phase:*",
					},
				},
			},
			{
				Source: "stack/hcp-tf-workspace-dev.yaml",
				Action: reconcile.ActionCreate,
				Manifest: &schema.Manifest{
					Kind:     schema.KindHCPTFWorkspace,
					Metadata: schema.Metadata{Name: "sample-dev"},
					Spec: &schema.HCPTFWorkspaceSpec{
						Name:         "sample-dev",
						Environment:  "dev",
						Organization: "emkaytec",
					},
				},
			},
		},
	}

	result, err := exec.Apply(context.Background(), plan, reconcile.ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("unexpected binding failure: %+v", result.Failed)
	}
	if hcpClient.createdVariable == nil {
		t.Fatal("expected HCP workspace variable to be created")
	}

	variable := hcpClient.createdVariable
	if variable.Key != "AWS_PROVISIONER_ROLE_ARN_DEV" {
		t.Fatalf("variable key = %q, want AWS_PROVISIONER_ROLE_ARN_DEV", variable.Key)
	}
	if variable.Value != "arn:aws:iam::123456789012:role/sample-dev-tfc-provisioner-role" {
		t.Fatalf("variable value = %q", variable.Value)
	}
	if variable.Category != "env" || variable.HCL || variable.Sensitive {
		t.Fatalf("unexpected variable shape: %#v", variable)
	}
}

func TestRemoteExecutorUpdatesTFCProvisionerRoleARNWhenCreateReportsAlreadyExists(t *testing.T) {
	roleClient := &fakeRoleBindingClient{roles: map[string]*iamcli.Role{
		"sample-dev-tfc-provisioner-role": {ARN: "arn:aws:iam::123456789012:role/sample-dev-tfc-provisioner-role"},
	}}
	hcpClient := &fakeHCPBindingClient{
		workspace: &hcpapi.Workspace{ID: "ws-dev", Name: "sample-dev"},
		variableLists: [][]hcpapi.WorkspaceVariable{
			nil,
			{{
				ID:       "var-123",
				Key:      "AWS_PROVISIONER_ROLE_ARN_DEV",
				Value:    "old",
				Category: "env",
			}},
		},
		createErr: &hcpapi.APIError{StatusCode: http.StatusUnprocessableEntity, Method: http.MethodPost, Path: "/workspaces/ws-dev/vars", Message: "Key has already been taken"},
	}

	exec := newExecutor(
		&fakeHandler{kind: schema.KindAWSIAMProvisioner},
		&fakeHandler{kind: schema.KindHCPTFWorkspace},
	)
	exec.bindings.newRoleClient = func(string) roleARNLookupClient { return roleClient }
	exec.bindings.newHCPClient = func() (hcpWorkspaceVariableClient, error) { return hcpClient, nil }

	plan := &reconcile.Plan{
		Target: reconcile.TargetRemote,
		Changes: []reconcile.ResourceChange{
			{
				Source: "stack/aws-iam-provisioner-dev-tfc.yaml",
				Action: reconcile.ActionCreate,
				Manifest: &schema.Manifest{
					Kind:     schema.KindAWSIAMProvisioner,
					Metadata: schema.Metadata{Name: "sample-dev-tfc"},
					Spec: &schema.AWSIAMProvisionerSpec{
						Name:         "sample-dev-tfc-provisioner-role",
						OIDCProvider: "app.terraform.io",
						OIDCSubject:  "organization:emkaytec:project:*:workspace:sample-dev:run_phase:*",
					},
				},
			},
			{
				Source: "stack/hcp-tf-workspace-dev.yaml",
				Action: reconcile.ActionNoOp,
				Manifest: &schema.Manifest{
					Kind:     schema.KindHCPTFWorkspace,
					Metadata: schema.Metadata{Name: "sample-dev"},
					Spec: &schema.HCPTFWorkspaceSpec{
						Name:         "sample-dev",
						Environment:  "dev",
						Organization: "emkaytec",
					},
				},
			},
		},
	}

	result, err := exec.Apply(context.Background(), plan, reconcile.ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("unexpected binding failure: %+v", result.Failed)
	}
	if hcpClient.updatedVariable == nil {
		t.Fatal("expected existing HCP workspace variable to be updated")
	}
	if hcpClient.updatedVariableID != "var-123" {
		t.Fatalf("updated variable id = %q, want var-123", hcpClient.updatedVariableID)
	}
	if hcpClient.updatedVariable.Value != "arn:aws:iam::123456789012:role/sample-dev-tfc-provisioner-role" {
		t.Fatalf("updated variable value = %q", hcpClient.updatedVariable.Value)
	}
}

func TestRemoteExecutorWiresGitHubProvisionerRoleARNToRepositoryVariable(t *testing.T) {
	roleClient := &fakeRoleBindingClient{roles: map[string]*iamcli.Role{
		"sample-dev-gha-provisioner-role": {ARN: "arn:aws:iam::123456789012:role/sample-dev-gha-provisioner-role"},
	}}
	githubClient := &fakeGitHubBindingClient{}

	exec := newExecutor(
		&fakeHandler{kind: schema.KindAWSIAMProvisioner},
		&fakeHandler{kind: schema.KindGitHubRepo},
	)
	exec.bindings.newRoleClient = func(string) roleARNLookupClient { return roleClient }
	exec.bindings.newGitHubClient = func() (githubRepositoryVariableClient, error) { return githubClient, nil }

	plan := &reconcile.Plan{
		Target: reconcile.TargetRemote,
		Changes: []reconcile.ResourceChange{
			{
				Source: "stack/aws-iam-provisioner-dev-gha.yaml",
				Action: reconcile.ActionCreate,
				Manifest: &schema.Manifest{
					Kind:     schema.KindAWSIAMProvisioner,
					Metadata: schema.Metadata{Name: "sample-dev-gha"},
					Spec: &schema.AWSIAMProvisionerSpec{
						Name:         "sample-dev-gha-provisioner-role",
						OIDCProvider: "token.actions.githubusercontent.com",
						OIDCSubject:  "repo:emkaytec/sample:*",
					},
				},
			},
			{
				Source: "stack/github-repo.yaml",
				Action: reconcile.ActionCreate,
				Manifest: &schema.Manifest{
					Kind:     schema.KindGitHubRepo,
					Metadata: schema.Metadata{Name: "emkaytec-sample"},
					Spec: &schema.GitHubRepoSpec{
						Owner: "emkaytec",
						Name:  "sample",
					},
				},
			},
		},
	}

	result, err := exec.Apply(context.Background(), plan, reconcile.ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("unexpected binding failure: %+v", result.Failed)
	}
	if githubClient.createdVariable == nil {
		t.Fatal("expected GitHub repository variable to be created")
	}

	variable := githubClient.createdVariable
	if variable.Name != "AWS_PROVISIONER_ROLE_ARN_DEV" {
		t.Fatalf("variable name = %q, want AWS_PROVISIONER_ROLE_ARN_DEV", variable.Name)
	}
	if variable.Value != "arn:aws:iam::123456789012:role/sample-dev-gha-provisioner-role" {
		t.Fatalf("variable value = %q", variable.Value)
	}
}

func TestRemoteExecutorUpdatesGitHubProvisionerRoleARNWhenCreateReportsAlreadyExists(t *testing.T) {
	roleClient := &fakeRoleBindingClient{roles: map[string]*iamcli.Role{
		"sample-dev-gha-provisioner-role": {ARN: "arn:aws:iam::123456789012:role/sample-dev-gha-provisioner-role"},
	}}
	githubClient := &fakeGitHubBindingClient{
		createErr: &ghapi.APIError{StatusCode: http.StatusConflict, Method: http.MethodPost, Path: "/repos/emkaytec/sample/actions/variables", Message: "Already exists - Variable already exists"},
	}

	exec := newExecutor(
		&fakeHandler{kind: schema.KindAWSIAMProvisioner},
		&fakeHandler{kind: schema.KindGitHubRepo},
	)
	exec.bindings.newRoleClient = func(string) roleARNLookupClient { return roleClient }
	exec.bindings.newGitHubClient = func() (githubRepositoryVariableClient, error) { return githubClient, nil }

	plan := &reconcile.Plan{
		Target: reconcile.TargetRemote,
		Changes: []reconcile.ResourceChange{
			{
				Source: "stack/aws-iam-provisioner-dev-gha.yaml",
				Action: reconcile.ActionCreate,
				Manifest: &schema.Manifest{
					Kind:     schema.KindAWSIAMProvisioner,
					Metadata: schema.Metadata{Name: "sample-dev-gha"},
					Spec: &schema.AWSIAMProvisionerSpec{
						Name:         "sample-dev-gha-provisioner-role",
						OIDCProvider: "token.actions.githubusercontent.com",
						OIDCSubject:  "repo:emkaytec/sample:*",
					},
				},
			},
			{
				Source: "stack/github-repo.yaml",
				Action: reconcile.ActionNoOp,
				Manifest: &schema.Manifest{
					Kind:     schema.KindGitHubRepo,
					Metadata: schema.Metadata{Name: "emkaytec-sample"},
					Spec: &schema.GitHubRepoSpec{
						Owner: "emkaytec",
						Name:  "sample",
					},
				},
			},
		},
	}

	result, err := exec.Apply(context.Background(), plan, reconcile.ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("unexpected binding failure: %+v", result.Failed)
	}
	if githubClient.updatedVariable == nil {
		t.Fatal("expected existing GitHub repository variable to be updated")
	}
	if githubClient.updatedVariable.Name != "AWS_PROVISIONER_ROLE_ARN_DEV" {
		t.Fatalf("updated variable name = %q", githubClient.updatedVariable.Name)
	}
	if githubClient.updatedVariable.Value != "arn:aws:iam::123456789012:role/sample-dev-gha-provisioner-role" {
		t.Fatalf("updated variable value = %q", githubClient.updatedVariable.Value)
	}
}
