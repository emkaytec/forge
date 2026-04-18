package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunWithNoArgsWritesHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run(nil, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout did not contain usage text: %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "███████╗") {
		t.Fatalf("stdout did not contain the banner: %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunWithExplicitHelpOmitsBanner(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"--help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if strings.Contains(stdout.String(), "███████╗") {
		t.Fatalf("expected explicit help to omit the banner, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Available commands:") {
		t.Fatalf("stdout did not contain help text: %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}
