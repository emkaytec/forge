package workstation

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

type fakeManager struct {
	listWorkstations []Workstation
	listWarnings     []string
	listErr          error

	startWorkstation Workstation
	startWarnings    []string
	startErr         error
	startName        string

	stopWorkstation Workstation
	stopWarnings    []string
	stopErr         error
	stopName        string

	connectWarnings []string
	connectErr      error
	connectName     string

	reloadWarnings []string
	reloadErr      error
	reloadName     string
}

func (f *fakeManager) List(context.Context) ([]Workstation, []string, error) {
	return f.listWorkstations, f.listWarnings, f.listErr
}

func (f *fakeManager) Start(_ context.Context, name string) (Workstation, []string, error) {
	f.startName = name
	return f.startWorkstation, f.startWarnings, f.startErr
}

func (f *fakeManager) Stop(_ context.Context, name string) (Workstation, []string, error) {
	f.stopName = name
	return f.stopWorkstation, f.stopWarnings, f.stopErr
}

func (f *fakeManager) Connect(_ context.Context, name string, _ io.Reader, _, _ io.Writer) ([]string, error) {
	f.connectName = name
	return f.connectWarnings, f.connectErr
}

func (f *fakeManager) ReloadConfig(_ context.Context, name string, _ io.Reader, _, _ io.Writer) ([]string, error) {
	f.reloadName = name
	return f.reloadWarnings, f.reloadErr
}

func TestListCommandRendersWorkstations(t *testing.T) {
	fake := &fakeManager{
		listWorkstations: []Workstation{{
			Name:              "forge-dev",
			Provider:          ProviderAWS,
			Status:            StatusRunning,
			TailscaleHostname: "forge-dev.tailnet.ts.net",
		}},
		listWarnings: []string{"gcp workstation support requires gcloud on PATH"},
	}

	previous := newManager
	newManager = func() manager { return fake }
	defer func() { newManager = previous }()

	cmd := Command()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "Workstations") || !strings.Contains(stdout.String(), "forge-dev") {
		t.Fatalf("expected workstation table in stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "gcp workstation support requires gcloud on PATH") {
		t.Fatalf("expected warning in stderr, got %q", stderr.String())
	}
}

func TestStartCommandUsesNamedWorkstation(t *testing.T) {
	fake := &fakeManager{
		startWorkstation: Workstation{Name: "forge-dev", Provider: ProviderAWS},
	}

	previous := newManager
	newManager = func() manager { return fake }
	defer func() { newManager = previous }()

	cmd := Command()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"start", "forge-dev"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if fake.startName != "forge-dev" {
		t.Fatalf("start name = %q, want forge-dev", fake.startName)
	}
	if !strings.Contains(stdout.String(), "Started workstation forge-dev") {
		t.Fatalf("expected success output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestReloadConfigCommandAcceptsOptionalName(t *testing.T) {
	fake := &fakeManager{}

	previous := newManager
	newManager = func() manager { return fake }
	defer func() { newManager = previous }()

	cmd := Command()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"reload-config", "forge-dev"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if fake.reloadName != "forge-dev" {
		t.Fatalf("reload name = %q, want forge-dev", fake.reloadName)
	}
}
