package workstation

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ansibleRepoEnv      = "FORGE_ANSIBLE_REPO"
	ansibleInventoryEnv = "FORGE_ANSIBLE_INVENTORY"
	ansiblePlaybookEnv  = "FORGE_ANSIBLE_PLAYBOOK"
)

// Manager coordinates workstation discovery and actions across providers.
type Manager struct {
	providers []provider
	loader    configLoader
	runner    commandRunner
}

// Option configures a Manager.
type Option func(*Manager)

// WithProviders overrides the provider set. Tests use this seam.
func WithProviders(providers ...provider) Option {
	return func(m *Manager) {
		m.providers = providers
	}
}

// WithConfigLoader overrides the config loader. Tests use this seam.
func WithConfigLoader(loader configLoader) Option {
	return func(m *Manager) {
		m.loader = loader
	}
}

// WithCommandRunner overrides the shell runner. Tests use this seam.
func WithCommandRunner(runner commandRunner) Option {
	return func(m *Manager) {
		m.runner = runner
	}
}

// NewManager returns a Manager wired to the built-in providers.
func NewManager(opts ...Option) *Manager {
	runner := execRunner{}
	manager := &Manager{
		loader: fileConfigLoader{},
		runner: runner,
	}
	manager.providers = []provider{
		newAWSProvider(runner),
		newGCPProvider(runner),
	}

	for _, opt := range opts {
		opt(manager)
	}

	return manager
}

// List returns the merged workstation inventory plus non-fatal discovery warnings.
func (m *Manager) List(ctx context.Context) ([]Workstation, []string, error) {
	workstations, warnings, _, err := m.inventory(ctx)
	return workstations, warnings, err
}

// Start starts a named workstation.
func (m *Manager) Start(ctx context.Context, name string) (Workstation, []string, error) {
	workstation, warnings, err := m.resolve(ctx, name)
	if err != nil {
		return Workstation{}, warnings, err
	}

	if workstation.isRunning() {
		return workstation, warnings, nil
	}

	provider, err := m.providerFor(workstation.Provider)
	if err != nil {
		return Workstation{}, warnings, err
	}

	if err := provider.Start(ctx, workstation); err != nil {
		return Workstation{}, warnings, err
	}

	return workstation, warnings, nil
}

// Stop stops a named workstation.
func (m *Manager) Stop(ctx context.Context, name string) (Workstation, []string, error) {
	workstation, warnings, err := m.resolve(ctx, name)
	if err != nil {
		return Workstation{}, warnings, err
	}

	if workstation.isStopped() {
		return workstation, warnings, nil
	}

	provider, err := m.providerFor(workstation.Provider)
	if err != nil {
		return Workstation{}, warnings, err
	}

	if err := provider.Stop(ctx, workstation); err != nil {
		return Workstation{}, warnings, err
	}

	return workstation, warnings, nil
}

// Connect opens an SSH session to the workstation's Tailscale hostname.
func (m *Manager) Connect(ctx context.Context, name string, stdin io.Reader, stdout, stderr io.Writer) ([]string, error) {
	workstation, warnings, err := m.resolve(ctx, name)
	if err != nil {
		return warnings, err
	}

	if workstation.isStopped() {
		return warnings, fmt.Errorf("workstation %s is stopped; run `forge workstation start %s` first", workstation.Name, workstation.Name)
	}
	if strings.TrimSpace(workstation.TailscaleHostname) == "" {
		return warnings, fmt.Errorf("workstation %s does not have a Tailscale hostname configured", workstation.Name)
	}

	return warnings, m.runner.Run(ctx, "", "ssh", []string{workstation.TailscaleHostname}, stdin, stdout, stderr)
}

// ReloadConfig re-runs the workstation Ansible playbook for one host or all.
func (m *Manager) ReloadConfig(ctx context.Context, name string, stdin io.Reader, stdout, stderr io.Writer) ([]string, error) {
	workstations, warnings, cfg, err := m.inventory(ctx)
	if err != nil {
		return warnings, err
	}

	if name != "" {
		workstation, resolveErr := resolveWorkstation(workstations, name)
		if resolveErr != nil {
			return warnings, resolveErr
		}
		if workstation.isStopped() {
			return warnings, fmt.Errorf("workstation %s is stopped; run `forge workstation start %s` first", workstation.Name, workstation.Name)
		}
	}

	repoPath, inventoryPath, playbookPath, err := resolveAnsiblePaths(cfg)
	if err != nil {
		return warnings, err
	}

	args := []string{"-i", inventoryPath, playbookPath}
	if name != "" {
		args = append(args, "--limit", name)
	}

	return warnings, m.runner.Run(ctx, repoPath, "ansible-playbook", args, stdin, stdout, stderr)
}

func (m *Manager) inventory(ctx context.Context) ([]Workstation, []string, config, error) {
	cfg, err := m.loader.Load()
	if err != nil {
		return nil, nil, config{}, err
	}

	var (
		discovered []Workstation
		warnings   []string
	)

	for _, provider := range m.providers {
		workstations, err := provider.List(ctx)
		if err != nil {
			if isBinaryUnavailable(err) {
				warnings = append(warnings, err.Error())
				continue
			}

			warnings = append(warnings, fmt.Sprintf("%s workstation discovery failed: %v", provider.Kind(), err))
			continue
		}

		discovered = append(discovered, workstations...)
	}

	merged := mergeConfiguredWorkstations(discovered, cfg.Workstations)
	sortWorkstations(merged)

	return merged, warnings, cfg, nil
}

