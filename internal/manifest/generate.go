package manifest

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/emkaytec/forge/internal/ui"
	"github.com/spf13/cobra"
)

type manifestTemplate struct {
	resource     string
	filename     string
	render       func(name string) string
	promptRender func(cmd *cobra.Command) (string, string, error)
}

type gitHubRepoTemplateData struct {
	ManifestName     string
	RepoName         string
	Visibility       string
	Description      string
	Topics           []string
	DefaultBranch    string
	BranchProtection bool
}

type hcpTFWorkspaceTemplateData struct {
	ManifestName     string
	WorkspaceName    string
	Organization     string
	Project          string
	VCSRepo          string
	ExecutionMode    string
	TerraformVersion string
}

type awsIAMProvisionerTemplateData struct {
	ManifestName    string
	ProvisionerName string
	AccountID       string
	OIDCProvider    string
	OIDCSubject     string
	ManagedPolicies []string
}

type launchAgentTemplateData struct {
	ManifestName    string
	AgentName       string
	Label           string
	Command         string
	ScheduleType    string
	IntervalSeconds int
	Hour            int
	Minute          int
	RunAtLoad       bool
}

func newGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate starter Forge manifests",
		Long:  "Generate starter manifests for the supported Forge schema kinds.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newGenerateTemplateCommand(manifestTemplate{
			resource:     "github-repo",
			filename:     "github-repo",
			render:       renderGitHubRepoTemplate,
			promptRender: promptGitHubRepoTemplate,
		}),
		newGenerateTemplateCommand(manifestTemplate{
			resource:     "hcp-tf-workspace",
			filename:     "hcp-tf-workspace",
			render:       renderHCPTFWorkspaceTemplate,
			promptRender: promptHCPTFWorkspaceTemplate,
		}),
		newGenerateTemplateCommand(manifestTemplate{
			resource:     "aws-iam-provisioner",
			filename:     "aws-iam-provisioner",
			render:       renderAWSIAMProvisionerTemplate,
			promptRender: promptAWSIAMProvisionerTemplate,
		}),
		newGenerateTemplateCommand(manifestTemplate{
			resource:     "launch-agent",
			filename:     "launch-agent",
			render:       renderLaunchAgentTemplate,
			promptRender: promptLaunchAgentTemplate,
		}),
	)

	return cmd
}

func newGenerateTemplateCommand(template manifestTemplate) *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:     template.resource + " [name]",
		Short:   fmt.Sprintf("Write a starter %s manifest", template.resource),
		Long:    "Write a starter manifest. If [name] is omitted, Forge prompts for the schema fields interactively.",
		Example: fmt.Sprintf("  forge manifest generate %s example\n  forge manifest generate %s", template.resource, template.resource),
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			var contents string
			var err error

			if len(args) == 0 {
				name, contents, err = template.promptRender(cmd)
				if err != nil {
					return err
				}
			} else {
				name = strings.TrimSpace(args[0])
				if name == "" {
					return fmt.Errorf("manifest name must not be empty")
				}
				contents = template.render(name)
			}

			path, err := resolveOutputPath(name, outputDir)
			if err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}

			if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
				return err
			}

			ui.Success(cmd.OutOrStdout(), fmt.Sprintf("Wrote %s manifest to %s", template.filename, path))
			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "dir", "", "Write the generated manifest into this relative directory")

	return cmd
}

func resolveOutputPath(name, dir string) (string, error) {
	if filepath.IsAbs(dir) {
		return "", fmt.Errorf("output directory must be relative, got %q", dir)
	}

	baseDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if dir != "" {
		baseDir = filepath.Join(baseDir, dir)
	}

	return filepath.Join(baseDir, name+".yaml"), nil
}

type promptSession struct {
	reader *bufio.Reader
	out    io.Writer
}

func newPromptSession(in io.Reader, out io.Writer) *promptSession {
	return &promptSession{
		reader: bufio.NewReader(in),
		out:    out,
	}
}

func (p *promptSession) required(label, defaultValue string) (string, error) {
	for {
		value, err := p.line(label, defaultValue)
		if err != nil {
			return "", err
		}
		if value != "" {
			return value, nil
		}
		fmt.Fprintln(p.out, "Value is required.")
	}
}

func (p *promptSession) optional(label, defaultValue string) (string, error) {
	return p.line(label, defaultValue)
}

func (p *promptSession) choice(label string, options []string, defaultValue string) (string, error) {
	for {
		prompt := fmt.Sprintf("%s (%s)", label, strings.Join(options, "/"))
		value, err := p.line(prompt, defaultValue)
		if err != nil {
			return "", err
		}
		for _, option := range options {
			if value == option {
				return value, nil
			}
		}
		fmt.Fprintf(p.out, "Choose one of: %s\n", strings.Join(options, ", "))
	}
}

