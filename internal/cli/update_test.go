package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	selfupdate "github.com/emkaytec/forge/internal/update"
)

type fakeUpdateRunner struct {
	result selfupdate.Result
	err    error
}

func (f fakeUpdateRunner) Run(_ context.Context, _ selfupdate.Options) (selfupdate.Result, error) {
	return f.result, f.err
}

func TestRunWithHelpListsUpdateCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run([]string{"--help"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "update") {
		t.Fatalf("expected update command in help output, got %q", stdout.String())
	}
}

func TestRunWithUpdateCheckWritesAvailableMessage(t *testing.T) {
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

	if err := Run([]string{"update", "--check"}, &stdout, &stderr, "v1.2.3"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "Update available: v1.2.3 -> v1.2.4") {
		t.Fatalf("expected update available output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunWithUpdateErrorWritesErrorMessage(t *testing.T) {
	previous := newUpdateRunner
	newUpdateRunner = func(string) updateRunner {
		return fakeUpdateRunner{err: errors.New("release v9.9.9 not found")}
	}
	defer func() {
		newUpdateRunner = previous
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run([]string{"update", "--version", "v9.9.9"}, &stdout, &stderr, "v1.2.3")
	if err == nil {
		t.Fatal("expected update error")
	}

	if !strings.Contains(stderr.String(), "release v9.9.9 not found") {
		t.Fatalf("expected operator-facing error output, got %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
}
