package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/emkaytec/forge/internal/ui"
	selfupdate "github.com/emkaytec/forge/internal/update"
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

func TestRunWithNoArgsShowsAvailableUpdateOnTitleScreen(t *testing.T) {
	previous := newUpdateRunner
	newUpdateRunner = func(version string) updateRunner {
		return fakeUpdateRunner{
			result: selfupdate.Result{
				CurrentVersion: version,
				TargetVersion:  "v1.2.4",
			},
		}
	}
	defer func() {
		newUpdateRunner = previous
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run(nil, &stdout, &stderr, "v1.2.3"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "Update available: v1.2.3 -> v1.2.4. Run `forge update` to install it.") {
		t.Fatalf("expected title-screen update notice, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunWithNoArgsSkipsUpdateNoticeWhenUpToDate(t *testing.T) {
	previous := newUpdateRunner
	newUpdateRunner = func(version string) updateRunner {
		return fakeUpdateRunner{
			result: selfupdate.Result{
				CurrentVersion: version,
				TargetVersion:  version,
				UpToDate:       true,
			},
		}
	}
	defer func() {
		newUpdateRunner = previous
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run(nil, &stdout, &stderr, "v1.2.3"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if strings.Contains(stdout.String(), "Update available:") {
		t.Fatalf("did not expect title-screen update notice, got %q", stdout.String())
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

func TestRunWithExplicitHelpDoesNotCheckForUpdates(t *testing.T) {
	previous := newUpdateRunner
	called := false
	newUpdateRunner = func(string) updateRunner {
		called = true
		return fakeUpdateRunner{}
	}
	defer func() {
		newUpdateRunner = previous
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"--help"}, &stdout, &stderr, "v1.2.3"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if called {
		t.Fatal("expected explicit help to skip update checks")
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

func TestRunWithHelpListsBootstrapGroup(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"--help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "Bootstrap\n  help") {
		t.Fatalf("expected bootstrap group in help output, got %q", stdout.String())
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

func TestRunWithHelpListsReconcileGroup(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"--help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "Reconcile\n  reconcile") {
		t.Fatalf("expected reconcile group contents in help output, got %q", stdout.String())
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
