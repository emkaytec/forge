package schema

import (
	"fmt"
	"unicode/utf8"
)

const (
	// APIVersionV1 is the first supported Forge manifest API version.
	APIVersionV1 = "forge/v1"

	// AWSIAMRoleNameMaxLength matches the AWS IAM role-name maximum length.
	AWSIAMRoleNameMaxLength = 64
)

// Kind identifies a supported manifest schema.
type Kind string

const (
	KindGitHubRepo        Kind = "GitHubRepository"
	KindHCPTFWorkspace    Kind = "HCPTerraformWorkspace"
	KindAWSIAMProvisioner Kind = "AWSIAMProvisioner"
	KindLaunchAgent       Kind = "LaunchAgent"
)

// Metadata contains the stable manifest envelope metadata.
type Metadata struct {
	Name string `yaml:"name"`
}

// Manifest is the typed Forge manifest envelope.
type Manifest struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       Kind     `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       any      `yaml:"spec"`
}

// GitHubRepoSpec is the initial GitHub repository schema staged in Forge.
type GitHubRepoSpec struct {
	Owner            string   `yaml:"owner"`
	Name             string   `yaml:"name"`
	Visibility       string   `yaml:"visibility"`
	Description      string   `yaml:"description,omitempty"`
	Topics           []string `yaml:"topics,omitempty"`
	DefaultBranch    string   `yaml:"default_branch,omitempty"`
	BranchProtection bool     `yaml:"branch_protection,omitempty"`
}

// HCPTFWorkspaceSpec is the initial HCP Terraform workspace schema staged in Forge.
type HCPTFWorkspaceSpec struct {
	Name             string `yaml:"name"`
	Organization     string `yaml:"organization"`
	Project          string `yaml:"project,omitempty"`
	VCSRepo          string `yaml:"vcs_repo,omitempty"`
	ExecutionMode    string `yaml:"execution_mode"`
	TerraformVersion string `yaml:"terraform_version,omitempty"`
}

// AWSIAMProvisionerSpec is the initial OIDC-backed AWS provisioner schema.
type AWSIAMProvisionerSpec struct {
	Name            string   `yaml:"name"`
	AccountID       string   `yaml:"account_id"`
	OIDCProvider    string   `yaml:"oidc_provider"`
	OIDCSubject     string   `yaml:"oidc_subject"`
	ManagedPolicies []string `yaml:"managed_policies,omitempty"`
}

// LaunchAgentSpec is the initial LaunchAgent schema staged in Forge.
type LaunchAgentSpec struct {
	Name      string              `yaml:"name"`
	Label     string              `yaml:"label"`
	Command   string              `yaml:"command"`
	Schedule  LaunchAgentSchedule `yaml:"schedule"`
	RunAtLoad bool                `yaml:"run_at_load,omitempty"`
}

// LaunchAgentScheduleType identifies the supported LaunchAgent schedule shapes.
type LaunchAgentScheduleType string

const (
	ScheduleTypeInterval LaunchAgentScheduleType = "interval"
	ScheduleTypeCalendar LaunchAgentScheduleType = "calendar"
)

// LaunchAgentSchedule holds either interval-based or calendar-based schedule fields.
type LaunchAgentSchedule struct {
	Type            LaunchAgentScheduleType `yaml:"type"`
	IntervalSeconds int                     `yaml:"interval_seconds,omitempty"`
	Hour            int                     `yaml:"hour,omitempty"`
	Minute          int                     `yaml:"minute,omitempty"`
}

// Validate reports whether the manifest envelope and typed spec are schema-valid.
func (m *Manifest) Validate() error {
	if err := ValidateAPIVersion(m.APIVersion); err != nil {
		return err
	}

	if err := ValidateKind(m.Kind); err != nil {
		return err
	}

	if err := m.Metadata.Validate(); err != nil {
		return err
	}

	if m.Spec == nil {
		return invalidField("spec", "is required")
	}

	switch spec := m.Spec.(type) {
	case *GitHubRepoSpec:
		if m.Kind != KindGitHubRepo {
			return kindSpecMismatchError(m.Kind, spec)
		}

		spec.applyDefaults()
		return spec.Validate()
	case *HCPTFWorkspaceSpec:
		if m.Kind != KindHCPTFWorkspace {
			return kindSpecMismatchError(m.Kind, spec)
		}

		return spec.Validate()
	case *AWSIAMProvisionerSpec:
		if m.Kind != KindAWSIAMProvisioner {
			return kindSpecMismatchError(m.Kind, spec)
		}

		return spec.Validate()
	case *LaunchAgentSpec:
		if m.Kind != KindLaunchAgent {
			return kindSpecMismatchError(m.Kind, spec)
		}

		spec.applyDefaults()
		return spec.Validate()
	default:
		return fmt.Errorf("schema: unsupported spec type %T", m.Spec)
	}
}

// Validate reports whether the manifest metadata is schema-valid.
func (m Metadata) Validate() error {
	if m.Name == "" {
		return invalidField("metadata.name", "must not be empty")
	}

	return nil
}

// Validate reports whether the GitHub repository schema is valid.
func (s *GitHubRepoSpec) Validate() error {
	if s.Owner == "" {
		return invalidField("spec.owner", "must not be empty")
	}

	if s.Name == "" {
		return invalidField("spec.name", "must not be empty")
	}

	switch s.Visibility {
	case "public", "private":
	default:
		return invalidField("spec.visibility", "must be one of public or private")
	}

	return nil
}

func (s *GitHubRepoSpec) applyDefaults() {
	if s.DefaultBranch == "" {
		s.DefaultBranch = "main"
	}
}

// Validate reports whether the HCP Terraform workspace schema is valid.
func (s *HCPTFWorkspaceSpec) Validate() error {
	if s.Name == "" {
		return invalidField("spec.name", "must not be empty")
	}

	if s.Organization == "" {
		return invalidField("spec.organization", "must not be empty")
	}

	switch s.ExecutionMode {
	case "remote", "local", "agent":
	default:
		return invalidField("spec.execution_mode", "must be one of remote, local, or agent")
	}

	return nil
}

// Validate reports whether the AWS IAM provisioner schema is valid.
func (s *AWSIAMProvisionerSpec) Validate() error {
	if s.Name == "" {
		return invalidField("spec.name", "must not be empty")
	}
	if utf8.RuneCountInString(s.Name) > AWSIAMRoleNameMaxLength {
		return invalidField("spec.name", fmt.Sprintf("must not exceed %d characters", AWSIAMRoleNameMaxLength))
	}

	if s.AccountID == "" {
		return invalidField("spec.account_id", "must not be empty")
	}

	if s.OIDCProvider == "" {
		return invalidField("spec.oidc_provider", "must not be empty")
	}

	if s.OIDCSubject == "" {
		return invalidField("spec.oidc_subject", "must not be empty")
	}

	return nil
}

// Validate reports whether the LaunchAgent schema is valid.
func (s *LaunchAgentSpec) Validate() error {
	if s.Name == "" {
		return invalidField("spec.name", "must not be empty")
	}

	if s.Label == "" {
		return invalidField("spec.label", "must not be empty")
	}

	if s.Command == "" {
		return invalidField("spec.command", "must not be empty")
	}

	return s.Schedule.Validate()
}

func (s *LaunchAgentSpec) applyDefaults() {
	s.RunAtLoad = s.RunAtLoad || false
}

// Validate reports whether the LaunchAgent schedule is valid.
func (s LaunchAgentSchedule) Validate() error {
	switch s.Type {
	case ScheduleTypeInterval:
		if s.IntervalSeconds <= 0 {
			return invalidField("spec.schedule.interval_seconds", "must be greater than zero for interval schedules")
		}

		if s.Hour != 0 || s.Minute != 0 {
			return invalidField("spec.schedule", "hour and minute are only supported for calendar schedules")
		}
	case ScheduleTypeCalendar:
		if s.IntervalSeconds != 0 {
			return invalidField("spec.schedule", "interval_seconds is only supported for interval schedules")
		}

		if s.Hour < 0 || s.Hour > 23 {
			return invalidField("spec.schedule.hour", "must be between 0 and 23 for calendar schedules")
		}

		if s.Minute < 0 || s.Minute > 59 {
			return invalidField("spec.schedule.minute", "must be between 0 and 59 for calendar schedules")
		}
	default:
		return invalidField("spec.schedule.type", "must be one of interval or calendar")
	}

	return nil
}

func kindSpecMismatchError(kind Kind, spec any) error {
	return fmt.Errorf("schema: manifest kind %q does not match spec type %T", kind, spec)
}
