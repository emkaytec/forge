package initcmd

import (
	"encoding/json"
	"testing"
)

func TestStackSetAdministrationPolicyScopesExecutionRoleResources(t *testing.T) {
	t.Parallel()

	policy, err := stackSetAdministrationPolicy(
		[]string{"222222222222", "111111111111", "111111111111"},
		defaultStackSetExecutionRoleName,
	)
	if err != nil {
		t.Fatalf("stackSetAdministrationPolicy() error = %v", err)
	}

	var document struct {
		Statement []struct {
			Action   string   `json:"Action"`
			Resource []string `json:"Resource"`
		} `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(policy), &document); err != nil {
		t.Fatalf("Unmarshal(policy) error = %v", err)
	}

	if got := document.Statement[0].Action; got != "sts:AssumeRole" {
		t.Fatalf("Action = %q", got)
	}
	want := []string{
		"arn:aws:iam::111111111111:role/AWSCloudFormationStackSetExecutionRole",
		"arn:aws:iam::222222222222:role/AWSCloudFormationStackSetExecutionRole",
	}
	if len(document.Statement[0].Resource) != len(want) {
		t.Fatalf("Resource = %#v, want %#v", document.Statement[0].Resource, want)
	}
	for i := range want {
		if document.Statement[0].Resource[i] != want[i] {
			t.Fatalf("Resource[%d] = %q, want %q", i, document.Statement[0].Resource[i], want[i])
		}
	}
}

func TestStackSetExecutionTrustPolicyTrustsAdministrationRole(t *testing.T) {
	t.Parallel()

	trustPolicy, err := stackSetExecutionTrustPolicy("arn:aws:iam::999999999999:role/AWSCloudFormationStackSetAdministrationRole")
	if err != nil {
		t.Fatalf("stackSetExecutionTrustPolicy() error = %v", err)
	}

	var document struct {
		Statement []struct {
			Principal map[string]string `json:"Principal"`
			Action    string            `json:"Action"`
		} `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(trustPolicy), &document); err != nil {
		t.Fatalf("Unmarshal(trustPolicy) error = %v", err)
	}

	if got := document.Statement[0].Principal["AWS"]; got != "arn:aws:iam::999999999999:role/AWSCloudFormationStackSetAdministrationRole" {
		t.Fatalf("Principal AWS = %q", got)
	}
	if got := document.Statement[0].Action; got != "sts:AssumeRole" {
		t.Fatalf("Action = %q", got)
	}
}
