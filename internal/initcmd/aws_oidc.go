package initcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/emkaytec/forge/internal/aws/iamcli"
	"github.com/emkaytec/forge/internal/aws/oidc"
)

type oidcManager interface {
	ResolveAccountID(ctx context.Context, override string) (string, error)
	EnsureProviders(ctx context.Context, accountID string) ([]providerResult, error)
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

func (m *awsOIDCManager) ResolveAccountID(ctx context.Context, override string) (string, error) {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed, nil
	}

	return m.client.GetCallerIdentity(ctx)
}

func (m *awsOIDCManager) EnsureProviders(ctx context.Context, accountID string) ([]providerResult, error) {
	providers := oidc.Providers()
	results := make([]providerResult, 0, len(providers))

	for _, provider := range providers {
		arn := oidcProviderARN(accountID, provider.Issuer)
		exists, err := m.client.OIDCProviderExists(ctx, arn)
		if err != nil {
			return nil, fmt.Errorf("check %s provider: %w", provider.Label, err)
		}

		if exists {
			results = append(results, providerResult{Provider: provider})
			continue
		}

		if _, err := m.client.CreateOIDCProvider(ctx, "https://"+provider.Issuer, []string{provider.Audience}); err != nil {
			return nil, fmt.Errorf("create %s provider: %w", provider.Label, err)
		}

		results = append(results, providerResult{
			Provider: provider,
			Created:  true,
		})
	}

	return results, nil
}

func oidcProviderARN(accountID, issuer string) string {
	return fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s", accountID, issuer)
}
