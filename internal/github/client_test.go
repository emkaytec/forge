package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(t *testing.T, statusCode int, body any) *http.Response {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(payload)),
	}
}

// observedTokenClient answers one GET /user call and records the
// Authorization header it received so tests can confirm which token
// landed on the wire.
func observedTokenClient(t *testing.T) (client *http.Client, got *string) {
	t.Helper()

	var header string
	got = &header

	client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodGet {
				t.Fatalf("method = %q, want GET", r.Method)
			}
			if r.URL.Path != "/user" {
				t.Fatalf("path = %q, want /user", r.URL.Path)
			}

			header = r.Header.Get("Authorization")
			return jsonResponse(t, http.StatusOK, Account{Login: "observer", Type: "User"}), nil
		}),
	}

	return client, got
}

func TestNewClientFromEnvPrefersEnvOverGH(t *testing.T) {
	restore := SetLookupGHTokenForTesting(func() string { return "gh-token" })
	t.Cleanup(restore)

	t.Setenv("GITHUB_TOKEN", "env-token")
	t.Setenv("GH_TOKEN", "")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if client.token != "env-token" {
		t.Fatalf("token = %q, want env-token", client.token)
	}

	// Also confirm it actually lands on outgoing requests.
	httpClient, authHeader := observedTokenClient(t)
	client.baseURL = "https://example.test"
	client.httpClient = httpClient
	if _, err := client.GetAuthenticatedUser(context.Background()); err != nil {
		t.Fatalf("GetAuthenticatedUser() error = %v", err)
	}
	if got := *authHeader; got != "Bearer env-token" {
		t.Fatalf("Authorization header = %q, want %q", got, "Bearer env-token")
	}
}

func TestNewClientFromEnvFallsBackToGHCLI(t *testing.T) {
	restore := SetLookupGHTokenForTesting(func() string { return "gh-token" })
	t.Cleanup(restore)

	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}

	httpClient, authHeader := observedTokenClient(t)
	client.baseURL = "https://example.test"
	client.httpClient = httpClient
	if _, err := client.GetAuthenticatedUser(context.Background()); err != nil {
		t.Fatalf("GetAuthenticatedUser() error = %v", err)
	}
	if got := *authHeader; got != "Bearer gh-token" {
		t.Fatalf("Authorization header = %q, want %q", got, "Bearer gh-token")
	}
}

func TestNewClientFromEnvErrorsWhenNothingConfigured(t *testing.T) {
	restore := SetLookupGHTokenForTesting(func() string { return "" })
	t.Cleanup(restore)

	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	_, err := NewClientFromEnv()
	if err == nil {
		t.Fatal("NewClientFromEnv() error = nil, want missing-token error")
	}
	if !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("error = %v, want mention of `gh auth login` in guidance", err)
	}
}

func TestNewClientFromEnvPrefersGHTokenWhenOnlyGHTokenSet(t *testing.T) {
	restore := SetLookupGHTokenForTesting(func() string { return "should-not-be-used" })
	t.Cleanup(restore)

	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "gh-token-env")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if client.token != "gh-token-env" {
		t.Fatalf("token = %q, want gh-token-env (GH_TOKEN should beat gh CLI fallback)", client.token)
	}
}

func TestListUserOrganizationsReturnsAccounts(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodGet {
				t.Fatalf("method = %q, want GET", r.Method)
			}
			if r.URL.Path != "/user/orgs" {
				t.Fatalf("path = %q, want /user/orgs", r.URL.Path)
			}
			if r.URL.RawQuery != "per_page=100" {
				t.Errorf("query = %q, want per_page=100", r.URL.RawQuery)
			}

			return jsonResponse(t, http.StatusOK, []Account{
				{Login: "emkaytec", Type: "Organization"},
				{Login: "some-other-org", Type: "Organization"},
			}), nil
		}),
	}

	client := NewClient("https://example.test", "test-token", httpClient)
	orgs, err := client.ListUserOrganizations(context.Background())
	if err != nil {
		t.Fatalf("ListUserOrganizations() error = %v", err)
	}
	if len(orgs) != 2 || orgs[0].Login != "emkaytec" || orgs[1].Login != "some-other-org" {
		t.Fatalf("orgs = %+v, want emkaytec + some-other-org", orgs)
	}
}

