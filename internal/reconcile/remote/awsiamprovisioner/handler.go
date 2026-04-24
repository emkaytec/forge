// Package awsiamprovisioner hosts the remote reconcile handler for
// the AWSIAMProvisioner kind.
package awsiamprovisioner

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/emkaytec/forge/internal/aws/iamcli"
	"github.com/emkaytec/forge/internal/aws/oidc"
	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

type client interface {
	OIDCProviderExists(ctx context.Context, arn string) (bool, error)
	GetRole(ctx context.Context, roleName string) (*iamcli.Role, error)
	CreateRole(ctx context.Context, roleName string, assumeRolePolicy string) error
	UpdateAssumeRolePolicy(ctx context.Context, roleName string, assumeRolePolicy string) error
	ListAttachedRolePolicies(ctx context.Context, roleName string) ([]string, error)
	AttachRolePolicy(ctx context.Context, roleName string, policyARN string) error
	DetachRolePolicy(ctx context.Context, roleName string, policyARN string) error
}

// Handler implements the AWSIAMProvisioner remote handler contract.
type Handler struct {
	newClient func() client
}

type Option func(*Handler)

// New returns a new handler.
func New(opts ...Option) *Handler {
	handler := &Handler{
		newClient: func() client {
			return iamcli.New()
		},
	}

	for _, opt := range opts {
		opt(handler)
	}

	return handler
}

func WithClientFactory(factory func() client) Option {
	return func(handler *Handler) {
		handler.newClient = factory
	}
}

// Kind reports schema.KindAWSIAMProvisioner.
func (h *Handler) Kind() schema.Kind { return schema.KindAWSIAMProvisioner }

func (h *Handler) DescribeChange(ctx context.Context, m *schema.Manifest, _ string) (reconcile.ResourceChange, error) {
	spec, ok := m.Spec.(*schema.AWSIAMProvisionerSpec)
	if !ok {
		return reconcile.ResourceChange{}, fmt.Errorf("AWSIAMProvisioner: unexpected spec type %T", m.Spec)
	}

	client := h.newClient()
	assumeRolePolicy, err := desiredAssumeRolePolicy(spec)
	if err != nil {
		return reconcile.ResourceChange{}, err
	}

	change := reconcile.ResourceChange{Manifest: m}
	missingProviders, err := missingOIDCProviders(ctx, client, spec)
	if err != nil {
		return reconcile.ResourceChange{}, err
	}
	if len(missingProviders) > 0 {
		change.Note = fmt.Sprintf("OIDC provider(s) %s are missing; run `forge init aws-oidc` first", strings.Join(missingProviders, ", "))
	}

	role, err := client.GetRole(ctx, spec.Name)
	switch {
	case iamcli.IsNoSuchEntity(err):
		change.Action = reconcile.ActionCreate
		return change, nil
	case err != nil:
		return reconcile.ResourceChange{}, err
	}

	livePolicy := role.AssumeRolePolicy
	if !equalJSON(livePolicy, assumeRolePolicy) {
		change.Drift = append(change.Drift, describeTrustPolicyDrift(spec, livePolicy)...)
	}

	if spec.ManagedPolicies != nil {
		currentPolicies, err := client.ListAttachedRolePolicies(ctx, spec.Name)
		if err != nil {
			return reconcile.ResourceChange{}, err
		}
		if !equalStringSets(spec.ManagedPolicies, currentPolicies) {
			change.Drift = append(change.Drift, reconcile.DriftField{
				Path:     "spec.managed_policies",
				Desired:  strings.Join(normalizeStrings(spec.ManagedPolicies), ","),
				Observed: strings.Join(normalizeStrings(currentPolicies), ","),
			})
		}
	}

	if len(change.Drift) == 0 {
		change.Action = reconcile.ActionNoOp
		return change, nil
	}

	change.Action = reconcile.ActionUpdate
	return change, nil
}

