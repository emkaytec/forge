package initcmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/emkaytec/forge/internal/aws/oidc"
)

type fakeOIDCManager struct {
	accountID       string
	resolveErr      error
	results         []providerResult
	ensureErr       error
	receivedAccount string
}

func (f *fakeOIDCManager) ResolveAccountID(context.Context, string) (string, error) {
	return f.accountID, f.resolveErr
}

func (f *fakeOIDCManager) EnsureProviders(_ context.Context, accountID string) ([]providerResult, error) {
	f.receivedAccount = accountID
	return f.results, f.ensureErr
}

func TestAWSOIDCCommandPrintsResolvedAccountAndStatuses(t *testing.T) {
	fake := &fakeOIDCManager{
		accountID: "123456789012",
		results: []providerResult{
			{Provider: mustLookupProvider(t, "github-actions"), Created: true},
			{Provider: mustLookupProvider(t, "hcp-terraform"), Created: false},
		},
	}

	previous := newManager
	newManager = func() oidcManager { return fake }
	defer func() { newManager = previous }()

	cmd := Command()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"aws-oidc"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if fake.receivedAccount != "123456789012" {
		t.Fatalf("received account = %q, want 123456789012", fake.receivedAccount)
	}
	if !strings.Contains(stdout.String(), "Target AWS account: 123456789012") {
		t.Fatalf("expected account banner, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Created GitHub Actions OIDC provider") {
		t.Fatalf("expected created provider output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "HCP Terraform OIDC provider already exists") {
		t.Fatalf("expected existing provider output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestAWSOIDCCommandReturnsEnsureError(t *testing.T) {
	fake := &fakeOIDCManager{
		accountID: "123456789012",
		ensureErr: errors.New("boom"),
	}

	previous := newManager
	newManager = func() oidcManager { return fake }
	defer func() { newManager = previous }()

	cmd := Command()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"aws-oidc"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("expected stderr to include error, got %q", stderr.String())
	}
}

func mustLookupProvider(t *testing.T, key string) oidc.Provider {
	t.Helper()

	provider, ok := oidc.Lookup(key)
	if !ok {
		t.Fatalf("provider %q not found", key)
	}

	return provider
}
