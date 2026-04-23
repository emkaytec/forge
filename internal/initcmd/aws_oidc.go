package initcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/emkaytec/forge/internal/aws/iamcli"
	"github.com/emkaytec/forge/internal/aws/oidc"
)

type oidcManager interface {
	ResolveAccount(ctx context.Context, target awsAccountTarget) (awsAccountTarget, error)
	EnsureProviders(ctx context.Context, target awsAccountTarget) ([]providerResult, error)
}

type providerResult struct {
	Provider oidc.Provider
	Created  bool
}

type awsOIDCManager struct {
	client *iamcli.Client
}

func newAWSOIDCManager() oidcManager {
	return &awsOIDCManager{client: iamcli.New()}
}

func (m *awsOIDCManager) ResolveAccount(ctx context.Context, target awsAccountTarget) (awsAccountTarget, error) {
	if strings.TrimSpace(target.AccountID) != "" {
		target.AccountID = strings.TrimSpace(target.AccountID)
		return target, nil
	}

	accountID, err := m.clientForTarget(target).GetCallerIdentity(ctx)
	if err != nil {
		return awsAccountTarget{}, err
	}
	target.AccountID = accountID
	return target, nil
}

func (m *awsOIDCManager) EnsureProviders(ctx context.Context, target awsAccountTarget) ([]providerResult, error) {
	if strings.TrimSpace(target.AccountID) == "" {
		resolved, err := m.ResolveAccount(ctx, target)
		if err != nil {
			return nil, err
		}
		target = resolved
	}

	client := m.clientForTarget(target)
	providers := oidc.Providers()
	results := make([]providerResult, 0, len(providers))

	for _, provider := range providers {
		arn := oidcProviderARN(target.AccountID, provider.Issuer)
		exists, err := client.OIDCProviderExists(ctx, arn)
		if err != nil {
			return nil, fmt.Errorf("check %s provider: %w", provider.Label, err)
		}

		if exists {
			results = append(results, providerResult{Provider: provider})
			continue
		}

		if _, err := client.CreateOIDCProvider(ctx, "https://"+provider.Issuer, []string{provider.Audience}); err != nil {
			return nil, fmt.Errorf("create %s provider: %w", provider.Label, err)
		}

		results = append(results, providerResult{
			Provider: provider,
			Created:  true,
		})
	}

	return results, nil
}

func (m *awsOIDCManager) clientForTarget(target awsAccountTarget) *iamcli.Client {
	if strings.TrimSpace(target.ProfileName) != "" {
		return m.client.ForProfile(target.ProfileName)
	}
	if strings.TrimSpace(target.AccountID) != "" {
		return m.client.ForAccount(target.AccountID)
	}
	return m.client
}

func oidcProviderARN(accountID, issuer string) string {
	return fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s", accountID, issuer)
}
