package initcmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emkaytec/forge/internal/aws/oidc"
)

type fakeOIDCManager struct {
	resolveErr      error
	results         []providerResult
	ensureErr       error
	receivedTargets []awsAccountTarget
}

func (f *fakeOIDCManager) ResolveAccount(_ context.Context, target awsAccountTarget) (awsAccountTarget, error) {
	return target, f.resolveErr
}

func (f *fakeOIDCManager) EnsureProviders(_ context.Context, target awsAccountTarget) ([]providerResult, error) {
	f.receivedTargets = append(f.receivedTargets, target)
	return f.results, f.ensureErr
}

func TestAWSOIDCCommandPrintsResolvedAccountAndStatuses(t *testing.T) {
	fake := &fakeOIDCManager{
		results: []providerResult{
			{Provider: mustLookupProvider(t, "github-actions"), Created: true},
			{Provider: mustLookupProvider(t, "hcp-terraform"), Created: false},
		},
	}

	previous := newOIDCManager
	newOIDCManager = func() oidcManager { return fake }
	defer func() { newOIDCManager = previous }()

	cmd := Command()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"aws-oidc", "--account-id", "123456789012"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(fake.receivedTargets) != 1 || fake.receivedTargets[0].AccountID != "123456789012" {
		t.Fatalf("received targets = %#v, want account 123456789012", fake.receivedTargets)
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
		ensureErr: errors.New("boom"),
	}

	previous := newOIDCManager
	newOIDCManager = func() oidcManager { return fake }
	defer func() { newOIDCManager = previous }()

	cmd := Command()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"aws-oidc", "--account-id", "123456789012"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("expected stderr to include error, got %q", stderr.String())
	}
}

func TestAWSOIDCCommandPromptsForProfiles(t *testing.T) {
	configureAWSProfiles(t, `
[profile dev-admin]
sso_account_id = 111111111111

[profile prod-admin]
sso_account_id = 222222222222
`)

	fake := &fakeOIDCManager{
		results: []providerResult{
			{Provider: mustLookupProvider(t, "github-actions"), Created: false},
			{Provider: mustLookupProvider(t, "hcp-terraform"), Created: false},
		},
	}

	previous := newOIDCManager
	newOIDCManager = func() oidcManager { return fake }
	defer func() { newOIDCManager = previous }()

	cmd := Command()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetIn(strings.NewReader("1,2\n"))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"aws-oidc"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(fake.receivedTargets) != 2 {
		t.Fatalf("received targets = %#v, want two", fake.receivedTargets)
	}
	if fake.receivedTargets[0].ProfileName != "dev-admin" || fake.receivedTargets[0].AccountID != "111111111111" {
		t.Fatalf("first target = %#v", fake.receivedTargets[0])
	}
	if fake.receivedTargets[1].ProfileName != "prod-admin" || fake.receivedTargets[1].AccountID != "222222222222" {
		t.Fatalf("second target = %#v", fake.receivedTargets[1])
	}
	if !strings.Contains(stdout.String(), "AWS accounts:") {
		t.Fatalf("expected account selector, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

type fakeStackSetManager struct {
	managementAccountID string
	adminResult         stackSetRoleResult
	executionResult     stackSetRoleResult
	receivedSetup       stackSetSetup
	receivedTargets     []awsAccountTarget
}

func (f *fakeStackSetManager) ResolveManagementAccount(context.Context) (string, error) {
	return f.managementAccountID, nil
}

func (f *fakeStackSetManager) ResolveAccount(_ context.Context, target awsAccountTarget) (awsAccountTarget, error) {
	return target, nil
}

func (f *fakeStackSetManager) EnsureAdministrationRole(_ context.Context, setup stackSetSetup) (stackSetRoleResult, error) {
	f.receivedSetup = setup
	return f.adminResult, nil
}

func (f *fakeStackSetManager) EnsureExecutionRole(_ context.Context, setup stackSetSetup, target awsAccountTarget) (stackSetRoleResult, error) {
	f.receivedSetup = setup
	f.receivedTargets = append(f.receivedTargets, target)
	return f.executionResult, nil
}

func TestAWSStackSetsCommandConfiguresAdministrationAndExecutionRoles(t *testing.T) {
	fake := &fakeStackSetManager{
		managementAccountID: "999999999999",
		adminResult: stackSetRoleResult{
			Name:                defaultStackSetAdministrationRoleName,
			UpdatedInlinePolicy: true,
		},
		executionResult: stackSetRoleResult{
			Name:    defaultStackSetExecutionRoleName,
			Created: true,
		},
	}

	previous := newStackSetManager
	newStackSetManager = func() stackSetManager { return fake }
	defer func() { newStackSetManager = previous }()

	cmd := Command()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"aws-stacksets", "--account-id", "111111111111", "--account-id", "222222222222"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if fake.receivedSetup.ManagementAccountID != "999999999999" {
		t.Fatalf("management account = %q", fake.receivedSetup.ManagementAccountID)
	}
	if fake.receivedSetup.AdministrationRoleName != defaultStackSetAdministrationRoleName {
		t.Fatalf("administration role name = %q", fake.receivedSetup.AdministrationRoleName)
	}
	if fake.receivedSetup.ExecutionRoleName != defaultStackSetExecutionRoleName {
		t.Fatalf("execution role name = %q", fake.receivedSetup.ExecutionRoleName)
	}
	if len(fake.receivedSetup.ExecutionManagedPolicies) != 1 || fake.receivedSetup.ExecutionManagedPolicies[0] != defaultStackSetExecutionPolicyARN {
		t.Fatalf("execution managed policies = %#v", fake.receivedSetup.ExecutionManagedPolicies)
	}
	if len(fake.receivedTargets) != 2 {
		t.Fatalf("received execution targets = %#v, want two", fake.receivedTargets)
	}
	for _, want := range []string{
		"Management AWS account: 999999999999",
		"Target AWS accounts:",
		"Updated StackSet administration role",
		"Created StackSet execution role in 111111111111",
		"Created StackSet execution role in 222222222222",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func configureAWSProfiles(t *testing.T, config string) {
	t.Helper()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config")
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	credentialsPath := filepath.Join(tempDir, "credentials")
	if err := os.WriteFile(credentialsPath, nil, 0o644); err != nil {
		t.Fatalf("WriteFile(credentials) error = %v", err)
	}

	t.Setenv("AWS_CONFIG_FILE", configPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentialsPath)
}

func mustLookupProvider(t *testing.T, key string) oidc.Provider {
	t.Helper()

	provider, ok := oidc.Lookup(key)
	if !ok {
		t.Fatalf("provider %q not found", key)
	}

	return provider
}
