package workstation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeConfiguredWorkstationsOverlaysAndAvoidsUnnamedDuplicates(t *testing.T) {
	t.Parallel()

	discovered := []Workstation{{
		Name:       "forge-dev",
		Provider:   ProviderAWS,
		Status:     StatusRunning,
		InstanceID: "i-123",
	}}
	configured := []configuredWorkstation{{
		Name:              "forge-dev",
		TailscaleHostname: "forge-dev.tailnet.ts.net",
	}}

	merged := mergeConfiguredWorkstations(discovered, configured)
	if len(merged) != 1 {
		t.Fatalf("len(merged) = %d, want 1", len(merged))
	}

	if merged[0].TailscaleHostname != "forge-dev.tailnet.ts.net" {
		t.Fatalf("tailscale hostname = %q, want overlay value", merged[0].TailscaleHostname)
	}
}

func TestResolveWorkstationRejectsAmbiguousName(t *testing.T) {
	t.Parallel()

	_, err := resolveWorkstation([]Workstation{
		{Name: "forge-dev", Provider: ProviderAWS},
		{Name: "forge-dev", Provider: ProviderGCP},
	}, "forge-dev")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}

	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveAnsiblePathsUsesConfiguredRepoAndDefaults(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "inventory"), 0o755); err != nil {
		t.Fatalf("MkdirAll(inventory) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "playbooks"), 0o755); err != nil {
		t.Fatalf("MkdirAll(playbooks) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "inventory", "hosts.yaml"), []byte("all:\n  hosts: {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(hosts.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "playbooks", "workstation.yaml"), []byte("- hosts: all\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(workstation.yaml) error = %v", err)
	}

	t.Setenv(ansibleRepoEnv, repo)

	gotRepo, gotInventory, gotPlaybook, err := resolveAnsiblePaths(config{})
	if err != nil {
		t.Fatalf("resolveAnsiblePaths() error = %v", err)
	}

	if gotRepo != repo {
		t.Fatalf("repo path = %q, want %q", gotRepo, repo)
	}
	if gotInventory != "inventory/hosts.yaml" {
		t.Fatalf("inventory path = %q, want inventory/hosts.yaml", gotInventory)
	}
	if gotPlaybook != "playbooks/workstation.yaml" {
		t.Fatalf("playbook path = %q, want playbooks/workstation.yaml", gotPlaybook)
	}
}