func TestRepositoryVariableMethodsUseActionsVariableEndpoints(t *testing.T) {
	expected := []struct {
		method string
		path   string
		body   RepositoryVariable
		status int
		resp   any
	}{
		{
			method: http.MethodGet,
			path:   "/repos/emkaytec/sample/actions/variables/AWS_PROVISIONER_ROLE_ARN_DEV",
			status: http.StatusOK,
			resp:   RepositoryVariable{Name: "AWS_PROVISIONER_ROLE_ARN_DEV", Value: "old-arn"},
		},
		{
			method: http.MethodPost,
			path:   "/repos/emkaytec/sample/actions/variables",
			body:   RepositoryVariable{Name: "AWS_PROVISIONER_ROLE_ARN_DEV", Value: "new-arn"},
			status: http.StatusCreated,
			resp:   map[string]any{},
		},
		{
			method: http.MethodPatch,
			path:   "/repos/emkaytec/sample/actions/variables/AWS_PROVISIONER_ROLE_ARN_DEV",
			body:   RepositoryVariable{Name: "AWS_PROVISIONER_ROLE_ARN_DEV", Value: "newer-arn"},
			status: http.StatusNoContent,
			resp:   map[string]any{},
		},
	}

	var call int
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if call >= len(expected) {
				t.Fatalf("unexpected extra request: %s %s", r.Method, r.URL.Path)
			}

			want := expected[call]
			call++

			if r.Method != want.method {
				t.Fatalf("method = %q, want %q", r.Method, want.method)
			}
			if r.URL.Path != want.path {
				t.Fatalf("path = %q, want %q", r.URL.Path, want.path)
			}
			if want.body.Name != "" {
				var got RepositoryVariable
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				if got != want.body {
					t.Fatalf("body = %#v, want %#v", got, want.body)
				}
			}

			return jsonResponse(t, want.status, want.resp), nil
		}),
	}

	client := NewClient("https://example.test", "token", httpClient)

	variable, err := client.GetRepositoryVariable(context.Background(), "emkaytec", "sample", "AWS_PROVISIONER_ROLE_ARN_DEV")
	if err != nil {
		t.Fatalf("GetRepositoryVariable() error = %v", err)
	}
	if variable.Name != "AWS_PROVISIONER_ROLE_ARN_DEV" || variable.Value != "old-arn" {
		t.Fatalf("variable = %#v", variable)
	}

	if err := client.CreateRepositoryVariable(context.Background(), "emkaytec", "sample", RepositoryVariable{Name: "AWS_PROVISIONER_ROLE_ARN_DEV", Value: "new-arn"}); err != nil {
		t.Fatalf("CreateRepositoryVariable() error = %v", err)
	}
	if err := client.UpdateRepositoryVariable(context.Background(), "emkaytec", "sample", "AWS_PROVISIONER_ROLE_ARN_DEV", RepositoryVariable{Name: "AWS_PROVISIONER_ROLE_ARN_DEV", Value: "newer-arn"}); err != nil {
		t.Fatalf("UpdateRepositoryVariable() error = %v", err)
	}

	if call != len(expected) {
		t.Fatalf("call count = %d, want %d", call, len(expected))
	}
}

func TestIsAlreadyExistsRecognizesRepositoryVariableConflict(t *testing.T) {
	err := &APIError{
		StatusCode: http.StatusConflict,
		Method:     http.MethodPost,
		Path:       "/repos/emkaytec/sample/actions/variables",
		Message:    "Already exists - Variable already exists",
	}

	if !IsAlreadyExists(err) {
		t.Fatalf("IsAlreadyExists(%v) = false, want true", err)
	}
}
