package schema_test

import (
	"strings"
	"testing"

	"github.com/emkaytec/forge/pkg/schema"
	"gopkg.in/yaml.v3"
)

func TestGitHubRepoRoundTripAndValidation(t *testing.T) {
	t.Parallel()

	manifest, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: GitHubRepository
metadata:
  name: portfolio-repo
spec:
  owner: emkaytec
  name: portfolio-repo
  visibility: private
  description: Portfolio-safe example repository
  topics:
    - go
    - automation
  default_branch: main
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

	spec := roundTripped.Spec.(*schema.GitHubRepoSpec)
	if spec.Owner != "emkaytec" {
		t.Fatalf("owner = %q, want emkaytec", spec.Owner)
	}
	if spec.Visibility != "private" {
		t.Fatalf("visibility = %q, want private", spec.Visibility)
	}

	if spec.DefaultBranch != "main" {
		t.Fatalf("default branch = %q, want main", spec.DefaultBranch)
	}

	if len(spec.Topics) != 2 || spec.Topics[0] != "go" || spec.Topics[1] != "automation" {
		t.Fatalf("topics = %#v, want [go automation]", spec.Topics)
	}
}

func TestGitHubRepoRejectsInvalidVisibility(t *testing.T) {
	t.Parallel()

	_, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: GitHubRepository
metadata:
  name: portfolio-repo
spec:
  owner: emkaytec
  name: portfolio-repo
  visibility: internal
`))
	if err == nil {
		t.Fatal("DecodeManifest() error = nil, want invalid visibility")
	}

	if !strings.Contains(err.Error(), "spec.visibility") {
		t.Fatalf("DecodeManifest() error = %v, want spec.visibility validation", err)
	}
}

func TestGitHubRepoRejectsMissingOwner(t *testing.T) {
	t.Parallel()

	_, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: GitHubRepository
metadata:
  name: portfolio-repo
spec:
  name: portfolio-repo
  visibility: private
`))
	if err == nil {
		t.Fatal("DecodeManifest() error = nil, want missing-owner error")
	}

	if !strings.Contains(err.Error(), "spec.owner") {
		t.Fatalf("DecodeManifest() error = %v, want spec.owner validation", err)
	}
}
