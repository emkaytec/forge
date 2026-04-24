package initcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/emkaytec/forge/internal/aws/iamcli"
)

const (
	defaultStackSetAdministrationRoleName = "AWSCloudFormationStackSetAdministrationRole"
	defaultStackSetExecutionRoleName      = "AWSCloudFormationStackSetExecutionRole"
	defaultStackSetExecutionPolicyARN     = "arn:aws:iam::aws:policy/AdministratorAccess"
	stackSetAdministrationPolicyName      = "AssumeStackSetExecutionRole"
)

var newStackSetManager = func() stackSetManager {
	return newAWSStackSetManager()
}

type stackSetManager interface {
	ResolveManagementAccount(ctx context.Context) (string, error)
	ResolveAccount(ctx context.Context, target awsAccountTarget) (awsAccountTarget, error)
	EnsureAdministrationRole(ctx context.Context, setup stackSetSetup) (stackSetRoleResult, error)
	EnsureExecutionRole(ctx context.Context, setup stackSetSetup, target awsAccountTarget) (stackSetRoleResult, error)
}

type stackSetSetup struct {
	ManagementAccountID      string
	TargetAccountIDs         []string
	AdministrationRoleName   string
	ExecutionRoleName        string
	ExecutionManagedPolicies []string
}

type stackSetRoleResult struct {
	Name                 string
	ARN                  string
	Created              bool
	UpdatedTrustPolicy   bool
	UpdatedInlinePolicy  bool
	AttachedPolicyARNs   []string
	AlreadyAttachedARNs  []string
	TargetAccountDisplay string
}

type awsStackSetManager struct {
	client *iamcli.Client
}

func newAWSStackSetManager() stackSetManager {
	return &awsStackSetManager{client: iamcli.New()}
}

func (m *awsStackSetManager) ResolveManagementAccount(ctx context.Context) (string, error) {
	return m.client.GetCallerIdentity(ctx)
}

func (m *awsStackSetManager) ResolveAccount(ctx context.Context, target awsAccountTarget) (awsAccountTarget, error) {
	return (&awsOIDCManager{client: m.client}).ResolveAccount(ctx, target)
}

func (m *awsStackSetManager) EnsureAdministrationRole(ctx context.Context, setup stackSetSetup) (stackSetRoleResult, error) {
	trustPolicy, err := cloudFormationTrustPolicy()
	if err != nil {
		return stackSetRoleResult{}, err
	}
	inlinePolicy, err := stackSetAdministrationPolicy(setup.TargetAccountIDs, setup.ExecutionRoleName)
	if err != nil {
		return stackSetRoleResult{}, err
	}

	result, err := ensureRole(ctx, m.client, setup.ManagementAccountID, setup.AdministrationRoleName, trustPolicy)
	if err != nil {
		return stackSetRoleResult{}, err
	}

	policyUpdated, err := ensureInlinePolicy(ctx, m.client, setup.AdministrationRoleName, stackSetAdministrationPolicyName, inlinePolicy)
	if err != nil {
		return stackSetRoleResult{}, err
	}
	result.UpdatedInlinePolicy = policyUpdated
	return result, nil
}

func (m *awsStackSetManager) EnsureExecutionRole(ctx context.Context, setup stackSetSetup, target awsAccountTarget) (stackSetRoleResult, error) {
	trustPolicy, err := stackSetExecutionTrustPolicy(stackSetAdministrationRoleARN(setup.ManagementAccountID, setup.AdministrationRoleName))
	if err != nil {
		return stackSetRoleResult{}, err
	}

	client := m.clientForTarget(target)
	result, err := ensureRole(ctx, client, target.AccountID, setup.ExecutionRoleName, trustPolicy)
	if err != nil {
		return stackSetRoleResult{}, err
	}
	result.TargetAccountDisplay = target.display()

	attached, alreadyAttached, err := ensureAttachedPolicies(ctx, client, setup.ExecutionRoleName, setup.ExecutionManagedPolicies)
	if err != nil {
		return stackSetRoleResult{}, err
	}
	result.AttachedPolicyARNs = attached
	result.AlreadyAttachedARNs = alreadyAttached
	return result, nil
}

