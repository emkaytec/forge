package schema_test

import (
	"strings"
	"testing"

	"github.com/emkaytec/forge/pkg/schema"
	"gopkg.in/yaml.v3"
)

func TestAWSIAMProvisionerRoundTrip(t *testing.T) {
	t.Parallel()

	manifest, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: AWSIAMProvisioner
metadata:
  name: github-actions
spec:
  name: github-actions
  account_id: "123456789012"
  oidc_provider: token.actions.githubusercontent.com
  oidc_subject: repo:emkaytec/forge:ref:refs/heads/main
  managed_policies:
    - arn:aws:iam::aws:policy/ReadOnlyAccess
`))
	if err != nil {
		t.Fatalf("DecodeManifest() error = %v", err)
	}

	rendered, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	roundTripped, err := schema.DecodeManifest(rendered)
	if err != nil {
		t.Fatalf("DecodeManifest(roundTrip) error = %v", err)
	}

	spec := roundTripped.Spec.(*schema.AWSIAMProvisionerSpec)
	if spec.OIDCProvider != "token.actions.githubusercontent.com" {
		t.Fatalf("oidc_provider = %q, want token.actions.githubusercontent.com", spec.OIDCProvider)
	}

	if len(spec.ManagedPolicies) != 1 {
		t.Fatalf("managed_policies len = %d, want 1", len(spec.ManagedPolicies))
	}
}

func TestAWSIAMProvisionerRejectsUnsupportedExtraField(t *testing.T) {
	t.Parallel()

	_, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: AWSIAMProvisioner
metadata:
  name: github-actions
spec:
  name: github-actions
  account_id: "123456789012"
  oidc_provider: token.actions.githubusercontent.com
  oidc_subject: repo:emkaytec/forge:ref:refs/heads/main
  assume_role_policy: {}
`))
	if err == nil {
		t.Fatal("DecodeManifest() error = nil, want unsupported extra field")
	}

	if !strings.Contains(err.Error(), "assume_role_policy") {
		t.Fatalf("DecodeManifest() error = %v, want assume_role_policy rejection", err)
	}
}

func TestAWSIAMProvisionerRejectsRoleNamesLongerThanAWSLimit(t *testing.T) {
	t.Parallel()

	_, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: AWSIAMProvisioner
metadata:
  name: github-actions
spec:
  name: this-role-name-is-deliberately-made-long-enough-to-exceed-sixty-four-chars
  account_id: "123456789012"
  oidc_provider: token.actions.githubusercontent.com
  oidc_subject: repo:emkaytec/forge:ref:refs/heads/main
`))
	if err == nil {
		t.Fatal("DecodeManifest() error = nil, want role-name length validation")
	}

	if !strings.Contains(err.Error(), "spec.name") || !strings.Contains(err.Error(), "64 characters") {
		t.Fatalf("DecodeManifest() error = %v, want spec.name max-length validation", err)
	}
}
