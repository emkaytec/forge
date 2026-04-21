package workstation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultConfigEnvVar = "FORGE_CONFIG"

type config struct {
	Workstations []configuredWorkstation `yaml:"workstations"`
	Ansible      ansibleConfig           `yaml:"ansible"`
}

type configuredWorkstation struct {
	Name              string       `yaml:"name"`
	Provider          ProviderKind `yaml:"provider,omitempty"`
	InstanceID        string       `yaml:"instance_id,omitempty"`
	Zone              string       `yaml:"zone,omitempty"`
	TailscaleHostname string       `yaml:"tailscale_hostname,omitempty"`
}

type ansibleConfig struct {
	RepoPath  string `yaml:"repo_path,omitempty"`
	Inventory string `yaml:"inventory,omitempty"`
	Playbook  string `yaml:"playbook,omitempty"`
}

type configLoader interface {
	Load() (config, error)
}

type fileConfigLoader struct{}

func (fileConfigLoader) Load() (config, error) {
	path, err := resolveConfigPath()
	if err != nil {
		return config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config{}, nil
		}
		return config{}, fmt.Errorf("read forge config %s: %w", path, err)
	}

	var cfg config
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return config{}, fmt.Errorf("decode forge config %s: %w", path, err)
	}

	for i := range cfg.Workstations {
		cfg.Workstations[i].Name = strings.TrimSpace(cfg.Workstations[i].Name)
		cfg.Workstations[i].InstanceID = strings.TrimSpace(cfg.Workstations[i].InstanceID)
		cfg.Workstations[i].Zone = strings.TrimSpace(cfg.Workstations[i].Zone)
		cfg.Workstations[i].TailscaleHostname = strings.TrimSpace(cfg.Workstations[i].TailscaleHostname)
	}
	cfg.Ansible.RepoPath = strings.TrimSpace(cfg.Ansible.RepoPath)
	cfg.Ansible.Inventory = strings.TrimSpace(cfg.Ansible.Inventory)
	cfg.Ansible.Playbook = strings.TrimSpace(cfg.Ansible.Playbook)

	return cfg, nil
}

func resolveConfigPath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(defaultConfigEnvVar)); configured != "" {
		return expandHome(configured)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}

	return filepath.Join(home, ".config", "forge", "config.yaml"), nil
}

func expandHome(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}

	if path == "~" {
		return home, nil
	}

	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}
