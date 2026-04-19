package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunManifestComposeReturnsPlaceholderError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"manifest", "compose", "terraform-github-repo", "forge"}, &stdout, &stderr, "dev")
	if err == nil {
		t.Fatal("expected placeholder error")
	}

	if !strings.Contains(err.Error(), "blueprint composition is not implemented yet: terraform-github-repo") {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunManifestComposeHelpDescribesBlueprints(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"manifest", "compose", "--help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "forge manifest compose [blueprint] [name]") {
		t.Fatalf("expected compose usage path, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Compose a blueprint into several primitive manifests") {
		t.Fatalf("expected compose help text, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "terraform-github-repo") {
		t.Fatalf("expected compose example, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}
