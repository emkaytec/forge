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

func TestRunWithVersionFlagWritesVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"--version"}, &stdout, &stderr, "v1.2.3"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if stdout.String() != "v1.2.3\n" {
		t.Fatalf("unexpected version output: %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunWithUnknownCommandWritesSuggestionAndHelpToStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"hep"}, &stdout, &stderr, "dev")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}

	if !strings.Contains(err.Error(), `unknown command "hep"`) {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stderr.String(), "Did you mean this?") {
		t.Fatalf("expected suggestion in stderr, got %q", stderr.String())
	}

	if !strings.Contains(stderr.String(), "Available commands:") {
		t.Fatalf("expected help output in stderr, got %q", stderr.String())
	}

	if strings.Contains(stderr.String(), "███████╗") {
		t.Fatalf("expected unknown command help to omit the banner, got %q", stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
}

func TestRunWithDemoBannerWritesBanner(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"demo", "banner"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "███████╗") {
		t.Fatalf("expected banner output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunWithDemoSpinnerWritesSuccessLine(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"demo", "spinner", "--duration", "1ms"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "✓ Spinner demo complete") {
		t.Fatalf("expected success output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}