func (p *promptSession) bool(label string, defaultValue bool) (bool, error) {
	defaultText := "no"
	if defaultValue {
		defaultText = "yes"
	}

	for {
		value, err := p.line(label+" (yes/no)", defaultText)
		if err != nil {
			return false, err
		}
		switch strings.ToLower(value) {
		case "y", "yes", "true":
			return true, nil
		case "n", "no", "false":
			return false, nil
		default:
			fmt.Fprintln(p.out, "Enter yes or no.")
		}
	}
}

func (p *promptSession) integer(label string, defaultValue int) (int, error) {
	for {
		value, err := p.line(label, strconv.Itoa(defaultValue))
		if err != nil {
			return 0, err
		}
		number, err := strconv.Atoi(value)
		if err == nil {
			return number, nil
		}
		fmt.Fprintln(p.out, "Enter a whole number.")
	}
}

func (p *promptSession) csv(label string, defaultValues []string) ([]string, error) {
	defaultText := strings.Join(defaultValues, ",")
	value, err := p.line(label, defaultText)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values, nil
}

func (p *promptSession) line(label, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Fprintf(p.out, "%s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(p.out, "%s: ", label)
	}

	line, err := p.reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			if strings.TrimSpace(line) != "" {
				return strings.TrimSpace(line), nil
			}
			return "", fmt.Errorf("prompt canceled before %s was provided", strings.ToLower(label))
		}
		return "", err
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}

func promptGitHubRepoTemplate(cmd *cobra.Command) (string, string, error) {
	p := newPromptSession(cmd.InOrStdin(), cmd.OutOrStdout())

	manifestName, err := p.required("Manifest name", "")
	if err != nil {
		return "", "", err
	}
	repoName, err := p.required("Repository name", manifestName)
	if err != nil {
		return "", "", err
	}
	visibility, err := p.choice("Visibility", []string{"public", "private"}, "public")
	if err != nil {
		return "", "", err
	}
	description, err := p.optional("Description", "Repository created by Forge")
	if err != nil {
		return "", "", err
	}
	topics, err := p.csv("Topics (comma-separated)", []string{"platform", "automation"})
	if err != nil {
		return "", "", err
	}
	defaultBranch, err := p.optional("Default branch", "main")
	if err != nil {
		return "", "", err
	}
	branchProtection, err := p.bool("Enable branch protection", true)
	if err != nil {
		return "", "", err
	}

	data := gitHubRepoTemplateData{
		ManifestName:     manifestName,
		RepoName:         repoName,
		Visibility:       visibility,
		Description:      description,
		Topics:           topics,
		DefaultBranch:    defaultBranch,
		BranchProtection: branchProtection,
	}
	return manifestName, renderGitHubRepoTemplateWithData(data), nil
}

func promptHCPTFWorkspaceTemplate(cmd *cobra.Command) (string, string, error) {
	p := newPromptSession(cmd.InOrStdin(), cmd.OutOrStdout())

	manifestName, err := p.required("Manifest name", "")
	if err != nil {
		return "", "", err
	}
	workspaceName, err := p.required("Workspace name", manifestName)
	if err != nil {
		return "", "", err
	}
	organization, err := p.required("Organization", "example-org")
	if err != nil {
		return "", "", err
	}
	project, err := p.optional("Project", "platform")
	if err != nil {
		return "", "", err
	}
	vcsRepo, err := p.optional("VCS repo", "emkaytec/forge")
	if err != nil {
		return "", "", err
	}
	executionMode, err := p.choice("Execution mode", []string{"remote", "local", "agent"}, "remote")
	if err != nil {
		return "", "", err
	}
	terraformVersion, err := p.optional("Terraform version", "1.9.8")
	if err != nil {
		return "", "", err
	}

	data := hcpTFWorkspaceTemplateData{
		ManifestName:     manifestName,
		WorkspaceName:    workspaceName,
		Organization:     organization,
		Project:          project,
		VCSRepo:          vcsRepo,
		ExecutionMode:    executionMode,
		TerraformVersion: terraformVersion,
	}
	return manifestName, renderHCPTFWorkspaceTemplateWithData(data), nil
}

