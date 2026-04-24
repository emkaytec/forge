package github

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

const (
	DefaultBaseURL    = "https://api.github.com"
	defaultAPIVersion = "2022-11-28"
)

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
		return fmt.Sprintf("github api %s %s returned %d: %s", e.Method, e.Path, e.StatusCode, e.Message)
	}

	return fmt.Sprintf("github api %s %s returned %d", e.Method, e.Path, e.StatusCode)
}

func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

func IsAlreadyExists(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	message := strings.ToLower(apiErr.Message)
	return apiErr.StatusCode == http.StatusConflict ||
		strings.Contains(message, "already exists") ||
		strings.Contains(message, "already been taken")
}

func NewClientFromEnv() (*Client, error) {
	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GH_TOKEN"))
	}
	if token == "" {
		token = lookupGHToken()
	}
	if token == "" {
		return nil, fmt.Errorf("missing GitHub token: set GITHUB_TOKEN or GH_TOKEN, or run `gh auth login`")
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

type Account struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

type Repository struct {
	Name             string   `json:"name"`
	FullName         string   `json:"full_name"`
	Visibility       string   `json:"visibility"`
	Description      *string  `json:"description"`
	DefaultBranch    string   `json:"default_branch"`
	Topics           []string `json:"topics"`
	HasIssues        bool     `json:"has_issues"`
	HasProjects      bool     `json:"has_projects"`
	HasWiki          bool     `json:"has_wiki"`
	Owner            Account  `json:"owner"`
	AllowAutoMerge   bool     `json:"allow_auto_merge"`
	AllowRebaseMerge bool     `json:"allow_rebase_merge"`
}

type CreateRepositoryRequest struct {
	Name        string  `json:"name"`
	Visibility  *string `json:"visibility,omitempty"`
	Description *string `json:"description,omitempty"`
	AutoInit    bool    `json:"auto_init,omitempty"`
}

type UpdateRepositoryRequest struct {
	Visibility    *string `json:"visibility,omitempty"`
	Description   *string `json:"description,omitempty"`
	DefaultBranch *string `json:"default_branch,omitempty"`
}

func (r UpdateRepositoryRequest) IsZero() bool {
	return r.Visibility == nil && r.Description == nil && r.DefaultBranch == nil
}

type TopicsResponse struct {
	Names []string `json:"names"`
}

type RepositoryVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (c *Client) GetAuthenticatedUser(ctx context.Context) (*Account, error) {
	var account Account
	if err := c.request(ctx, http.MethodGet, "/user", nil, &account, http.StatusOK); err != nil {
		return nil, err
	}

	return &account, nil
}

func (c *Client) GetRepository(ctx context.Context, owner string, repo string) (*Repository, error) {
	var repository Repository
	if err := c.request(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo)), nil, &repository, http.StatusOK); err != nil {
		return nil, err
	}

	return &repository, nil
}

func (c *Client) GetAccount(ctx context.Context, owner string) (*Account, error) {
	var account Account
	if err := c.request(ctx, http.MethodGet, fmt.Sprintf("/users/%s", url.PathEscape(owner)), nil, &account, http.StatusOK); err != nil {
		return nil, err
	}

	return &account, nil
}

// ListUserOrganizations returns the organizations the authenticated user
// is a member of. Only the first page (up to 100 entries) is returned —
// the forge selector UX degrades gracefully past that and no one
// operating a forge CLI has more than 100 GitHub orgs.
func (c *Client) ListUserOrganizations(ctx context.Context) ([]Account, error) {
	var accounts []Account
	if err := c.request(ctx, http.MethodGet, "/user/orgs?per_page=100", nil, &accounts, http.StatusOK); err != nil {
		return nil, err
	}

	return accounts, nil
}

func (c *Client) CreateOrganizationRepository(ctx context.Context, org string, request CreateRepositoryRequest) (*Repository, error) {
	var repository Repository
	if err := c.request(ctx, http.MethodPost, fmt.Sprintf("/orgs/%s/repos", url.PathEscape(org)), request, &repository, http.StatusCreated); err != nil {
		return nil, err
	}

	return &repository, nil
}

func (c *Client) CreateUserRepository(ctx context.Context, request CreateRepositoryRequest) (*Repository, error) {
	var repository Repository
	if err := c.request(ctx, http.MethodPost, "/user/repos", request, &repository, http.StatusCreated); err != nil {
		return nil, err
	}

	return &repository, nil
}

func (c *Client) UpdateRepository(ctx context.Context, owner string, repo string, request UpdateRepositoryRequest) (*Repository, error) {
	var repository Repository
	if err := c.request(ctx, http.MethodPatch, fmt.Sprintf("/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo)), request, &repository, http.StatusOK); err != nil {
		return nil, err
	}

	return &repository, nil
}

func (c *Client) ReplaceTopics(ctx context.Context, owner string, repo string, topics []string) error {
	request := TopicsResponse{Names: topics}
	return c.request(ctx, http.MethodPut, fmt.Sprintf("/repos/%s/%s/topics", url.PathEscape(owner), url.PathEscape(repo)), request, nil, http.StatusOK)
}

func (c *Client) GetRepositoryVariable(ctx context.Context, owner string, repo string, name string) (*RepositoryVariable, error) {
	var variable RepositoryVariable
	if err := c.request(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/actions/variables/%s", url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(name)), nil, &variable, http.StatusOK); err != nil {
		return nil, err
	}

	return &variable, nil
}

func (c *Client) CreateRepositoryVariable(ctx context.Context, owner string, repo string, variable RepositoryVariable) error {
	return c.request(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/actions/variables", url.PathEscape(owner), url.PathEscape(repo)), variable, nil, http.StatusCreated)
}

func (c *Client) UpdateRepositoryVariable(ctx context.Context, owner string, repo string, name string, variable RepositoryVariable) error {
	return c.request(ctx, http.MethodPatch, fmt.Sprintf("/repos/%s/%s/actions/variables/%s", url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(name)), variable, nil, http.StatusNoContent, http.StatusOK)
}

func (c *Client) request(ctx context.Context, method string, path string, requestBody any, responseBody any, expectedStatusCodes ...int) error {
	var body io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("marshal github request %s %s: %w", method, path, err)
		}

		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build github request %s %s: %w", method, path, err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", defaultAPIVersion)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute github request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	for _, expectedStatus := range expectedStatusCodes {
		if resp.StatusCode == expectedStatus {
			if responseBody == nil || resp.StatusCode == http.StatusNoContent {
				io.Copy(io.Discard, resp.Body)
				return nil
			}

			if err := json.NewDecoder(resp.Body).Decode(responseBody); err != nil {
				return fmt.Errorf("decode github response %s %s: %w", method, path, err)
			}

			return nil
		}
	}

	var apiErrBody struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiErrBody); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode github error response %s %s: %w", method, path, err)
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		Method:     method,
		Path:       path,
		Message:    apiErrBody.Message,
	}
}