func (m *Manager) resolve(ctx context.Context, name string) (Workstation, []string, error) {
	workstations, warnings, _, err := m.inventory(ctx)
	if err != nil {
		return Workstation{}, warnings, err
	}

	workstation, err := resolveWorkstation(workstations, name)
	return workstation, warnings, err
}

func resolveWorkstation(workstations []Workstation, name string) (Workstation, error) {
	var matches []Workstation
	for _, workstation := range workstations {
		if workstation.Name == name {
			matches = append(matches, workstation)
		}
	}

	switch len(matches) {
	case 0:
		return Workstation{}, fmt.Errorf("workstation %s not found", name)
	case 1:
		return matches[0], nil
	default:
		providers := make([]string, 0, len(matches))
		for _, workstation := range matches {
			providers = append(providers, workstation.displayProvider())
		}
		sort.Strings(providers)
		return Workstation{}, fmt.Errorf("workstation %s is ambiguous across providers: %s", name, strings.Join(providers, ", "))
	}
}

func (m *Manager) providerFor(kind ProviderKind) (provider, error) {
	for _, provider := range m.providers {
		if provider.Kind() == kind {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("workstation %s does not have a registered provider", kind)
}

func mergeConfiguredWorkstations(discovered []Workstation, configured []configuredWorkstation) []Workstation {
	merged := make([]Workstation, 0, len(discovered)+len(configured))
	seen := make(map[string]struct{}, len(discovered))

	for _, workstation := range discovered {
		overlay := findConfiguredWorkstation(configured, workstation.Name, workstation.Provider)
		if overlay != nil {
			workstation.InstanceID = firstNonEmpty(overlay.InstanceID, workstation.InstanceID)
			workstation.Zone = firstNonEmpty(overlay.Zone, workstation.Zone)
			workstation.TailscaleHostname = firstNonEmpty(overlay.TailscaleHostname, workstation.TailscaleHostname)
		}

		merged = append(merged, workstation)
		seen[mergeKey(workstation.Name, workstation.Provider)] = struct{}{}
	}

	for _, workstation := range configured {
		if workstation.Provider == "" {
			found := false
			for _, item := range merged {
				if item.Name == workstation.Name {
					found = true
					break
				}
			}
			if found {
				continue
			}
		}

		key := mergeKey(workstation.Name, workstation.Provider)
		if _, ok := seen[key]; ok {
			continue
		}

		merged = append(merged, Workstation{
			Name:              workstation.Name,
			Provider:          workstation.Provider,
			Status:            StatusUnknown,
			TailscaleHostname: workstation.TailscaleHostname,
			InstanceID:        workstation.InstanceID,
			Zone:              workstation.Zone,
		})
	}

	return merged
}

func findConfiguredWorkstation(configured []configuredWorkstation, name string, provider ProviderKind) *configuredWorkstation {
	for i := range configured {
		if configured[i].Name != name {
			continue
		}
		if configured[i].Provider != "" && configured[i].Provider != provider {
			continue
		}

		return &configured[i]
	}

	return nil
}

func mergeKey(name string, provider ProviderKind) string {
	return string(provider) + ":" + name
}

func resolveAnsiblePaths(cfg config) (string, string, string, error) {
	repoPath := strings.TrimSpace(os.Getenv(ansibleRepoEnv))
	if repoPath == "" {
		repoPath = cfg.Ansible.RepoPath
	}
	if repoPath == "" {
		return "", "", "", fmt.Errorf("ansible repo path is not configured; set %s or ansible.repo_path in ~/.config/forge/config.yaml", ansibleRepoEnv)
	}

	expandedRepo, err := expandHome(repoPath)
	if err != nil {
		return "", "", "", err
	}

	inventory := strings.TrimSpace(os.Getenv(ansibleInventoryEnv))
	if inventory == "" {
		inventory = cfg.Ansible.Inventory
	}
	inventoryPath, err := resolveRepoRelativePath(expandedRepo, inventory, "inventory/hosts.yml", "inventory/hosts.yaml")
	if err != nil {
		return "", "", "", fmt.Errorf("resolve ansible inventory: %w", err)
	}

	playbook := strings.TrimSpace(os.Getenv(ansiblePlaybookEnv))
	if playbook == "" {
		playbook = cfg.Ansible.Playbook
	}
	playbookPath, err := resolveRepoRelativePath(expandedRepo, playbook, "playbooks/workstation.yml", "playbooks/workstation.yaml")
	if err != nil {
		return "", "", "", fmt.Errorf("resolve ansible playbook: %w", err)
	}

	return expandedRepo, inventoryPath, playbookPath, nil
}

func resolveRepoRelativePath(repoPath, configured string, defaults ...string) (string, error) {
	if configured != "" {
		if filepath.IsAbs(configured) {
			return configured, nil
		}
		return configured, nil
	}

	for _, candidate := range defaults {
		path := filepath.Join(repoPath, candidate)
		if _, err := os.Stat(path); err == nil {
			return candidate, nil
		}
	}

	if len(defaults) == 0 {
		return "", fmt.Errorf("no candidate paths configured")
	}

	return "", fmt.Errorf("none of the default paths exist: %s", strings.Join(defaults, ", "))
}
