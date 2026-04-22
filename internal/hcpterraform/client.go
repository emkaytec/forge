package hcpterraform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const DefaultBaseURL = "https://app.terraform.io/api/v2"

type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("hcp terraform api %s %s returned %d: %s", e.Method, e.Path, e.StatusCode, e.Message)
	}

	return fmt.Sprintf("hcp terraform api %s %s returned %d", e.Method, e.Path, e.StatusCode)
}

func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

func NewClientFromEnv() (*Client, error) {
	token := strings.TrimSpace(os.Getenv("TF_TOKEN_app_terraform_io"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("TFE_TOKEN"))
	}
	if token == "" {
		return nil, fmt.Errorf("missing HCP Terraform token: set TF_TOKEN_app_terraform_io or TFE_TOKEN")
	}

	return NewClient(DefaultBaseURL, token, nil), nil
}

func NewClient(baseURL string, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
		token:      token,
	}
}

type Workspace struct {
	ID               string
	Name             string
	ExecutionMode    string
	TerraformVersion string
	VCSRepo          *WorkspaceVCSRepo
	ProjectID        string
}

type WorkspaceVCSRepo struct {
	Identifier string `json:"identifier"`
}

type Project struct {
	ID   string
	Name string
}

type WorkspaceVariable struct {
	ID          string
	Key         string
	Value       string
	Description string
	Category    string
	HCL         bool
	Sensitive   bool
}

type WorkspaceRequest struct {
	ExecutionMode    *string
	TerraformVersion *string
	VCSRepo          *WorkspaceVCSRepo
	ProjectID        *string
}

func (c *Client) GetWorkspace(ctx context.Context, organization string, name string) (*Workspace, error) {
	path := fmt.Sprintf("/organizations/%s/workspaces/%s", url.PathEscape(organization), url.PathEscape(name))
	var response struct {
		Data workspaceResponseData `json:"data"`
	}

	if err := c.request(ctx, http.MethodGet, path, nil, &response, http.StatusOK); err != nil {
		return nil, err
	}

	return response.Data.toWorkspace(), nil
}

func (c *Client) CreateWorkspace(ctx context.Context, organization string, name string, request WorkspaceRequest) (*Workspace, error) {
	path := fmt.Sprintf("/organizations/%s/workspaces", url.PathEscape(organization))
	var response struct {
		Data workspaceResponseData `json:"data"`
	}

	if err := c.request(ctx, http.MethodPost, path, buildWorkspacePayload(name, request), &response, http.StatusCreated, http.StatusOK); err != nil {
		return nil, err
	}

	return response.Data.toWorkspace(), nil
}

func (c *Client) UpdateWorkspace(ctx context.Context, workspaceID string, request WorkspaceRequest) (*Workspace, error) {
	path := fmt.Sprintf("/workspaces/%s", url.PathEscape(workspaceID))
	var response struct {
		Data workspaceResponseData `json:"data"`
	}

	if err := c.request(ctx, http.MethodPatch, path, buildWorkspacePayload("", request), &response, http.StatusOK); err != nil {
		return nil, err
	}

	return response.Data.toWorkspace(), nil
}

func (c *Client) FindProjectByName(ctx context.Context, organization string, name string) (*Project, error) {
	path := fmt.Sprintf("/organizations/%s/projects?filter[names]=%s", url.PathEscape(organization), url.QueryEscape(name))
	var response struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := c.request(ctx, http.MethodGet, path, nil, &response, http.StatusOK); err != nil {
		return nil, err
	}

	for _, item := range response.Data {
		if strings.EqualFold(strings.TrimSpace(item.Attributes.Name), strings.TrimSpace(name)) {
			return &Project{
				ID:   item.ID,
				Name: item.Attributes.Name,
			}, nil
		}
	}

	return nil, fmt.Errorf("hcp terraform project %q not found in organization %q", name, organization)
}

