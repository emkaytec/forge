// Package hcptfworkspace hosts the remote reconcile handler for the
// HCPTerraformWorkspace kind.
package hcptfworkspace

import (
	"context"
	"fmt"
	"strings"

	hcpapi "github.com/emkaytec/forge/internal/hcpterraform"
	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

type client interface {
	GetWorkspace(ctx context.Context, organization string, name string) (*hcpapi.Workspace, error)
	CreateWorkspace(ctx context.Context, organization string, name string, request hcpapi.WorkspaceRequest) (*hcpapi.Workspace, error)
	UpdateWorkspace(ctx context.Context, workspaceID string, request hcpapi.WorkspaceRequest) (*hcpapi.Workspace, error)
	FindProjectByName(ctx context.Context, organization string, name string) (*hcpapi.Project, error)
}

// Handler implements the HCPTerraformWorkspace remote handler contract.
type Handler struct {
	newClient func() (client, error)
}

type Option func(*Handler)

// New returns a new handler.
func New(opts ...Option) *Handler {
	handler := &Handler{
		newClient: func() (client, error) {
			return hcpapi.NewClientFromEnv()
		},
	}

	for _, opt := range opts {
		opt(handler)
	}

	return handler
}

func WithClientFactory(factory func() (client, error)) Option {
	return func(handler *Handler) {
		handler.newClient = factory
	}
}

// Kind reports schema.KindHCPTFWorkspace.
func (h *Handler) Kind() schema.Kind { return schema.KindHCPTFWorkspace }

func (h *Handler) DescribeChange(ctx context.Context, m *schema.Manifest, _ string) (reconcile.ResourceChange, error) {
	spec, ok := m.Spec.(*schema.HCPTFWorkspaceSpec)
	if !ok {
		return reconcile.ResourceChange{}, fmt.Errorf("HCPTerraformWorkspace: unexpected spec type %T", m.Spec)
	}

	client, err := h.newClient()
	if err != nil {
		return reconcile.ResourceChange{}, err
	}

	var projectID string
	if strings.TrimSpace(spec.Project) != "" {
		project, err := client.FindProjectByName(ctx, spec.Organization, spec.Project)
		if err != nil {
			return reconcile.ResourceChange{}, err
		}
		projectID = project.ID
	}

	change := reconcile.ResourceChange{Manifest: m}
	workspace, err := client.GetWorkspace(ctx, spec.Organization, spec.Name)
	switch {
	case hcpapi.IsNotFound(err):
		change.Action = reconcile.ActionCreate
		if projectID != "" {
			change.Note = "project " + spec.Project
		}
		return change, nil
	case err != nil:
		return reconcile.ResourceChange{}, err
	}

	if workspace.ExecutionMode != spec.ExecutionMode {
		change.Drift = append(change.Drift, reconcile.DriftField{
			Path:     "spec.execution_mode",
			Desired:  spec.ExecutionMode,
			Observed: workspace.ExecutionMode,
		})
	}
	if spec.TerraformVersion != "" && workspace.TerraformVersion != spec.TerraformVersion {
		change.Drift = append(change.Drift, reconcile.DriftField{
			Path:     "spec.terraform_version",
			Desired:  spec.TerraformVersion,
			Observed: workspace.TerraformVersion,
		})
	}
	if spec.VCSRepo != "" && currentVCSIdentifier(workspace) != spec.VCSRepo {
		change.Drift = append(change.Drift, reconcile.DriftField{
			Path:     "spec.vcs_repo",
			Desired:  spec.VCSRepo,
			Observed: currentVCSIdentifier(workspace),
		})
	}
	if projectID != "" && workspace.ProjectID != projectID {
		change.Drift = append(change.Drift, reconcile.DriftField{
			Path:     "spec.project",
			Desired:  spec.Project,
			Observed: workspace.ProjectID,
		})
	}

	if len(change.Drift) == 0 {
		change.Action = reconcile.ActionNoOp
		return change, nil
	}

	change.Action = reconcile.ActionUpdate
	return change, nil
}

func (h *Handler) Apply(ctx context.Context, change reconcile.ResourceChange, _ reconcile.ApplyOptions) error {
	spec, ok := change.Manifest.Spec.(*schema.HCPTFWorkspaceSpec)
	if !ok {
		return fmt.Errorf("HCPTerraformWorkspace: unexpected spec type %T", change.Manifest.Spec)
	}

	client, err := h.newClient()
	if err != nil {
		return err
	}

	projectID, err := lookupProjectID(ctx, client, spec)
	if err != nil {
		return err
	}

	request := workspaceRequestFromSpec(spec, projectID)
	workspace, err := client.GetWorkspace(ctx, spec.Organization, spec.Name)
	switch {
	case hcpapi.IsNotFound(err):
		_, err = client.CreateWorkspace(ctx, spec.Organization, spec.Name, request)
		return err
	case err != nil:
		return err
	}

	updateRequest := workspaceUpdateRequest(workspace, spec, projectID)
	if updateRequest == nil {
		return nil
	}

	_, err = client.UpdateWorkspace(ctx, workspace.ID, *updateRequest)
	return err
}

func workspaceRequestFromSpec(spec *schema.HCPTFWorkspaceSpec, projectID string) hcpapi.WorkspaceRequest {
	request := hcpapi.WorkspaceRequest{
		ExecutionMode: stringPtr(spec.ExecutionMode),
	}
	if spec.TerraformVersion != "" {
		request.TerraformVersion = stringPtr(spec.TerraformVersion)
	}
	if spec.VCSRepo != "" {
		request.VCSRepo = &hcpapi.WorkspaceVCSRepo{Identifier: spec.VCSRepo}
	}
	if projectID != "" {
		request.ProjectID = stringPtr(projectID)
	}
	return request
}

func workspaceUpdateRequest(workspace *hcpapi.Workspace, spec *schema.HCPTFWorkspaceSpec, projectID string) *hcpapi.WorkspaceRequest {
	request := &hcpapi.WorkspaceRequest{}

	if workspace.ExecutionMode != spec.ExecutionMode {
		request.ExecutionMode = stringPtr(spec.ExecutionMode)
	}
	if spec.TerraformVersion != "" && workspace.TerraformVersion != spec.TerraformVersion {
		request.TerraformVersion = stringPtr(spec.TerraformVersion)
	}
	if spec.VCSRepo != "" && currentVCSIdentifier(workspace) != spec.VCSRepo {
		request.VCSRepo = &hcpapi.WorkspaceVCSRepo{Identifier: spec.VCSRepo}
	}
	if projectID != "" && workspace.ProjectID != projectID {
		request.ProjectID = stringPtr(projectID)
	}

	if request.ExecutionMode == nil && request.TerraformVersion == nil && request.VCSRepo == nil && request.ProjectID == nil {
		return nil
	}

	return request
}

func currentVCSIdentifier(workspace *hcpapi.Workspace) string {
	if workspace.VCSRepo == nil {
		return ""
	}
	return workspace.VCSRepo.Identifier
}

func lookupProjectID(ctx context.Context, client client, spec *schema.HCPTFWorkspaceSpec) (string, error) {
	if strings.TrimSpace(spec.Project) == "" {
		return "", nil
	}

	project, err := client.FindProjectByName(ctx, spec.Organization, spec.Project)
	if err != nil {
		return "", err
	}
	return project.ID, nil
}

func stringPtr(value string) *string {
	return &value
}
