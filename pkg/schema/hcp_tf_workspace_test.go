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
  organization: emkaytec
  project: platform
  vcs_repo: github.com/emkaytec/forge
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

	if spec.VCSRepo != "github.com/emkaytec/forge" {
		t.Fatalf("vcs_repo = %q, want github.com/emkaytec/forge", spec.VCSRepo)
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