func (h *Handler) Apply(ctx context.Context, change reconcile.ResourceChange, _ reconcile.ApplyOptions) error {
	spec, ok := change.Manifest.Spec.(*schema.AWSIAMProvisionerSpec)
	if !ok {
		return fmt.Errorf("AWSIAMProvisioner: unexpected spec type %T", change.Manifest.Spec)
	}

	client := h.newClient()
	missingProviders, err := missingOIDCProviders(ctx, client, spec)
	if err != nil {
		return err
	}
	if len(missingProviders) > 0 {
		return fmt.Errorf("OIDC provider(s) %s are missing in account %s; run `forge init aws-oidc` first", strings.Join(missingProviders, ", "), spec.AccountID)
	}

	assumeRolePolicy, err := desiredAssumeRolePolicy(spec)
	if err != nil {
		return err
	}

	_, err = client.GetRole(ctx, spec.Name)
	switch {
	case iamcli.IsNoSuchEntity(err):
		if err := client.CreateRole(ctx, spec.Name, assumeRolePolicy); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		if err := client.UpdateAssumeRolePolicy(ctx, spec.Name, assumeRolePolicy); err != nil {
			return err
		}
	}

	if spec.ManagedPolicies == nil {
		return nil
	}

	currentPolicies, err := client.ListAttachedRolePolicies(ctx, spec.Name)
	if err != nil {
		return err
	}

	desired := normalizeStrings(spec.ManagedPolicies)
	current := normalizeStrings(currentPolicies)

	desiredSet := make(map[string]struct{}, len(desired))
	for _, policy := range desired {
		desiredSet[policy] = struct{}{}
	}
	currentSet := make(map[string]struct{}, len(current))
	for _, policy := range current {
		currentSet[policy] = struct{}{}
	}

	for _, policy := range desired {
		if _, ok := currentSet[policy]; ok {
			continue
		}
		if err := client.AttachRolePolicy(ctx, spec.Name, policy); err != nil {
			return err
		}
	}
	for _, policy := range current {
		if _, ok := desiredSet[policy]; ok {
			continue
		}
		if err := client.DetachRolePolicy(ctx, spec.Name, policy); err != nil {
			return err
		}
	}

	return nil
}

func missingOIDCProviders(ctx context.Context, client client, spec *schema.AWSIAMProvisionerSpec) ([]string, error) {
	seen := make(map[string]struct{}, len(spec.Trusts))
	var missing []string
	for _, trust := range spec.Trusts {
		if _, ok := seen[trust.OIDCProvider]; ok {
			continue
		}
		seen[trust.OIDCProvider] = struct{}{}

		arn := oidcProviderARN(spec.AccountID, trust.OIDCProvider)
		exists, err := client.OIDCProviderExists(ctx, arn)
		if err != nil {
			return nil, err
		}
		if !exists {
			missing = append(missing, trust.OIDCProvider)
		}
	}
	sort.Strings(missing)
	return missing, nil
}

