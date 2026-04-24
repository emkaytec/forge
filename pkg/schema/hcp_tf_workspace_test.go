package schema_test

import (
	"strings"
	"testing"

	"github.com/emkaytec/forge/pkg/schema"
	"gopkg.in/yaml.v3"
)

func TestHCPTFWorkspaceRoundTripAndValidation(t *testing.T) {
	t.Parallel()

	manifest, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: HCPTerraformWorkspace
metadata:
  name: core-platform
spec:
  name: core-platform
  environment: dev
  organization: emkaytec
  project: platform
  account_id: "123456789012"
  execution_mode: agent
  terraform_version: 1.11.4
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

	spec := roundTripped.Spec.(*schema.HCPTFWorkspaceSpec)
	if spec.ExecutionMode != "agent" {
		t.Fatalf("execution mode = %q, want agent", spec.ExecutionMode)
	}

	if spec.Environment != "dev" {
		t.Fatalf("environment = %q, want dev", spec.Environment)
	}
	if spec.AccountID != "123456789012" {
		t.Fatalf("account_id = %q, want 123456789012", spec.AccountID)
	}
}

func TestHCPTFWorkspaceRejectsInvalidExecutionMode(t *testing.T) {
	t.Parallel()

	_, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: HCPTerraformWorkspace
metadata:
  name: core-platform
spec:
  name: core-platform
  organization: emkaytec
  execution_mode: queued
`))
	if err == nil {
		t.Fatal("DecodeManifest() error = nil, want invalid execution mode")
	}

	if !strings.Contains(err.Error(), "spec.execution_mode") {
		t.Fatalf("DecodeManifest() error = %v, want spec.execution_mode validation", err)
	}
}

func TestHCPTFWorkspaceRejectsInvalidEnvironment(t *testing.T) {
	t.Parallel()

	_, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: HCPTerraformWorkspace
metadata:
  name: core-platform-stage
spec:
  name: core-platform-stage
  environment: stage
  organization: emkaytec
  account_id: "123456789012"
  execution_mode: remote
`))
	if err == nil {
		t.Fatal("DecodeManifest() error = nil, want invalid environment")
	}

	if !strings.Contains(err.Error(), "spec.environment") {
		t.Fatalf("DecodeManifest() error = %v, want spec.environment validation", err)
	}
}

func TestHCPTFWorkspaceAcceptsBlankEnvironment(t *testing.T) {
	t.Parallel()

	manifest, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: HCPTerraformWorkspace
metadata:
  name: admin-core
spec:
  name: admin-core
  organization: emkaytec
  execution_mode: remote
`))
	if err != nil {
		t.Fatalf("DecodeManifest() error = %v, want accepted blank environment", err)
	}

	spec := manifest.Spec.(*schema.HCPTFWorkspaceSpec)
	if spec.Environment != "" {
		t.Fatalf("environment = %q, want empty", spec.Environment)
	}
}
