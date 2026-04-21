package schema_test

import (
	"errors"
	"testing"

	"github.com/emkaytec/forge/pkg/schema"
)

func TestDecodeManifestSupportedKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		data   string
		assert func(t *testing.T, manifest *schema.Manifest)
	}{
		{
			name: "github repo applies default branch",
			data: `
apiVersion: forge/v1
kind: GitHubRepository
metadata:
  name: sample-repo
spec:
  owner: emkaytec
  name: sample-repo
  visibility: public
`,
			assert: func(t *testing.T, manifest *schema.Manifest) {
				t.Helper()

				if manifest.Metadata.Name != "sample-repo" {
					t.Fatalf("metadata.name = %q, want sample-repo", manifest.Metadata.Name)
				}

				spec, ok := manifest.Spec.(*schema.GitHubRepoSpec)
				if !ok {
					t.Fatalf("spec type = %T, want *schema.GitHubRepoSpec", manifest.Spec)
				}

				if spec.DefaultBranch != "main" {
					t.Fatalf("default branch = %q, want main", spec.DefaultBranch)
				}
			},
		},
		{
			name: "hcp workspace decodes typed spec",
			data: `
apiVersion: forge/v1
kind: HCPTerraformWorkspace
metadata:
  name: shared-workspace
spec:
  name: shared-workspace
  organization: emkaytec
  execution_mode: remote
`,
			assert: func(t *testing.T, manifest *schema.Manifest) {
				t.Helper()

				if _, ok := manifest.Spec.(*schema.HCPTFWorkspaceSpec); !ok {
					t.Fatalf("spec type = %T, want *schema.HCPTFWorkspaceSpec", manifest.Spec)
				}
			},
		},
		{
			name: "aws provisioner decodes typed spec",
			data: `
apiVersion: forge/v1
kind: AWSIAMProvisioner
metadata:
  name: github-actions
spec:
  name: github-actions
  account_id: "123456789012"
  oidc_provider: token.actions.githubusercontent.com
  oidc_subject: repo:emkaytec/forge:ref:refs/heads/main
`,
			assert: func(t *testing.T, manifest *schema.Manifest) {
				t.Helper()

				if _, ok := manifest.Spec.(*schema.AWSIAMProvisionerSpec); !ok {
					t.Fatalf("spec type = %T, want *schema.AWSIAMProvisionerSpec", manifest.Spec)
				}
			},
		},
		{
			name: "launch agent applies run-at-load default",
			data: `
apiVersion: forge/v1
kind: LaunchAgent
metadata:
  name: workstation-sync
spec:
  name: workstation-sync
  label: dev.emkaytec.workstation-sync
  command: /usr/local/bin/forge workstation sync
  schedule:
    type: interval
    interval_seconds: 900
`,
			assert: func(t *testing.T, manifest *schema.Manifest) {
				t.Helper()

				spec, ok := manifest.Spec.(*schema.LaunchAgentSpec)
				if !ok {
					t.Fatalf("spec type = %T, want *schema.LaunchAgentSpec", manifest.Spec)
				}

				if spec.RunAtLoad {
					t.Fatalf("run_at_load = %t, want false", spec.RunAtLoad)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			manifest, err := schema.DecodeManifest([]byte(tt.data))
			if err != nil {
				t.Fatalf("DecodeManifest() error = %v", err)
			}

			tt.assert(t, manifest)
		})
	}
}

func TestDecodeManifestUnsupportedVersion(t *testing.T) {
	t.Parallel()

	_, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v2
kind: GitHubRepository
metadata:
  name: sample-repo
spec:
  name: sample-repo
  visibility: public
`))
	if err == nil {
		t.Fatal("DecodeManifest() error = nil, want unsupported version")
	}

	var unsupported *schema.UnsupportedVersionError
	if !errors.As(err, &unsupported) {
		t.Fatalf("DecodeManifest() error = %v, want UnsupportedVersionError", err)
	}
}

func TestDecodeManifestUnsupportedKind(t *testing.T) {
	t.Parallel()

	_, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: mystery
metadata:
  name: sample
spec:
  name: sample
`))
	if err == nil {
		t.Fatal("DecodeManifest() error = nil, want unsupported kind")
	}

	var unsupported *schema.UnsupportedKindError
	if !errors.As(err, &unsupported) {
		t.Fatalf("DecodeManifest() error = %v, want UnsupportedKindError", err)
	}
}
