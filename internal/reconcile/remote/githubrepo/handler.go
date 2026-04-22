// Package githubrepo hosts the remote reconcile handler for the
// GitHubRepository kind.
package githubrepo

import (
	"context"
	"fmt"
	"sort"
	"strings"

	ghapi "github.com/emkaytec/forge/internal/github"
	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

type client interface {
	GetAuthenticatedUser(ctx context.Context) (*ghapi.Account, error)
	GetAccount(ctx context.Context, owner string) (*ghapi.Account, error)
	GetRepository(ctx context.Context, owner string, repo string) (*ghapi.Repository, error)
	CreateOrganizationRepository(ctx context.Context, org string, request ghapi.CreateRepositoryRequest) (*ghapi.Repository, error)
	CreateUserRepository(ctx context.Context, request ghapi.CreateRepositoryRequest) (*ghapi.Repository, error)
	UpdateRepository(ctx context.Context, owner string, repo string, request ghapi.UpdateRepositoryRequest) (*ghapi.Repository, error)
	ReplaceTopics(ctx context.Context, owner string, repo string, topics []string) error
}

// Handler implements the GitHubRepository remote handler contract.
type Handler struct {
	newClient func() (client, error)
}

type Option func(*Handler)

// New returns a new handler.
func New(opts ...Option) *Handler {
	handler := &Handler{
		newClient: func() (client, error) {
			return ghapi.NewClientFromEnv()
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

// Kind reports schema.KindGitHubRepo.
func (h *Handler) Kind() schema.Kind { return schema.KindGitHubRepo }

func (h *Handler) DescribeChange(ctx context.Context, m *schema.Manifest, _ string) (reconcile.ResourceChange, error) {
	spec, ok := m.Spec.(*schema.GitHubRepoSpec)
	if !ok {
		return reconcile.ResourceChange{}, reconcileUnexpectedSpecError("GitHubRepository", m.Spec)
	}

	client, err := h.newClient()
	if err != nil {
		return reconcile.ResourceChange{}, err
	}

	owner, ownerType, err := resolveOwner(ctx, client, spec)
	if err != nil {
		return reconcile.ResourceChange{}, err
	}

	change := reconcile.ResourceChange{
		Manifest: m,
		Note:     "owner " + owner,
	}

	repository, err := client.GetRepository(ctx, owner, spec.Name)
	switch {
	case ghapi.IsNotFound(err):
		change.Action = reconcile.ActionCreate
		change.Note = strings.TrimSpace(change.Note + "; auto-inits the repository so the managed default branch exists")
		return change, nil
	case err != nil:
		return reconcile.ResourceChange{}, err
	}

	change.Drift = append(change.Drift, describeRepositorySettingsDrift(spec, repository)...)
	change.Drift = append(change.Drift, describeTopicsDrift(spec, repository)...)

	if len(change.Drift) == 0 {
		change.Action = reconcile.ActionNoOp
		return change, nil
	}

	change.Action = reconcile.ActionUpdate
	if ownerType == "Organization" {
		change.Note = strings.TrimSpace(change.Note + "; organization repository")
	}
	return change, nil
}

func (h *Handler) Apply(ctx context.Context, change reconcile.ResourceChange, _ reconcile.ApplyOptions) error {
	spec, ok := change.Manifest.Spec.(*schema.GitHubRepoSpec)
	if !ok {
		return reconcileUnexpectedSpecError("GitHubRepository", change.Manifest.Spec)
	}

	client, err := h.newClient()
	if err != nil {
		return err
	}

	owner, ownerType, err := resolveOwner(ctx, client, spec)
	if err != nil {
		return err
	}

	repository, err := client.GetRepository(ctx, owner, spec.Name)
	created := false
	switch {
	case ghapi.IsNotFound(err):
		visibility := spec.Visibility
		description := spec.Description
		createRequest := ghapi.CreateRepositoryRequest{
			Name:        spec.Name,
			Visibility:  &visibility,
			Description: &description,
			AutoInit:    true,
		}

		if ownerType == "Organization" {
			repository, err = client.CreateOrganizationRepository(ctx, owner, createRequest)
		} else {
			repository, err = client.CreateUserRepository(ctx, createRequest)
		}
		if err != nil {
			return err
		}
		created = true
	case err != nil:
		return err
	}

	updateRequest := ghapi.UpdateRepositoryRequest{}
	if repository.Visibility != spec.Visibility {
		updateRequest.Visibility = stringPtr(spec.Visibility)
	}
	if dereferenceString(repository.Description) != spec.Description {
		updateRequest.Description = stringPtr(spec.Description)
	}
	if repository.DefaultBranch != spec.DefaultBranch {
		updateRequest.DefaultBranch = stringPtr(spec.DefaultBranch)
	}

	if !updateRequest.IsZero() {
		repository, err = client.UpdateRepository(ctx, owner, spec.Name, updateRequest)
		if err != nil {
			return err
		}
	}

	if spec.Topics != nil && !equalTopics(spec.Topics, repository.Topics) {
		if err := client.ReplaceTopics(ctx, owner, spec.Name, normalizeTopics(spec.Topics)); err != nil {
			return err
		}
	}

	if created {
		return nil
	}

	return nil
}

func resolveOwner(ctx context.Context, client client, spec *schema.GitHubRepoSpec) (string, string, error) {
	owner := strings.TrimSpace(spec.Owner)
	if owner == "" {
		return "", "", fmt.Errorf("spec.owner must not be empty")
	}

	account, err := client.GetAccount(ctx, owner)
	if err != nil {
		return "", "", err
	}

	// User-typed owners route through POST /user/repos, which ignores the
	// requested owner and creates under the authenticated user. Fail loudly
	// if those two don't match instead of silently writing to the wrong
	// account.
	if account.Type == "User" {
		authenticated, err := client.GetAuthenticatedUser(ctx)
		if err != nil {
			return "", "", err
		}
		if !strings.EqualFold(authenticated.Login, owner) {
			return "", "", fmt.Errorf("spec.owner %q is a user account; only %q can create repositories there", owner, authenticated.Login)
		}
	}

	return owner, account.Type, nil
}

func describeRepositorySettingsDrift(spec *schema.GitHubRepoSpec, repository *ghapi.Repository) []reconcile.DriftField {
	var drift []reconcile.DriftField

	if repository.Visibility != spec.Visibility {
		drift = append(drift, reconcile.DriftField{
			Path:     "spec.visibility",
			Desired:  spec.Visibility,
			Observed: repository.Visibility,
		})
	}
	if dereferenceString(repository.Description) != spec.Description {
		drift = append(drift, reconcile.DriftField{
			Path:     "spec.description",
			Desired:  spec.Description,
			Observed: dereferenceString(repository.Description),
		})
	}
	if repository.DefaultBranch != spec.DefaultBranch {
		drift = append(drift, reconcile.DriftField{
			Path:     "spec.default_branch",
			Desired:  spec.DefaultBranch,
			Observed: repository.DefaultBranch,
		})
	}

	return drift
}

func describeTopicsDrift(spec *schema.GitHubRepoSpec, repository *ghapi.Repository) []reconcile.DriftField {
	if spec.Topics == nil || equalTopics(spec.Topics, repository.Topics) {
		return nil
	}

	return []reconcile.DriftField{{
		Path:     "spec.topics",
		Desired:  strings.Join(normalizeTopics(spec.Topics), ","),
		Observed: strings.Join(normalizeTopics(repository.Topics), ","),
	}}
}

func equalTopics(desired, current []string) bool {
	normalizedDesired := normalizeTopics(desired)
	normalizedCurrent := normalizeTopics(current)
	if len(normalizedDesired) != len(normalizedCurrent) {
		return false
	}

	for i := range normalizedDesired {
		if normalizedDesired[i] != normalizedCurrent[i] {
			return false
		}
	}

	return true
}

func normalizeTopics(values []string) []string {
	if values == nil {
		return nil
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(value)))
	}
	sort.Strings(normalized)
	return normalized
}

func dereferenceString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringPtr(value string) *string {
	return &value
}

func reconcileUnexpectedSpecError(kind string, spec any) error {
	return fmt.Errorf("%s: unexpected spec type %T", kind, spec)
}
