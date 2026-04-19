package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/emkaytec/forge/internal/ui"
)

func TestRunWithNoArgsWritesHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run(nil, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "Usage") {
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

	if !strings.Contains(stdout.String(), "Bootstrap") || !strings.Contains(stdout.String(), "help") {
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

func TestRunWithShortVersionFlagWritesVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"-v"}, &stdout, &stderr, "v1.2.3"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if stdout.String() != "v1.2.3\n" {
		t.Fatalf("unexpected version output: %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunWithBogusCommandWritesHelpToStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"bogus"}, &stdout, &stderr, "dev")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}

	if !strings.Contains(err.Error(), `unknown command "bogus"`) {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stderr.String(), "Bootstrap") {
		t.Fatalf("expected help output in stderr, got %q", stderr.String())
	}

	if strings.Contains(stderr.String(), "███████╗") {
		t.Fatalf("expected bogus command help to omit the banner, got %q", stderr.String())
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
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

	if !strings.Contains(stderr.String(), "Bootstrap") {
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

func TestRunWithNoColorProducesNoANSIInHelpOrBanner(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	var helpStdout bytes.Buffer
	var helpStderr bytes.Buffer
	if err := Run([]string{"--help"}, &helpStdout, &helpStderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if strings.Contains(helpStdout.String(), "\x1b[") {
		t.Fatalf("expected help output without ANSI, got %q", helpStdout.String())
	}

	var banner bytes.Buffer
	ui.Banner(&banner, ui.Profile())
	if strings.Contains(banner.String(), "\x1b[") {
		t.Fatalf("expected banner output without ANSI, got %q", banner.String())
	}
}

func TestRunWithHelpListsDemoGroup(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"--help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "Demo") {
		t.Fatalf("expected demo group in help output, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "demo") {
		t.Fatalf("expected demo command in help output, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Bootstrap\n  help") {
		t.Fatalf("expected bootstrap group in help output, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Demo\n  demo") {
		t.Fatalf("expected demo group contents in help output, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "-h, --help      Show help for forge") {
		t.Fatalf("expected polished help flag copy, got %q", stdout.String())
	}
}

func TestRunWithHelpListsManifestGroup(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"--help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "Manifest") {
		t.Fatalf("expected manifest group in help output, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "manifest") {
		t.Fatalf("expected manifest command in help output, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "Manifest\n  manifest") {
		t.Fatalf("expected manifest group contents in help output, got %q", stdout.String())
	}
}

func TestRunWithDemoShowsSubcommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"demo"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "forge demo [command]") {
		t.Fatalf("expected demo usage path, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "banner") || !strings.Contains(stdout.String(), "spinner") {
		t.Fatalf("expected demo subcommands in output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunWithDemoHelpShowsSubcommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"demo", "--help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "forge demo [command]") {
		t.Fatalf("expected demo usage path, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "banner") || !strings.Contains(stdout.String(), "spinner") {
		t.Fatalf("expected demo subcommands in help output, got %q", stdout.String())
	}

	if strings.Contains(stdout.String(), "███████╗") {
		t.Fatalf("expected demo help to omit the banner, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunWithManifestShowsGenerateSubcommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"manifest"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "forge manifest [command]") {
		t.Fatalf("expected manifest usage path, got %q", stdout.String())
	}

	if !strings.Contains(stdout.String(), "generate") {
		t.Fatalf("expected generate subcommand in output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}