func desiredAssumeRolePolicy(spec *schema.AWSIAMProvisionerSpec) (string, error) {
	statements := make([]map[string]any, 0, len(spec.Trusts))
	for _, trust := range spec.Trusts {
		audience, err := audienceForIssuer(trust.OIDCProvider)
		if err != nil {
			return "", err
		}

		statements = append(statements, map[string]any{
			"Effect": "Allow",
			"Principal": map[string]any{
				"Federated": oidcProviderARN(spec.AccountID, trust.OIDCProvider),
			},
			"Action": "sts:AssumeRoleWithWebIdentity",
			"Condition": map[string]any{
				"StringEquals": map[string]any{
					trust.OIDCProvider + ":aud": audience,
				},
				"StringLike": map[string]any{
					trust.OIDCProvider + ":sub": trust.OIDCSubject,
				},
			},
		})
	}

	document := map[string]any{
		"Version":   "2012-10-17",
		"Statement": statements,
	}

	payload, err := json.Marshal(document)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func audienceForIssuer(issuer string) (string, error) {
	providers := oidc.Providers()
	for _, provider := range providers {
		if provider.Issuer == issuer {
			return provider.Audience, nil
		}
	}
	return "", fmt.Errorf("unsupported OIDC provider %q", issuer)
}

func oidcProviderARN(accountID, issuer string) string {
	return fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s", accountID, issuer)
}

type trustPair struct {
	provider string
	subject  string
}

func describeTrustPolicyDrift(spec *schema.AWSIAMProvisionerSpec, livePolicy string) []reconcile.DriftField {
	desired := make([]trustPair, 0, len(spec.Trusts))
	for _, trust := range spec.Trusts {
		desired = append(desired, trustPair{
			provider: oidcProviderARN(spec.AccountID, trust.OIDCProvider),
			subject:  trust.OIDCSubject,
		})
	}
	observed := liveTrustDetails(livePolicy)

	if equalTrustPairs(desired, observed) {
		return nil
	}

	return []reconcile.DriftField{{
		Path:     "spec.trusts",
		Desired:  formatTrustPairs(desired),
		Observed: formatTrustPairs(observed),
	}}
}

func liveTrustDetails(policy string) []trustPair {
	var document map[string]any
	if err := json.Unmarshal([]byte(policy), &document); err != nil {
		return nil
	}

	statements, _ := document["Statement"].([]any)
	pairs := make([]trustPair, 0, len(statements))
	for _, raw := range statements {
		statement, _ := raw.(map[string]any)
		if statement == nil {
			continue
		}

		principal, _ := statement["Principal"].(map[string]any)
		providerARN := ""
		if principal != nil {
			if federated, ok := principal["Federated"].(string); ok {
				providerARN = federated
			}
		}

		condition, _ := statement["Condition"].(map[string]any)
		subject := ""
		if condition != nil {
			if stringLike, ok := condition["StringLike"].(map[string]any); ok {
				for _, value := range stringLike {
					if parsed, ok := value.(string); ok {
						subject = parsed
						break
					}
				}
			}
		}

		pairs = append(pairs, trustPair{provider: providerARN, subject: subject})
	}
	return pairs
}

func equalTrustPairs(left, right []trustPair) bool {
	if len(left) != len(right) {
		return false
	}
	leftSorted := sortedTrustPairs(left)
	rightSorted := sortedTrustPairs(right)
	for i := range leftSorted {
		if leftSorted[i] != rightSorted[i] {
			return false
		}
	}
	return true
}

func sortedTrustPairs(pairs []trustPair) []trustPair {
	out := make([]trustPair, len(pairs))
	copy(out, pairs)
	sort.Slice(out, func(i, j int) bool {
		if out[i].provider != out[j].provider {
			return out[i].provider < out[j].provider
		}
		return out[i].subject < out[j].subject
	})
	return out
}

func formatTrustPairs(pairs []trustPair) string {
	sorted := sortedTrustPairs(pairs)
	parts := make([]string, 0, len(sorted))
	for _, pair := range sorted {
		parts = append(parts, pair.provider+"|"+pair.subject)
	}
	return strings.Join(parts, ",")
}

func equalJSON(left, right string) bool {
	var leftValue any
	var rightValue any
	if err := json.Unmarshal([]byte(left), &leftValue); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(right), &rightValue); err != nil {
		return false
	}

	leftBytes, _ := json.Marshal(leftValue)
	rightBytes, _ := json.Marshal(rightValue)
	return string(leftBytes) == string(rightBytes)
}

func equalStringSets(left, right []string) bool {
	normalizedLeft := normalizeStrings(left)
	normalizedRight := normalizeStrings(right)
	if len(normalizedLeft) != len(normalizedRight) {
		return false
	}
	for i := range normalizedLeft {
		if normalizedLeft[i] != normalizedRight[i] {
			return false
		}
	}
	return true
}

func normalizeStrings(values []string) []string {
	if values == nil {
		return nil
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		normalized = append(normalized, strings.TrimSpace(value))
	}
	sort.Strings(normalized)
	return normalized
}