func (c *Client) ListVariables(ctx context.Context, workspaceID string) ([]WorkspaceVariable, error) {
	path := fmt.Sprintf("/workspaces/%s/vars", url.PathEscape(workspaceID))
	var response struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Key         string `json:"key"`
				Value       string `json:"value"`
				Description string `json:"description"`
				Category    string `json:"category"`
				HCL         bool   `json:"hcl"`
				Sensitive   bool   `json:"sensitive"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := c.request(ctx, http.MethodGet, path, nil, &response, http.StatusOK); err != nil {
		return nil, err
	}

	variables := make([]WorkspaceVariable, 0, len(response.Data))
	for _, item := range response.Data {
		variables = append(variables, WorkspaceVariable{
			ID:          item.ID,
			Key:         item.Attributes.Key,
			Value:       item.Attributes.Value,
			Description: item.Attributes.Description,
			Category:    item.Attributes.Category,
			HCL:         item.Attributes.HCL,
			Sensitive:   item.Attributes.Sensitive,
		})
	}

	return variables, nil
}

func (c *Client) CreateVariable(ctx context.Context, workspaceID string, variable WorkspaceVariable) error {
	path := fmt.Sprintf("/workspaces/%s/vars", url.PathEscape(workspaceID))
	request := map[string]any{
		"data": map[string]any{
			"type": "vars",
			"attributes": map[string]any{
				"key":         variable.Key,
				"value":       variable.Value,
				"description": variable.Description,
				"category":    variable.Category,
				"hcl":         variable.HCL,
				"sensitive":   variable.Sensitive,
			},
		},
	}

	return c.request(ctx, http.MethodPost, path, request, nil, http.StatusCreated, http.StatusOK)
}

func (c *Client) UpdateVariable(ctx context.Context, workspaceID string, variableID string, variable WorkspaceVariable) error {
	path := fmt.Sprintf("/workspaces/%s/vars/%s", url.PathEscape(workspaceID), url.PathEscape(variableID))
	request := map[string]any{
		"data": map[string]any{
			"id":   variableID,
			"type": "vars",
			"attributes": map[string]any{
				"key":         variable.Key,
				"value":       variable.Value,
				"description": variable.Description,
				"category":    variable.Category,
				"hcl":         variable.HCL,
				"sensitive":   variable.Sensitive,
			},
		},
	}

	return c.request(ctx, http.MethodPatch, path, request, nil, http.StatusOK)
}

type workspaceResponseData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name             string            `json:"name"`
		ExecutionMode    string            `json:"execution-mode"`
		TerraformVersion string            `json:"terraform-version"`
		VCSRepo          *WorkspaceVCSRepo `json:"vcs-repo"`
	} `json:"attributes"`
	Relationships struct {
		Project struct {
			Data *struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"project"`
	} `json:"relationships"`
}

func (d workspaceResponseData) toWorkspace() *Workspace {
	workspace := &Workspace{
		ID:               d.ID,
		Name:             d.Attributes.Name,
		ExecutionMode:    d.Attributes.ExecutionMode,
		TerraformVersion: d.Attributes.TerraformVersion,
		VCSRepo:          d.Attributes.VCSRepo,
	}
	if d.Relationships.Project.Data != nil {
		workspace.ProjectID = d.Relationships.Project.Data.ID
	}
	return workspace
}

func buildWorkspacePayload(name string, request WorkspaceRequest) map[string]any {
	attributes := make(map[string]any)
	if name != "" {
		attributes["name"] = name
	}
	if request.ExecutionMode != nil {
		attributes["execution-mode"] = *request.ExecutionMode
	}
	if request.TerraformVersion != nil {
		attributes["terraform-version"] = *request.TerraformVersion
	}
	if request.VCSRepo != nil {
		attributes["vcs-repo"] = request.VCSRepo
	}

	data := map[string]any{
		"type": "workspaces",
	}
	if len(attributes) > 0 {
		data["attributes"] = attributes
	}
	if request.ProjectID != nil {
		data["relationships"] = map[string]any{
			"project": map[string]any{
				"data": map[string]any{
					"type": "projects",
					"id":   *request.ProjectID,
				},
			},
		}
	}

	return map[string]any{"data": data}
}

func (c *Client) request(ctx context.Context, method string, path string, requestBody any, responseBody any, expectedStatusCodes ...int) error {
	var body io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("marshal hcp terraform request %s %s: %w", method, path, err)
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build hcp terraform request %s %s: %w", method, path, err)
	}

	req.Header.Set("Accept", "application/vnd.api+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/vnd.api+json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute hcp terraform request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	for _, expectedStatus := range expectedStatusCodes {
		if resp.StatusCode == expectedStatus {
			if responseBody == nil || resp.StatusCode == http.StatusNoContent {
				io.Copy(io.Discard, resp.Body)
				return nil
			}

			if err := json.NewDecoder(resp.Body).Decode(responseBody); err != nil {
				return fmt.Errorf("decode hcp terraform response %s %s: %w", method, path, err)
			}

			return nil
		}
	}

	var apiErrBody struct {
		Errors []struct {
			Detail string `json:"detail"`
			Title  string `json:"title"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiErrBody); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode hcp terraform error response %s %s: %w", method, path, err)
	}

	message := ""
	if len(apiErrBody.Errors) > 0 {
		message = apiErrBody.Errors[0].Detail
		if message == "" {
			message = apiErrBody.Errors[0].Title
		}
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		Method:     method,
		Path:       path,
		Message:    message,
	}
}
