package hcptfworkspace

import (
	"context"
	"testing"

	hcpapi "github.com/emkaytec/forge/internal/hcpterraform"
	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

type fakeClient struct {
	workspace *hcpapi.Workspace
	project   *hcpapi.Project
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

func TestDescribeChangeDetectsWorkspaceDrift(t *testing.T) {
	fake := &fakeClient{
		workspace: &hcpapi.Workspace{
			ID:               "ws-123",
			Name:             "sample",
			ExecutionMode:    "remote",
			TerraformVersion: "1.7.0",
			ProjectID:        "prj-old",
			VCSRepo:          &hcpapi.WorkspaceVCSRepo{Identifier: "emkaytec/old"},
		},
		project: &hcpapi.Project{ID: "prj-new", Name: "platform"},
	}
	handler := New(WithClientFactory(func() (client, error) { return fake, nil }))

	change, err := handler.DescribeChange(context.Background(), &schema.Manifest{
		Kind:     schema.KindHCPTFWorkspace,
		Metadata: schema.Metadata{Name: "sample"},
		Spec: &schema.HCPTFWorkspaceSpec{
			Name:             "sample",
			Organization:     "emkaytec",
			Project:          "platform",
			VCSRepo:          "emkaytec/forge",
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