func promptAWSIAMProvisionerTemplate(cmd *cobra.Command) (string, string, error) {
	p := newPromptSession(cmd.InOrStdin(), cmd.OutOrStdout())

	manifestName, err := p.required("Manifest name", "")
	if err != nil {
		return "", "", err
	}
	provisionerName, err := p.required("Provisioner role name", manifestName)
	if err != nil {
		return "", "", err
	}
	accountID, err := p.required("AWS account ID", "123456789012")
	if err != nil {
		return "", "", err
	}
	oidcProvider, err := p.required("OIDC provider", "token.actions.githubusercontent.com")
	if err != nil {
		return "", "", err
	}
	oidcSubject, err := p.required("OIDC subject", "repo:emkaytec/forge:ref:refs/heads/main")
	if err != nil {
		return "", "", err
	}
	managedPolicies, err := p.csv("Managed policy ARNs (comma-separated)", []string{"arn:aws:iam::aws:policy/ReadOnlyAccess"})
	if err != nil {
		return "", "", err
	}

	data := awsIAMProvisionerTemplateData{
		ManifestName:    manifestName,
		ProvisionerName: provisionerName,
		AccountID:       accountID,
		OIDCProvider:    oidcProvider,
		OIDCSubject:     oidcSubject,
		ManagedPolicies: managedPolicies,
	}
	return manifestName, renderAWSIAMProvisionerTemplateWithData(data), nil
}

func promptLaunchAgentTemplate(cmd *cobra.Command) (string, string, error) {
	p := newPromptSession(cmd.InOrStdin(), cmd.OutOrStdout())

	manifestName, err := p.required("Manifest name", "")
	if err != nil {
		return "", "", err
	}
	agentName, err := p.required("Launch agent name", manifestName)
	if err != nil {
		return "", "", err
	}
	label, err := p.required("Launchd label", "dev.emkaytec."+manifestName)
	if err != nil {
		return "", "", err
	}
	command, err := p.required("Command", "/opt/homebrew/bin/brew update")
	if err != nil {
		return "", "", err
	}
	scheduleType, err := p.choice("Schedule type", []string{"interval", "calendar"}, "interval")
	if err != nil {
		return "", "", err
	}

	data := launchAgentTemplateData{
		ManifestName: manifestName,
		AgentName:    agentName,
		Label:        label,
		Command:      command,
		ScheduleType: scheduleType,
		RunAtLoad:    true,
	}

	if scheduleType == "interval" {
		intervalSeconds, err := p.integer("Interval seconds", 86400)
		if err != nil {
			return "", "", err
		}
		data.IntervalSeconds = intervalSeconds
	} else {
		hour, err := p.integer("Hour", 9)
		if err != nil {
			return "", "", err
		}
		minute, err := p.integer("Minute", 30)
		if err != nil {
			return "", "", err
		}
		data.Hour = hour
		data.Minute = minute
	}

	runAtLoad, err := p.bool("Run at load", true)
	if err != nil {
		return "", "", err
	}
	data.RunAtLoad = runAtLoad

	return manifestName, renderLaunchAgentTemplateWithData(data), nil
}

func renderGitHubRepoTemplate(name string) string {
	return renderGitHubRepoTemplateWithData(gitHubRepoTemplateData{
		ManifestName:     name,
		RepoName:         name,
		Visibility:       "public",
		Description:      "Repository created by Forge",
		Topics:           []string{"platform", "automation"},
		DefaultBranch:    "main",
		BranchProtection: true,
	})
}

func renderGitHubRepoTemplateWithData(data gitHubRepoTemplateData) string {
	return fmt.Sprintf(`# Generated by "forge manifest generate github-repo %s".
apiVersion: forge/v1
kind: github-repo
metadata:
  # metadata.name is the stable manifest identifier Forge reports on.
  name: %q
spec:
  # spec.name is the GitHub repository name to create or manage.
  name: %q
  # visibility must be either public or private.
  visibility: %s
  # description is optional and should stay sanitized in this public repo.
  description: %q
  # topics is optional and should use GitHub topic slugs.
%s
  # default_branch is optional; Forge defaults it to main when omitted.
  default_branch: %s
  # branch_protection enables the baseline protected-branch workflow.
  branch_protection: %t
`, data.ManifestName, data.ManifestName, data.RepoName, data.Visibility, data.Description, renderStringListBlock("topics", data.Topics), data.DefaultBranch, data.BranchProtection)
}

func renderHCPTFWorkspaceTemplate(name string) string {
	return renderHCPTFWorkspaceTemplateWithData(hcpTFWorkspaceTemplateData{
		ManifestName:     name,
		WorkspaceName:    name,
		Organization:     "example-org",
		Project:          "platform",
		VCSRepo:          "emkaytec/forge",
		ExecutionMode:    "remote",
		TerraformVersion: "1.9.8",
	})
}

