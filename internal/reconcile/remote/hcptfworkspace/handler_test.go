package hcptfworkspace

import (
	"context"
	"testing"

	hcpapi "github.com/emkaytec/forge/internal/hcpterraform"
	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

type fakeClient struct {
	workspace         *hcpapi.Workspace
	project           *hcpapi.Project
	variables         []hcpapi.WorkspaceVariable
	createdVariable   *hcpapi.WorkspaceVariable
	updatedVariable   *hcpapi.WorkspaceVariable
	updatedVariableID string
}

func (f *fakeClient) GetWorkspace(context.Context, string, string) (*hcpapi.Workspace, error) {
	return f.workspace, nil
}

func (f *fakeClient) CreateWorkspace(context.Context, string, string, hcpapi.WorkspaceRequest) (*hcpapi.Workspace, error) {
	return f.workspace, nil
}

func (f *fakeClient) UpdateWorkspace(context.Context, string, hcpapi.WorkspaceRequest) (*hcpapi.Workspace, error) {
	return f.workspace, nil
}

func (f *fakeClient) FindProjectByName(context.Context, string, string) (*hcpapi.Project, error) {
	return f.project, nil
}

func (f *fakeClient) ListVariables(context.Context, string) ([]hcpapi.WorkspaceVariable, error) {
	return f.variables, nil
}

func (f *fakeClient) CreateVariable(_ context.Context, _ string, variable hcpapi.WorkspaceVariable) error {
	copied := variable
	f.createdVariable = &copied
	return nil
}

func (f *fakeClient) UpdateVariable(_ context.Context, _ string, variableID string, variable hcpapi.WorkspaceVariable) error {
	copied := variable
	f.updatedVariable = &copied
	f.updatedVariableID = variableID
	return nil
}

func TestDescribeChangeDetectsWorkspaceDrift(t *testing.T) {
	fake := &fakeClient{
		workspace: &hcpapi.Workspace{
			ID:               "ws-123",
			Name:             "sample",
			ExecutionMode:    "remote",
			TerraformVersion: "1.7.0",
			ProjectID:        "prj-old",
		},
		project: &hcpapi.Project{ID: "prj-new", Name: "platform"},
	}
	handler := New(WithClientFactory(func() (client, error) { return fake, nil }))

	change, err := handler.DescribeChange(context.Background(), &schema.Manifest{
		Kind:     schema.KindHCPTFWorkspace,
		Metadata: schema.Metadata{Name: "sample"},
		Spec: &schema.HCPTFWorkspaceSpec{
			Name:             "sample",
			Environment:      "dev",
			Organization:     "emkaytec",
			Project:          "platform",
			AccountID:        "123456789012",
			ExecutionMode:    "agent",
			TerraformVersion: "1.9.0",
		},
	}, "sample.yaml")
	if err != nil {
		t.Fatalf("DescribeChange() error = %v", err)
	}
	if change.Action != reconcile.ActionUpdate {
		t.Fatalf("action = %q, want update", change.Action)
	}
	if len(change.Drift) != 4 {
		t.Fatalf("len(drift) = %d, want 4", len(change.Drift))
	}
}

func TestApplyCreatesAccountVariableWhenMissing(t *testing.T) {
	fake := &fakeClient{
		workspace: &hcpapi.Workspace{
			ID:            "ws-123",
			Name:          "sample-dev",
			ExecutionMode: "remote",
		},
	}
	handler := New(WithClientFactory(func() (client, error) { return fake, nil }))

	err := handler.Apply(context.Background(), reconcile.ResourceChange{
		Manifest: &schema.Manifest{
			Kind:     schema.KindHCPTFWorkspace,
			Metadata: schema.Metadata{Name: "sample-dev"},
			Spec: &schema.HCPTFWorkspaceSpec{
				Name:          "sample-dev",
				Environment:   "dev",
				Organization:  "emkaytec",
				AccountID:     "123456789012",
				ExecutionMode: "remote",
			},
		},
	}, reconcile.ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if fake.createdVariable == nil {
		t.Fatal("expected account_id variable to be created")
	}
	if fake.createdVariable.Key != "account_id" || fake.createdVariable.Value != `"123456789012"` || fake.createdVariable.Category != "terraform" || !fake.createdVariable.HCL {
		t.Fatalf("unexpected created variable: %#v", fake.createdVariable)
	}
}

func TestApplyUpdatesAccountVariableWhenChanged(t *testing.T) {
	fake := &fakeClient{
		workspace: &hcpapi.Workspace{
			ID:            "ws-123",
			Name:          "sample-dev",
			ExecutionMode: "remote",
		},
		variables: []hcpapi.WorkspaceVariable{{
			ID:       "var-123",
			Key:      "account_id",
			Value:    `"999999999999"`,
			Category: "terraform",
			HCL:      true,
		}},
	}
	handler := New(WithClientFactory(func() (client, error) { return fake, nil }))

	err := handler.Apply(context.Background(), reconcile.ResourceChange{
		Manifest: &schema.Manifest{
			Kind:     schema.KindHCPTFWorkspace,
			Metadata: schema.Metadata{Name: "sample-dev"},
			Spec: &schema.HCPTFWorkspaceSpec{
				Name:          "sample-dev",
				Environment:   "dev",
				Organization:  "emkaytec",
				AccountID:     "123456789012",
				ExecutionMode: "remote",
			},
		},
	}, reconcile.ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if fake.updatedVariable == nil {
		t.Fatal("expected account_id variable to be updated")
	}
	if fake.updatedVariableID != "var-123" {
		t.Fatalf("updated variable id = %q, want var-123", fake.updatedVariableID)
	}
	if fake.updatedVariable.Key != "account_id" || fake.updatedVariable.Value != `"123456789012"` || fake.updatedVariable.Category != "terraform" || !fake.updatedVariable.HCL {
		t.Fatalf("unexpected updated variable: %#v", fake.updatedVariable)
	}
}
