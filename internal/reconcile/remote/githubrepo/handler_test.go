package githubrepo

import (
	"context"
	"strings"
	"testing"

	ghapi "github.com/emkaytec/forge/internal/github"
	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

type fakeClient struct {
	authenticatedUser *ghapi.Account
	accountType       string
	repository        *ghapi.Repository
	repositoryErr     error
	createdRequest    ghapi.CreateRepositoryRequest
	created           bool
}

func (f *fakeClient) GetAuthenticatedUser(context.Context) (*ghapi.Account, error) {
	return f.authenticatedUser, nil
}

func (f *fakeClient) GetAccount(_ context.Context, owner string) (*ghapi.Account, error) {
	if f.accountType != "" {
		return &ghapi.Account{Login: owner, Type: f.accountType}, nil
	}
	return &ghapi.Account{Login: owner, Type: "User"}, nil
}

func (f *fakeClient) GetRepository(context.Context, string, string) (*ghapi.Repository, error) {
	return f.repository, f.repositoryErr
}

func (f *fakeClient) CreateOrganizationRepository(context.Context, string, ghapi.CreateRepositoryRequest) (*ghapi.Repository, error) {
	return nil, nil
}

func (f *fakeClient) CreateUserRepository(_ context.Context, request ghapi.CreateRepositoryRequest) (*ghapi.Repository, error) {
	f.createdRequest = request
	f.created = true
	return &ghapi.Repository{Name: request.Name, Visibility: dereferenceString(request.Visibility), Description: request.Description}, nil
}

func (f *fakeClient) UpdateRepository(context.Context, string, string, ghapi.UpdateRepositoryRequest) (*ghapi.Repository, error) {
	return f.repository, nil
}

func (f *fakeClient) ReplaceTopics(context.Context, string, string, []string) error {
	return nil
}

func (f *fakeClient) GetBranchProtection(context.Context, string, string, string) (*ghapi.BranchProtection, error) {
	return nil, &ghapi.APIError{StatusCode: 404}
}

func (f *fakeClient) UpdateBranchProtection(context.Context, string, string, string, map[string]any) error {
	return nil
}

func (f *fakeClient) DeleteBranchProtection(context.Context, string, string, string) error {
	return nil
}

func TestDescribeChangeCreatesRepositoryWhenMissing(t *testing.T) {
	fake := &fakeClient{
		authenticatedUser: &ghapi.Account{Login: "example", Type: "User"},
		repositoryErr:     &ghapi.APIError{StatusCode: 404},
	}
	handler := New(WithClientFactory(func() (client, error) { return fake, nil }))

	change, err := handler.DescribeChange(context.Background(), &schema.Manifest{
		Kind:     schema.KindGitHubRepo,
		Metadata: schema.Metadata{Name: "sample"},
		Spec: &schema.GitHubRepoSpec{
			Owner:      "example",
			Name:       "sample",
			Visibility: "private",
		},
	}, "sample.yaml")
	if err != nil {
		t.Fatalf("DescribeChange() error = %v", err)
	}
	if change.Action != reconcile.ActionCreate {
		t.Fatalf("action = %q, want create", change.Action)
	}
}

func TestApplyCreatesUserRepositoryWithAutoInit(t *testing.T) {
	fake := &fakeClient{
		authenticatedUser: &ghapi.Account{Login: "example", Type: "User"},
		repositoryErr:     &ghapi.APIError{StatusCode: 404},
	}
	handler := New(WithClientFactory(func() (client, error) { return fake, nil }))

	err := handler.Apply(context.Background(), reconcile.ResourceChange{
		Manifest: &schema.Manifest{
			Kind:     schema.KindGitHubRepo,
			Metadata: schema.Metadata{Name: "sample"},
			Spec: &schema.GitHubRepoSpec{
				Owner:       "example",
				Name:        "sample",
				Visibility:  "private",
				Description: "managed repo",
			},
		},
		Action: reconcile.ActionCreate,
	}, reconcile.ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !fake.created {
		t.Fatal("expected repository create")
	}
	if !fake.createdRequest.AutoInit {
		t.Fatal("expected auto_init=true")
	}
}

func TestApplyRejectsUserOwnerMismatch(t *testing.T) {
	fake := &fakeClient{
		authenticatedUser: &ghapi.Account{Login: "example", Type: "User"},
		accountType:       "User",
		repositoryErr:     &ghapi.APIError{StatusCode: 404},
	}
	handler := New(WithClientFactory(func() (client, error) { return fake, nil }))

	err := handler.Apply(context.Background(), reconcile.ResourceChange{
		Manifest: &schema.Manifest{
			Kind:     schema.KindGitHubRepo,
			Metadata: schema.Metadata{Name: "sample"},
			Spec: &schema.GitHubRepoSpec{
				Owner:      "someone-else",
				Name:       "sample",
				Visibility: "private",
			},
		},
		Action: reconcile.ActionCreate,
	}, reconcile.ApplyOptions{})
	if err == nil {
		t.Fatal("expected error when spec.owner is a different user")
	}
	if !strings.Contains(err.Error(), "someone-else") {
		t.Fatalf("error did not mention the mismatched owner: %v", err)
	}
	if fake.created {
		t.Fatal("expected no repository to be created")
	}
}