func (m *awsStackSetManager) clientForTarget(target awsAccountTarget) *iamcli.Client {
	if strings.TrimSpace(target.ProfileName) != "" {
		return m.client.ForProfile(target.ProfileName)
	}
	if strings.TrimSpace(target.AccountID) != "" {
		return m.client.ForAccount(target.AccountID)
	}
	return m.client
}

func ensureRole(ctx context.Context, client *iamcli.Client, accountID, roleName, trustPolicy string) (stackSetRoleResult, error) {
	result := stackSetRoleResult{
		Name: roleName,
		ARN:  roleARN(accountID, roleName),
	}

	role, err := client.GetRole(ctx, roleName)
	switch {
	case iamcli.IsNoSuchEntity(err):
		if err := client.CreateRole(ctx, roleName, trustPolicy); err != nil {
			return stackSetRoleResult{}, err
		}
		result.Created = true
		return result, nil
	case err != nil:
		return stackSetRoleResult{}, err
	}

	result.ARN = role.ARN
	if !equalJSON(role.AssumeRolePolicy, trustPolicy) {
		if err := client.UpdateAssumeRolePolicy(ctx, roleName, trustPolicy); err != nil {
			return stackSetRoleResult{}, err
		}
		result.UpdatedTrustPolicy = true
	}

	return result, nil
}

func ensureInlinePolicy(ctx context.Context, client *iamcli.Client, roleName, policyName, policyDocument string) (bool, error) {
	current, err := client.GetRolePolicy(ctx, roleName, policyName)
	switch {
	case iamcli.IsNoSuchEntity(err):
	case err != nil:
		return false, err
	default:
		if equalJSON(current, policyDocument) {
			return false, nil
		}
	}

	if err := client.PutRolePolicy(ctx, roleName, policyName, policyDocument); err != nil {
		return false, err
	}
	return true, nil
}

func ensureAttachedPolicies(ctx context.Context, client *iamcli.Client, roleName string, policyARNs []string) ([]string, []string, error) {
	current, err := client.ListAttachedRolePolicies(ctx, roleName)
	if err != nil {
		return nil, nil, err
	}
	currentSet := map[string]struct{}{}
	for _, policyARN := range current {
		currentSet[strings.TrimSpace(policyARN)] = struct{}{}
	}

	var attached []string
	var alreadyAttached []string
	for _, policyARN := range normalizeStrings(policyARNs) {
		if _, ok := currentSet[policyARN]; ok {
			alreadyAttached = append(alreadyAttached, policyARN)
			continue
		}
		if err := client.AttachRolePolicy(ctx, roleName, policyARN); err != nil {
			return nil, nil, err
		}
		attached = append(attached, policyARN)
	}
	return attached, alreadyAttached, nil
}

func cloudFormationTrustPolicy() (string, error) {
	return marshalPolicy(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{{
			"Effect": "Allow",
			"Principal": map[string]any{
				"Service": "cloudformation.amazonaws.com",
			},
			"Action": "sts:AssumeRole",
		}},
	})
}

func stackSetExecutionTrustPolicy(administrationRoleARN string) (string, error) {
	return marshalPolicy(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{{
			"Effect": "Allow",
			"Principal": map[string]any{
				"AWS": administrationRoleARN,
			},
			"Action": "sts:AssumeRole",
		}},
	})
}

func stackSetAdministrationPolicy(targetAccountIDs []string, executionRoleName string) (string, error) {
	resources := make([]string, 0, len(targetAccountIDs))
	for _, accountID := range normalizeStrings(targetAccountIDs) {
		resources = append(resources, roleARN(accountID, executionRoleName))
	}
	sort.Strings(resources)

	return marshalPolicy(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{{
			"Effect":   "Allow",
			"Action":   "sts:AssumeRole",
			"Resource": resources,
		}},
	})
}

func marshalPolicy(document map[string]any) (string, error) {
	payload, err := json.Marshal(document)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func stackSetAdministrationRoleARN(accountID, roleName string) string {
	return roleARN(accountID, roleName)
}

func roleARN(accountID, roleName string) string {
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, roleName)
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

func normalizeStrings(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	sort.Strings(normalized)
	return normalized
}