func renderHCPTFWorkspaceTemplateWithData(data hcpTFWorkspaceTemplateData) string {
	return fmt.Sprintf(`# Generated by "forge manifest generate hcp-tf-workspace %s".
apiVersion: forge/v1
kind: hcp-tf-workspace
metadata:
  # metadata.name is the stable manifest identifier Forge reports on.
  name: %q
spec:
  # spec.name is the HCP Terraform workspace name.
  name: %q
  # organization is the HCP Terraform organization slug.
  organization: %q
  # project is optional and can group related workspaces.
  project: %q
  # vcs_repo is optional and should use the connected VCS identifier.
  vcs_repo: %q
  # execution_mode must be remote, local, or agent.
  execution_mode: %s
  # terraform_version is optional and pins the workspace runtime.
  terraform_version: %q
`, data.ManifestName, data.ManifestName, data.WorkspaceName, data.Organization, data.Project, data.VCSRepo, data.ExecutionMode, data.TerraformVersion)
}

func renderAWSIAMProvisionerTemplate(name string) string {
	return renderAWSIAMProvisionerTemplateWithData(awsIAMProvisionerTemplateData{
		ManifestName:    name,
		ProvisionerName: name,
		AccountID:       "123456789012",
		OIDCProvider:    "token.actions.githubusercontent.com",
		OIDCSubject:     "repo:emkaytec/forge:ref:refs/heads/main",
		ManagedPolicies: []string{"arn:aws:iam::aws:policy/ReadOnlyAccess"},
	})
}

func renderAWSIAMProvisionerTemplateWithData(data awsIAMProvisionerTemplateData) string {
	return fmt.Sprintf(`# Generated by "forge manifest generate aws-iam-provisioner %s".
apiVersion: forge/v1
kind: aws-iam-provisioner
metadata:
  # metadata.name is the stable manifest identifier Forge reports on.
  name: %q
spec:
  # spec.name is the AWS IAM role name Forge will manage.
  name: %q
  # account_id is the 12-digit AWS account identifier.
  account_id: %q
  # oidc_provider is the trusted OIDC issuer host or ARN fragment.
  oidc_provider: %q
  # oidc_subject is the subject pattern allowed to assume this role.
  oidc_subject: %q
  # managed_policies is optional and attaches AWS managed policy ARNs.
%s
`, data.ManifestName, data.ManifestName, data.ProvisionerName, data.AccountID, data.OIDCProvider, data.OIDCSubject, renderStringListBlock("managed_policies", data.ManagedPolicies))
}

func renderLaunchAgentTemplate(name string) string {
	return renderLaunchAgentTemplateWithData(launchAgentTemplateData{
		ManifestName:    name,
		AgentName:       name,
		Label:           "dev.emkaytec." + name,
		Command:         "/opt/homebrew/bin/brew update",
		ScheduleType:    "interval",
		IntervalSeconds: 86400,
		RunAtLoad:       true,
	})
}

func renderLaunchAgentTemplateWithData(data launchAgentTemplateData) string {
	return fmt.Sprintf(`# Generated by "forge manifest generate launch-agent %s".
apiVersion: forge/v1
kind: launch-agent
metadata:
  # metadata.name is the stable manifest identifier Forge reports on.
  name: %q
spec:
  # spec.name is the operator-facing name for this launch agent.
  name: %q
  # label is the macOS launchd label and should stay globally unique.
  label: %q
  # command is the full command line launchd should execute.
  command: %q
  schedule:
    # type must be interval or calendar.
    type: %s
%s
  # run_at_load controls whether the agent also runs on load.
  run_at_load: %t
`, data.ManifestName, data.ManifestName, data.AgentName, data.Label, data.Command, data.ScheduleType, renderLaunchAgentSchedule(data), data.RunAtLoad)
}

func renderStringListBlock(field string, values []string) string {
	if len(values) == 0 {
		return fmt.Sprintf("  %s: []", field)
	}

	var builder strings.Builder
	builder.WriteString("  " + field + ":\n")
	for _, value := range values {
		builder.WriteString(fmt.Sprintf("    - %q\n", value))
	}
	return strings.TrimSuffix(builder.String(), "\n")
}

func renderLaunchAgentSchedule(data launchAgentTemplateData) string {
	if data.ScheduleType == "calendar" {
		return fmt.Sprintf("    # interval_seconds is only used for interval schedules.\n    # interval_seconds: 86400\n    # hour and minute are required for calendar schedules.\n    hour: %d\n    minute: %d", data.Hour, data.Minute)
	}

	return fmt.Sprintf("    # interval_seconds is required for interval schedules.\n    interval_seconds: %d\n    # hour and minute are only used for calendar schedules.\n    # hour: 9\n    # minute: 30", data.IntervalSeconds)
}
