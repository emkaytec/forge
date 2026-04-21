// Package githubrepo hosts the remote reconcile handler for the
// GitHubRepository kind.
package githubrepo

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	ghapi "github.com/emkaytec/forge/internal/github"
	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

const ownerEnvVar = "FORGE_GITHUB_OWNER"

type client interface {
	GetAuthenticatedUser(ctx context.Context) (*ghapi.Account, error)
	GetAccount(ctx context.Context, owner string) (*ghapi.Account, error)
	GetRepository(ctx context.Context, owner string, repo string) (*ghapi.Repository, error)
	CreateOrganizationRepository(ctx context.Context, org string, request ghapi.CreateRepositoryRequest) (*ghapi.Repository, error)
	CreateUserRepository(ctx context.Context, request ghapi.CreateRepositoryRequest) (*ghapi.Repository, error)
	UpdateRepository(ctx context.Context, owner string, repo string, request ghapi.UpdateRepositoryRequest) (*ghapi.Repository, error)
	ReplaceTopics(ctx context.Context, owner string, repo string, topics []string) error
	GetBranchProtection(ctx context.Context, owner string, repo string, branch string) (*ghapi.BranchProtection, error)
	UpdateBranchProtection(ctx context.Context, owner string, repo string, branch string, request map[string]any) error
	DeleteBranchProtection(ctx context.Context, owner string, repo string, branch string) error
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

	owner, ownerType, err := resolveOwner(ctx, client)
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

	protected, err := isBranchProtected(ctx, client, owner, spec.Name, spec.DefaultBranch)
	if err != nil {
		return reconcile.ResourceChange{}, err
	}
	if spec.BranchProtection != protected {
		change.Drift = append(change.Drift, reconcile.DriftField{
			Path:     "spec.branch_protection",
			Desired:  boolString(spec.BranchProtection),
			Observed: boolString(protected),
		})
	}

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

	owner, ownerType, err := resolveOwner(ctx, client)
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

	protected, err := isBranchProtected(ctx, client, owner, spec.Name, spec.DefaultBranch)
	if err != nil {
		return err
	}
	if spec.BranchProtection && !protected {
		if err := client.UpdateBranchProtection(ctx, owner, spec.Name, spec.DefaultBranch, baselineBranchProtectionRequest()); err != nil {
			return err
		}
	}
	if !spec.BranchProtection && protected {
		if err := client.DeleteBranchProtection(ctx, owner, spec.Name, spec.DefaultBranch); err != nil && !ghapi.IsNotFound(err) {
			return err
		}
	}

	if created {
		return nil
	}

	return nil
}

func resolveOwner(ctx context.Context, client client) (string, string, error) {
	if configured := strings.TrimSpace(os.Getenv(ownerEnvVar)); configured != "" {
		account, err := client.GetAccount(ctx, configured)
		if err != nil {
			return "", "", err
		}
		return configured, account.Type, nil
	}

	account, err := client.GetAuthenticatedUser(ctx)
	if err != nil {
		return "", "", err
	}
	return account.Login, account.Type, nil
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

func isBranchProtected(ctx context.Context, client client, owner, repo, branch string) (bool, error) {
	_, err := client.GetBranchProtection(ctx, owner, repo, branch)
	switch {
	case err == nil:
		return true, nil
	case ghapi.IsNotFound(err):
		return false, nil
	default:
		return false, err
	}
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

func baselineBranchProtectionRequest() map[string]any {
	return map[string]any{
		"required_status_checks": nil,
		"enforce_admins":         false,
		"required_pull_request_reviews": map[string]any{
			"dismiss_stale_reviews":           true,
			"require_code_owner_reviews":      false,
			"required_approving_review_count": 1,
		},
		"restrictions":                     nil,
		"allow_force_pushes":               false,
		"allow_deletions":                  false,
		"block_creations":                  false,
		"required_conversation_resolution": true,
		"lock_branch":                      false,
		"allow_fork_syncing":               false,
	}
}

func reconcileUnexpectedSpecError(kind string, spec any) error {
	return fmt.Errorf("%s: unexpected spec type %T", kind, spec)
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
