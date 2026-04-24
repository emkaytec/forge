package manifest

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/emkaytec/forge/internal/aws/accounts"
	"github.com/emkaytec/forge/internal/aws/oidc"
	ghapi "github.com/emkaytec/forge/internal/github"
	"github.com/emkaytec/forge/internal/ui"
	"github.com/emkaytec/forge/pkg/schema"
	"github.com/spf13/cobra"
)

var defaultManagedPolicies = []string{"arn:aws:iam::aws:policy/ReadOnlyAccess"}

const (
	defaultGitHubVisibility        = "private"
	defaultGitHubDefaultBranch     = "main"
	defaultHCPTFOrganization       = "emkaytec"
	defaultHCPTFProject            = "*"
	defaultHCPTFExecutionMode      = "remote"
	defaultHCPTFTerraformVersion   = "1.14.0"
	defaultLaunchAgentScheduleKind = "interval"
	defaultLaunchAgentInterval     = 86400
	defaultLaunchAgentHour         = 9
	defaultLaunchAgentMinute       = 30
	defaultLaunchAgentRunAtLoad    = true
	launchAgentLabelPrefix         = "dev.emkaytec."
)

type gitHubRepoTemplateData struct {
	GeneratorCommand string
	ApplicationName  string
	ManifestName     string
	Owner            string
	Visibility       string
	Description      string
	Topics           []string
	DefaultBranch    string
}

type hcpTFWorkspaceTemplateData struct {
	GeneratorCommand string
	ManifestName     string
	WorkspaceName    string
	Environment      string
	Organization     string
	Project          string
	AccountID        string
	VCSRepo          string
	ExecutionMode    string
	TerraformVersion string
}

type awsIAMProvisionerTemplateData struct {
	GeneratorCommand string
	ApplicationName  string
	RoleName         string
	AccountID        string
	OIDCProvider     string
	OIDCSubject      string
	ManagedPolicies  []string
}

type launchAgentTemplateData struct {
	GeneratorCommand string
	ApplicationName  string
	Label            string
	Command          string
	ScheduleType     string
	IntervalSeconds  int
	Hour             int
	Minute           int
	RunAtLoad        bool
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
		newGenerateGitHubRepoCommand(),
		newGenerateHCPTFWorkspaceCommand(),
		newGenerateAWSIAMProvisionerCommand(),
		newGenerateLaunchAgentCommand(),
	)

	return cmd
}

func newGenerateGitHubRepoCommand() *cobra.Command {
	var (
		outputDir     string
		application   string
		owner         string
		visibility    string
		description   string
		topics        []string
		defaultBranch string
	)

	cmd := &cobra.Command{
		Use:   "github-repo [application]",
		Short: "Write a starter github-repo manifest",
		Long: strings.TrimSpace(`Write a starter github-repo manifest.

If the required inputs are not provided as flags, Forge prompts for:
  1. the application name
  2. the repository owner (user or organization); defaults to the current GitHub login when available
  3. the repository visibility (public or private)
  4. an optional description and topic list
  5. the default branch

Forge writes the manifest to .forge/<application>/github-repo.yaml under the application directory.`),
		Example: strings.Join([]string{
			"  forge manifest generate github-repo forge",
			"  forge manifest generate github-repo --application forge --owner emkaytec --visibility private --default-branch main",
			"  forge manifest generate github-repo --application forge --owner emkaytec --topic platform --topic automation",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutputDir(outputDir); err != nil {
				return err
			}

			p := newPromptSession(cmd.InOrStdin(), cmd.OutOrStdout())
			configureGitHubRepoFlow(p)

			applicationName, err := resolveApplicationName(p, args, application)
			if err != nil {
				return err
			}

			ownerValue, err := resolveGitHubOwner(cmd.Context(), p, owner)
			if err != nil {
				return err
			}

			visibilityValue, err := resolveSelect(p, "Visibility", visibility, []selectOption{
				{Label: "Private", Value: "private"},
				{Label: "Public", Value: "public"},
			}, 0)
			if err != nil {
				return err
			}

			descriptionValue, err := resolveOptionalText(p, "Description", description, "")
			if err != nil {
				return err
			}

			topicsValue, err := resolveCSV(p, "Topics (comma-separated)", topics, nil)
			if err != nil {
				return err
			}

			defaultBranchValue, err := resolveOptionalText(p, "Default branch", defaultBranch, defaultGitHubDefaultBranch)
			if err != nil {
				return err
			}

			if p.preludeDone {
				fmt.Fprintln(p.out)
			}

			manifestName := scopedManifestName(ownerValue, applicationName)
			data := gitHubRepoTemplateData{
				GeneratorCommand: fmt.Sprintf("forge manifest generate github-repo %s", applicationName),
				ApplicationName:  applicationName,
				ManifestName:     manifestName,
				Owner:            ownerValue,
				Visibility:       visibilityValue,
				Description:      descriptionValue,
				Topics:           topicsValue,
				DefaultBranch:    defaultBranchValue,
			}

			return writeGeneratedManifest(cmd, applicationName, "github-repo", outputDir, renderGitHubRepoTemplateWithData(data))
		},
	}

	cmd.Flags().StringVar(&outputDir, "dir", "", "Write the generated manifest under this relative directory")
	cmd.Flags().StringVar(&application, "application", "", "Application name to use for spec.name and the output directory")
	cmd.Flags().StringVar(&owner, "owner", "", "GitHub user or organization that will own the repository")
	cmd.Flags().StringVar(&visibility, "visibility", "", "Repository visibility: public or private")
	cmd.Flags().StringVar(&description, "description", "", "Repository description (optional)")
	cmd.Flags().StringSliceVar(&topics, "topic", nil, "GitHub topic slug to attach (repeat or comma-separate)")
	cmd.Flags().StringVar(&defaultBranch, "default-branch", "", "Default branch name")

	return cmd
}

func newGenerateHCPTFWorkspaceCommand() *cobra.Command {
	var (
		outputDir        string
		environment      string
		accountProfile   string
		accountID        string
		organization     string
		project          string
		vcsRepo          string
		executionMode    string
		terraformVersion string
	)

	cmd := &cobra.Command{
		Use:   "hcp-tf-workspace [vcs-repo]",
		Short: "Write a starter hcp-tf-workspace manifest",
		Long: strings.TrimSpace(`Write a starter hcp-tf-workspace manifest.

If the required inputs are not provided as flags, Forge prompts for:
  1. the connected VCS repo (owner/repo)
  2. the deployment environment and AWS account
  3. the HCP Terraform organization and optional project
  4. the execution mode and Terraform version

Forge derives a shared application directory from the repository name
(for example, emkaytec/forge becomes forge), appends the selected
environment to metadata.name and spec.name, and writes the
manifest to .forge/<application-name>/hcp-tf-workspace-<env>.yml.`),
		Example: strings.Join([]string{
			"  forge manifest generate hcp-tf-workspace emkaytec/forge --environment dev --account-id 123456789012",
			"  forge manifest generate hcp-tf-workspace --vcs-repo emkaytec/forge --environment pre --account-profile preprod-admin --organization emkaytec --project platform",
			"  forge manifest generate hcp-tf-workspace --vcs-repo emkaytec/forge --environment prod --account-id 123456789012 --organization emkaytec --execution-mode remote",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutputDir(outputDir); err != nil {
				return err
			}

			p := newPromptSession(cmd.InOrStdin(), cmd.OutOrStdout())
			configureHCPTFWorkspaceFlow(p)

			vcsRepoValue, err := resolveRequiredVCSRepo(p, args, vcsRepo)
			if err != nil {
				return err
			}

			environmentValue, err := resolveSelect(p, "Environment", environment, hcpTFEnvironmentOptions(), 0)
			if err != nil {
				return err
			}

			accountIDValue, err := resolveAWSAccountID(p, accountProfile, accountID, environmentValue)
			if err != nil {
				return err
			}

			applicationName, err := applicationNameFromVCSRepo(vcsRepoValue)
			if err != nil {
				return err
			}
			manifestBaseName, err := manifestNameFromVCSRepo(vcsRepoValue)
			if err != nil {
				return err
			}
			manifestName := appendHCPTFEnvironmentSuffix(manifestBaseName, environmentValue)

			workspaceName, err := workspaceNameFromVCSRepo(vcsRepoValue)
			if err != nil {
				return err
			}
			workspaceName = appendHCPTFEnvironmentSuffix(workspaceName, environmentValue)

			organizationValue, err := resolveRequiredText(p, "HCP TF organization", organization, defaultHCPTFOrganization)
			if err != nil {
				return err
			}

			projectValue, err := resolveOptionalText(p, "HCP TF project", project, defaultHCPTFProject)
			if err != nil {
				return err
			}

			executionModeValue, err := resolveSelect(p, "Execution mode", executionMode, []selectOption{
				{Label: "Remote", Value: "remote"},
				{Label: "Local", Value: "local"},
				{Label: "Agent", Value: "agent"},
			}, 0)
			if err != nil {
				return err
			}

			terraformVersionValue, err := resolveOptionalText(p, "Terraform version", terraformVersion, defaultHCPTFTerraformVersion)
			if err != nil {
				return err
			}

			if p.preludeDone {
				fmt.Fprintln(p.out)
			}

			data := hcpTFWorkspaceTemplateData{
				GeneratorCommand: fmt.Sprintf("forge manifest generate hcp-tf-workspace %s", vcsRepoValue),
				ManifestName:     manifestName,
				WorkspaceName:    workspaceName,
				Environment:      environmentValue,
				Organization:     organizationValue,
				Project:          projectValue,
				AccountID:        accountIDValue,
				VCSRepo:          vcsRepoValue,
				ExecutionMode:    executionModeValue,
				TerraformVersion: terraformVersionValue,
			}

			return writeGeneratedManifestWithFilename(
				cmd,
				applicationName,
				"hcp-tf-workspace",
				"hcp-tf-workspace-"+environmentValue+".yml",
				outputDir,
				renderHCPTFWorkspaceTemplateWithData(data),
			)
		},
	}

	cmd.Flags().StringVar(&outputDir, "dir", "", "Write the generated manifest under this relative directory")
	cmd.Flags().StringVar(&environment, "environment", "", "Deployment environment: dev, pre, prod, or admin")
	cmd.Flags().StringVar(&accountProfile, "account-profile", "", "AWS shared-config profile to derive the account ID from")
	cmd.Flags().StringVar(&accountID, "account-id", "", "12-digit AWS account ID to write into spec.account_id")
	cmd.Flags().StringVar(&organization, "organization", "", "HCP Terraform organization slug")
	cmd.Flags().StringVar(&project, "project", "", "HCP Terraform project name")
	cmd.Flags().StringVar(&vcsRepo, "vcs-repo", "", "Connected GitHub repository path, e.g. emkaytec/forge")
	cmd.Flags().StringVar(&executionMode, "execution-mode", "", "Execution mode: remote, local, or agent")
	cmd.Flags().StringVar(&terraformVersion, "terraform-version", "", "Pinned Terraform version for the workspace")

	return cmd
}

func newGenerateAWSIAMProvisionerCommand() *cobra.Command {
	var (
		outputDir      string
		vcsRepo        string
		environment    string
		accountProfile string
		accountID      string
		managedPolicy  []string
	)

	cmd := &cobra.Command{
		Use:   "aws-iam-provisioner [vcs-repo]",
		Short: "Write a starter aws-iam-provisioner manifest",
		Long: strings.TrimSpace(`Write a starter aws-iam-provisioner manifest.

If the required inputs are not provided as flags, Forge prompts for:
  1. the connected VCS repo (owner/repo)
  2. the deployment environment and AWS account
  3. the managed policy ARNs to attach

Forge always writes both GitHub Actions and HCP Terraform provisioner manifests.
The shared application directory comes from the repository name, while the
manifest and role names stay owner-scoped with the environment suffix. The HCP
Terraform trust subject is derived as <owner>/*/<repo>-<env> by default, and the
manifests are written as
.forge/<application>/aws-iam-provisioner-<env>-gha.yaml and
.forge/<application>/aws-iam-provisioner-<env>-tfc.yaml.`),
		Example: strings.Join([]string{
			"  forge manifest generate aws-iam-provisioner emkaytec/forge --environment dev --account-id 123456789012 --managed-policy arn:aws:iam::aws:policy/ReadOnlyAccess",
			"  forge manifest generate aws-iam-provisioner --vcs-repo emkaytec/forge --environment prod --account-profile prod-admin",
			"  forge manifest generate aws-iam-provisioner emkaytec/test-repo --environment pre --managed-policy arn:aws:iam::aws:policy/PowerUserAccess",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutputDir(outputDir); err != nil {
				return err
			}

			p := newPromptSession(cmd.InOrStdin(), cmd.OutOrStdout())
			configureAWSIAMProvisionerFlow(p)

			vcsRepoValue, err := resolveRequiredVCSRepo(p, args, vcsRepo)
			if err != nil {
				return err
			}

			environmentValue, err := resolveSelect(p, "Environment", environment, hcpTFEnvironmentOptions(), 0)
			if err != nil {
				return err
			}

			accountIDValue, err := resolveAWSAccountID(p, accountProfile, accountID, environmentValue)
			if err != nil {
				return err
			}

			directoryName, err := applicationNameFromVCSRepo(vcsRepoValue)
			if err != nil {
				return err
			}

			manifestRootName, err := manifestNameFromVCSRepo(vcsRepoValue)
			if err != nil {
				return err
			}

			targets, err := defaultAWSIAMProvisionerTargets(vcsRepoValue, environmentValue)
			if err != nil {
				return err
			}

			policies, err := resolveManagedPolicies(p, managedPolicy)
			if err != nil {
				return err
			}

			if p.preludeDone {
				fmt.Fprintln(p.out)
			}

			return writeAWSIAMProvisionerManifests(
				cmd,
				directoryName,
				manifestRootName,
				environmentValue,
				accountIDValue,
				fmt.Sprintf("forge manifest generate aws-iam-provisioner %s", vcsRepoValue),
				defaultAWSIAMProvisionerProviders(),
				targets,
				policies,
				outputDir,
			)
		},
	}

	cmd.Flags().StringVar(&outputDir, "dir", "", "Write the generated manifest under this relative directory")
	cmd.Flags().StringVar(&vcsRepo, "vcs-repo", "", "Connected GitHub repository path, e.g. emkaytec/forge")
	cmd.Flags().StringVar(&environment, "environment", "", "Deployment environment: dev, pre, prod, or admin")
	cmd.Flags().StringVar(&accountProfile, "account-profile", "", "AWS config profile to resolve the target account from")
	cmd.Flags().StringVar(&accountID, "account-id", "", "12-digit AWS account ID to write into spec.account_id")
	cmd.Flags().StringSliceVar(&managedPolicy, "managed-policy", nil, "Managed policy ARN to attach (repeat or comma-separate)")

	return cmd
}

func newGenerateLaunchAgentCommand() *cobra.Command {
	var (
		outputDir       string
		application     string
		command         string
		schedule        string
		intervalSeconds int
		hour            int
		minute          int
		runAtLoad       bool
	)

	cmd := &cobra.Command{
		Use:   "launch-agent [application]",
		Short: "Write a starter launch-agent manifest",
		Long: strings.TrimSpace(`Write a starter launch-agent manifest.

If the required inputs are not provided as flags, Forge prompts for:
  1. the application name (drives metadata.name and the launchd label)
  2. the command launchd should execute
  3. the schedule type and its parameters
  4. whether the agent should also run at load

Forge writes the manifest to .forge/<application>/launch-agent.yaml under the application directory.`),
		Example: strings.Join([]string{
			"  forge manifest generate launch-agent brew-update --command \"/opt/homebrew/bin/brew update\"",
			"  forge manifest generate launch-agent --application brew-update --command \"/opt/homebrew/bin/brew update\" --schedule interval --interval-seconds 86400",
			"  forge manifest generate launch-agent --application nightly-report --command \"/usr/local/bin/report.sh\" --schedule calendar --hour 2 --minute 15",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutputDir(outputDir); err != nil {
				return err
			}

			p := newPromptSession(cmd.InOrStdin(), cmd.OutOrStdout())
			configureLaunchAgentFlow(p)

			applicationName, err := resolveApplicationName(p, args, application)
			if err != nil {
				return err
			}

			commandValue, err := resolveRequiredText(p, "Command", command, "")
			if err != nil {
				return err
			}

			scheduleValue, err := resolveSelect(p, "Schedule type", schedule, []selectOption{
				{Label: "Interval", Value: "interval"},
				{Label: "Calendar", Value: "calendar"},
			}, 0)
			if err != nil {
				return err
			}

			data := launchAgentTemplateData{
				GeneratorCommand: fmt.Sprintf("forge manifest generate launch-agent %s", applicationName),
				ApplicationName:  applicationName,
				Label:            launchAgentLabelPrefix + applicationName,
				Command:          commandValue,
				ScheduleType:     scheduleValue,
			}

			switch scheduleValue {
			case "interval":
				data.IntervalSeconds, err = resolveInteger(p, "Interval seconds",
					cmd.Flags().Changed("interval-seconds"), intervalSeconds, defaultLaunchAgentInterval)
				if err != nil {
					return err
				}
			case "calendar":
				data.Hour, err = resolveInteger(p, "Hour (0–23)",
					cmd.Flags().Changed("hour"), hour, defaultLaunchAgentHour)
				if err != nil {
					return err
				}
				data.Minute, err = resolveInteger(p, "Minute (0–59)",
					cmd.Flags().Changed("minute"), minute, defaultLaunchAgentMinute)
				if err != nil {
					return err
				}
			}

			data.RunAtLoad, err = resolveYesNo(p, "Run at load",
				cmd.Flags().Changed("run-at-load"), runAtLoad, defaultLaunchAgentRunAtLoad)
			if err != nil {
				return err
			}

			if p.preludeDone {
				fmt.Fprintln(p.out)
			}

			return writeGeneratedManifest(cmd, applicationName, "launch-agent", outputDir, renderLaunchAgentTemplateWithData(data))
		},
	}

	cmd.Flags().StringVar(&outputDir, "dir", "", "Write the generated manifest under this relative directory")
	cmd.Flags().StringVar(&application, "application", "", "Application name to use for metadata.name and the launchd label")
	cmd.Flags().StringVar(&command, "command", "", "Command line launchd should execute")
	cmd.Flags().StringVar(&schedule, "schedule", "", "Schedule type: interval or calendar")
	cmd.Flags().IntVar(&intervalSeconds, "interval-seconds", 0, "Seconds between runs for interval schedules")
	cmd.Flags().IntVar(&hour, "hour", 0, "Hour (0–23) for calendar schedules")
	cmd.Flags().IntVar(&minute, "minute", 0, "Minute (0–59) for calendar schedules")
	cmd.Flags().BoolVar(&runAtLoad, "run-at-load", defaultLaunchAgentRunAtLoad, "Also run the agent when launchd loads it")

	return cmd
}

func writeGeneratedManifest(cmd *cobra.Command, applicationName, resource, outputDir, contents string) error {
	return writeGeneratedManifestWithFilename(cmd, applicationName, resource, generatedManifestFilename(resource), outputDir, contents)
}

func writeGeneratedManifestWithFilename(cmd *cobra.Command, applicationName, resource, filename, outputDir, contents string) error {
	if err := validateGeneratedManifest(contents); err != nil {
		return err
	}

	path, err := applicationDirectoryOutputPath(applicationName, filename, outputDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return err
	}

	ui.Success(cmd.OutOrStdout(), fmt.Sprintf("Wrote %s manifest to %s", resource, path))
	return nil
}

func validateGeneratedManifest(contents string) error {
	if _, err := schema.DecodeManifest([]byte(contents)); err != nil {
		return fmt.Errorf("generated manifest is invalid: %w", err)
	}

	return nil
}

// ForgeDirName is the hidden container that wraps per-application manifest
// directories (e.g. .forge/<application>/github-repo.yaml). The reconcile
// walker special-cases this name so the default hidden-directory skip does
// not hide generated manifests from `forge reconcile`.
const ForgeDirName = ".forge"

func applicationDirectoryOutputPath(name, filename, dir string) (string, error) {
	baseDir, err := resolveBaseOutputDir(dir)
	if err != nil {
		return "", err
	}

	return filepath.Join(baseDir, ForgeDirName, name, filename), nil
}

func generatedManifestFilename(resource string) string {
	return resource + ".yaml"
}

func validateOutputDir(dir string) error {
	if filepath.IsAbs(dir) {
		return fmt.Errorf("output directory must be relative, got %q", dir)
	}
	return nil
}

func resolveBaseOutputDir(dir string) (string, error) {
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

	return baseDir, nil
}

type promptSession struct {
	in          io.Reader
	reader      *bufio.Reader
	out         io.Writer
	labelWidth  int
	prelude     func(io.Writer)
	preludeDone bool
}

func newPromptSession(in io.Reader, out io.Writer) *promptSession {
	return &promptSession{
		in:     in,
		reader: bufio.NewReader(in),
		out:    out,
	}
}

// runPrelude emits the flow-level prelude (e.g. section header + leading
// blank line) exactly once, right before the first prompt writes output.
func (p *promptSession) runPrelude() {
	if p.prelude == nil || p.preludeDone {
		return
	}
	p.preludeDone = true
	p.prelude(p.out)
}

func (p *promptSession) required(label, defaultValue string) (string, error) {
	for {
		value, eof, err := p.line(label, defaultValue)
		if err != nil {
			return "", err
		}
		if value != "" {
			return value, nil
		}
		if eof {
			return "", fmt.Errorf("prompt canceled before %s was provided", strings.ToLower(label))
		}
		fmt.Fprintln(p.out, "Value is required.")
	}
}

func (p *promptSession) optional(label, defaultValue string) (string, error) {
	value, _, err := p.line(label, defaultValue)
	return value, err
}

func (p *promptSession) line(label, defaultValue string) (string, bool, error) {
	if defaultValue != "" {
		fmt.Fprintf(p.out, "%s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(p.out, "%s: ", label)
	}

	line, err := p.reader.ReadString('\n')
	trimmed := strings.TrimSpace(line)
	if err != nil {
		if err == io.EOF {
			if trimmed != "" {
				return trimmed, true, nil
			}
			return defaultValue, true, nil
		}
		return "", false, err
	}

	if trimmed == "" {
		return defaultValue, false, nil
	}
	return trimmed, false, nil
}

func configureFlow(p *promptSession, title string, labels []string) {
	p.labelWidth = ui.ChipLabelWidth(labels...)
	p.prelude = func(w io.Writer) {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.RenderSectionHeader(title))
		fmt.Fprintln(w)
	}
}

func configureGitHubRepoFlow(p *promptSession) {
	configureFlow(p, "Generate github-repo", []string{
		"Application name",
		"Repository owner",
		"Visibility",
		"Description",
		"Topics (comma-separated)",
		"Default branch",
	})
}

func configureHCPTFWorkspaceFlow(p *promptSession) {
	configureFlow(p, "Generate hcp-tf-workspace", []string{
		"VCS repo (owner/repo)",
		"Environment",
		"AWS account",
		"AWS account ID",
		"HCP TF organization",
		"HCP TF project",
		"Execution mode",
		"Terraform version",
	})
}

func hcpTFEnvironmentOptions() []selectOption {
	return []selectOption{
		{Label: "Development", Value: "dev"},
		{Label: "Pre-prod", Value: "pre"},
		{Label: "Prod", Value: "prod"},
		{Label: "Admin", Value: "admin"},
	}
}

func hcpTFEnvironmentLabel(environment string) string {
	for _, option := range hcpTFEnvironmentOptions() {
		if option.Value == strings.TrimSpace(environment) {
			return option.Label
		}
	}
	return strings.TrimSpace(environment)
}

func configureLaunchAgentFlow(p *promptSession) {
	configureFlow(p, "Generate launch-agent", []string{
		"Application name",
		"Command",
		"Schedule type",
		"Interval seconds",
		"Hour (0–23)",
		"Minute (0–59)",
		"Run at load",
	})
}

func configureAWSIAMProvisionerFlow(p *promptSession) {
	labels := []string{
		"VCS repo (owner/repo)",
		"Environment",
		"AWS account",
		"AWS account ID",
		"Managed policy ARNs (comma-separated)",
	}
	configureFlow(p, "Generate aws-iam-provisioner", labels)
}

func resolveApplicationName(p *promptSession, args []string, flagValue string) (string, error) {
	normalizedFlag, err := normalizeApplicationName(flagValue)
	if err != nil && strings.TrimSpace(flagValue) != "" {
		return "", err
	}

	if len(args) > 0 {
		normalizedArg, err := normalizeApplicationName(args[0])
		if err != nil {
			return "", err
		}
		if normalizedFlag != "" && normalizedFlag != normalizedArg {
			return "", fmt.Errorf("application name %q does not match --application %q", normalizedArg, normalizedFlag)
		}
		return normalizedArg, nil
	}

	if normalizedFlag != "" {
		return normalizedFlag, nil
	}

	rawValue, err := inputPrompt(p, "Application name", "", true)
	if err != nil {
		return "", err
	}

	return normalizeApplicationName(rawValue)
}

func resolveRequiredVCSRepo(p *promptSession, args []string, flagValue string) (string, error) {
	normalizedFlag, err := normalizeVCSRepo(flagValue)
	if err != nil && strings.TrimSpace(flagValue) != "" {
		return "", err
	}

	if len(args) > 0 {
		normalizedArg, err := normalizeVCSRepo(args[0])
		if err != nil {
			return "", err
		}
		if normalizedFlag != "" && normalizedFlag != normalizedArg {
			return "", fmt.Errorf("vcs repo %q does not match --vcs-repo %q", normalizedArg, normalizedFlag)
		}
		return normalizedArg, nil
	}

	if normalizedFlag != "" {
		return normalizedFlag, nil
	}

	rawValue, err := inputPrompt(p, "VCS repo (owner/repo)", "", true)
	if err != nil {
		return "", err
	}

	return normalizeVCSRepo(rawValue)
}

func normalizeApplicationName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("application name must not be empty")
	}

	var builder strings.Builder
	previousWasSeparator := false
	previousWasLowerOrDigit := false

	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			isUpper := unicode.IsUpper(r)
			if isUpper && previousWasLowerOrDigit && builder.Len() > 0 && !previousWasSeparator {
				builder.WriteByte('-')
			}
			builder.WriteRune(unicode.ToLower(r))
			previousWasSeparator = false
			previousWasLowerOrDigit = unicode.IsLower(r) || unicode.IsDigit(r)
		default:
			if builder.Len() > 0 && !previousWasSeparator {
				builder.WriteByte('-')
				previousWasSeparator = true
			}
			previousWasLowerOrDigit = false
		}
	}

	normalized := strings.Trim(builder.String(), "-")
	if normalized == "" {
		return "", fmt.Errorf("application name must contain letters or digits")
	}

	return normalized, nil
}

func normalizeVCSRepo(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("vcs repo must not be empty")
	}

	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("vcs repo must use owner/repo")
	}

	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	if owner == "" || repo == "" {
		return "", fmt.Errorf("vcs repo must use owner/repo")
	}

	return owner + "/" + repo, nil
}

func normalizeHCPTFWorkspaceTarget(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("HCP TF workspace must not be empty")
	}

	parts := strings.Split(value, "/")
	if len(parts) != 3 {
		return "", fmt.Errorf("HCP TF workspace must use organization/project/workspace")
	}

	organization := strings.TrimSpace(parts[0])
	project := strings.TrimSpace(parts[1])
	workspace := strings.TrimSpace(parts[2])
	if organization == "" || project == "" || workspace == "" {
		return "", fmt.Errorf("HCP TF workspace must use organization/project/workspace")
	}

	return organization + "/" + project + "/" + workspace, nil
}

func resolveRequiredHCPTFWorkspaceTarget(p *promptSession, flagValue string) (string, error) {
	flagValue = strings.TrimSpace(flagValue)
	if flagValue != "" {
		return normalizeHCPTFWorkspaceTarget(flagValue)
	}

	rawValue, err := inputPrompt(p, "HCP TF workspace (organization/project/workspace)", "", true)
	if err != nil {
		return "", err
	}

	return normalizeHCPTFWorkspaceTarget(rawValue)
}

func resolveRequiredText(p *promptSession, label, flagValue, defaultValue string) (string, error) {
	flagValue = strings.TrimSpace(flagValue)
	if flagValue != "" {
		return flagValue, nil
	}
	return inputPrompt(p, label, defaultValue, true)
}

func resolveOptionalText(p *promptSession, label, flagValue, defaultValue string) (string, error) {
	flagValue = strings.TrimSpace(flagValue)
	if flagValue != "" {
		return flagValue, nil
	}
	return inputPrompt(p, label, defaultValue, false)
}

func resolveSelect(p *promptSession, label, flagValue string, options []selectOption, defaultIndex int) (string, error) {
	flagValue = strings.TrimSpace(flagValue)
	if flagValue != "" {
		for _, opt := range options {
			if opt.Value == flagValue {
				return flagValue, nil
			}
		}
		allowed := make([]string, 0, len(options))
		for _, opt := range options {
			allowed = append(allowed, opt.Value)
		}
		return "", fmt.Errorf("invalid value %q for %s; allowed: %s", flagValue, label, strings.Join(allowed, ", "))
	}
	selected, err := selectOnePrompt(p, label, options, defaultIndex)
	if err != nil {
		return "", err
	}
	return selected.Value, nil
}

func resolveYesNo(p *promptSession, label string, flagChanged, flagValue, defaultValue bool) (bool, error) {
	if flagChanged {
		return flagValue, nil
	}
	defaultIndex := 1
	if defaultValue {
		defaultIndex = 0
	}
	selected, err := selectOnePrompt(p, label, []selectOption{
		{Label: "Yes", Value: "yes"},
		{Label: "No", Value: "no"},
	}, defaultIndex)
	if err != nil {
		return false, err
	}
	return selected.Value == "yes", nil
}

func resolveCSV(p *promptSession, label string, flagValues, defaultValues []string) ([]string, error) {
	if len(flagValues) > 0 {
		return normalizeStringList(flagValues), nil
	}
	defaultText := strings.Join(defaultValues, ",")
	raw, err := inputPrompt(p, label, defaultText, false)
	if err != nil {
		return nil, err
	}
	return parseCSVValues(raw), nil
}

func resolveInteger(p *promptSession, label string, flagChanged bool, flagValue, defaultValue int) (int, error) {
	if flagChanged {
		return flagValue, nil
	}
	for {
		raw, err := inputPrompt(p, label, strconv.Itoa(defaultValue), true)
		if err != nil {
			return 0, err
		}
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err == nil {
			return n, nil
		}
		fmt.Fprintln(p.out, "Enter a whole number.")
	}
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, v := range values {
		t := strings.TrimSpace(v)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		result = append(result, t)
	}
	return result
}

func parseCSVValues(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	return normalizeStringList(strings.Split(raw, ","))
}

// scopedManifestName combines the GitHub owner and the application name so
// the same repository name under different owners (e.g. alice/forge and
// bob/forge) produces distinct metadata.name values and output directories.
func scopedManifestName(owner, applicationName string) string {
	normalizedOwner := normalizeOwnerSlug(owner)
	if normalizedOwner == "" {
		return applicationName
	}
	if strings.HasPrefix(applicationName, normalizedOwner+"-") {
		return applicationName
	}
	return normalizedOwner + "-" + applicationName
}

// normalizeOwnerSlug lowercases the GitHub owner and replaces any runs of
// disallowed characters with a single hyphen. It intentionally does not split
// camelCase the way normalizeApplicationName does — GitHub logins are
// case-insensitive, so "EmKayTec" should round-trip to "emkaytec" rather than
// "em-kay-tec".
func normalizeOwnerSlug(owner string) string {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return ""
	}

	var b strings.Builder
	previousHyphen := false
	for _, r := range strings.ToLower(owner) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			previousHyphen = false
		default:
			if b.Len() > 0 && !previousHyphen {
				b.WriteByte('-')
				previousHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func manifestNameFromVCSRepo(vcsRepo string) (string, error) {
	owner, repo, err := splitVCSRepo(vcsRepo)
	if err != nil {
		return "", err
	}

	normalizedRepo, err := normalizeApplicationName(repo)
	if err != nil {
		return "", fmt.Errorf("vcs repo %q has invalid repository name: %w", vcsRepo, err)
	}

	return scopedManifestName(owner, normalizedRepo), nil
}

func applicationNameFromVCSRepo(vcsRepo string) (string, error) {
	_, repo, err := splitVCSRepo(vcsRepo)
	if err != nil {
		return "", err
	}

	normalizedRepo, err := normalizeApplicationName(repo)
	if err != nil {
		return "", fmt.Errorf("vcs repo %q has invalid repository name: %w", vcsRepo, err)
	}

	return normalizedRepo, nil
}

func defaultAWSIAMProvisionerProviders() []oidc.Provider {
	providers := make([]oidc.Provider, 0, 2)
	for _, key := range []string{"github-actions", "hcp-terraform"} {
		provider, ok := oidc.Lookup(key)
		if !ok {
			continue
		}
		providers = append(providers, provider)
	}
	return providers
}

func defaultAWSIAMProvisionerTargets(vcsRepo, environment string) (map[string]string, error) {
	owner, _, err := splitVCSRepo(vcsRepo)
	if err != nil {
		return nil, err
	}

	workspaceName, err := workspaceNameFromVCSRepo(vcsRepo)
	if err != nil {
		return nil, err
	}
	workspaceName = appendHCPTFEnvironmentSuffix(workspaceName, environment)

	return map[string]string{
		"github-actions": vcsRepo,
		"hcp-terraform":  strings.Join([]string{owner, defaultHCPTFProject, workspaceName}, "/"),
	}, nil
}

func workspaceNameFromVCSRepo(vcsRepo string) (string, error) {
	_, repo, err := splitVCSRepo(vcsRepo)
	if err != nil {
		return "", err
	}

	return repo, nil
}

func appendHCPTFEnvironmentSuffix(name, environment string) string {
	environment = strings.TrimSpace(environment)
	if environment == "" {
		return name
	}
	return name + "-" + environment
}

func splitVCSRepo(vcsRepo string) (string, string, error) {
	normalized, err := normalizeVCSRepo(vcsRepo)
	if err != nil {
		return "", "", err
	}

	parts := strings.SplitN(normalized, "/", 2)
	return parts[0], parts[1], nil
}

func splitHCPTFWorkspaceTarget(target string) (string, string, string, error) {
	normalized, err := normalizeHCPTFWorkspaceTarget(target)
	if err != nil {
		return "", "", "", err
	}

	parts := strings.SplitN(normalized, "/", 3)
	return parts[0], parts[1], parts[2], nil
}

func trimEnvironmentSuffix(name, environment string) string {
	environment = strings.TrimSpace(environment)
	if environment == "" {
		return name
	}

	suffix := "-" + environment
	if strings.HasSuffix(name, suffix) {
		trimmed := strings.TrimSuffix(name, suffix)
		if trimmed != "" {
			return trimmed
		}
	}

	return name
}

// ghMemberships groups the GitHub identities a single token can act on
// behalf of: the user's own login plus every organization they are a
// member of.
type ghMemberships struct {
	Login string
	Orgs  []string
}

const manualOwnerValue = "__manual__"

// resolveGitHubOwner returns spec.owner from --owner or a prompt. When
// prompting, Forge tries to fetch the authenticated user's login plus
// their organization memberships and presents them as a selector; if the
// lookup fails (no token, no network, reduced scopes) the prompt falls
// back to a free-form text entry with no default.
func resolveGitHubOwner(ctx context.Context, p *promptSession, flagValue string) (string, error) {
	if trimmed := strings.TrimSpace(flagValue); trimmed != "" {
		return trimmed, nil
	}

	memberships := currentGitHubMemberships(ctx)
	if memberships.Login == "" {
		return inputPrompt(p, "Repository owner", "", true)
	}

	options := make([]selectOption, 0, len(memberships.Orgs)+2)
	options = append(options, selectOption{
		Label: memberships.Login + " (personal)",
		Value: memberships.Login,
	})
	for _, org := range memberships.Orgs {
		options = append(options, selectOption{
			Label: org + " (organization)",
			Value: org,
		})
	}
	options = append(options, selectOption{
		Label: "Enter a different owner manually",
		Value: manualOwnerValue,
	})

	selected, err := selectOnePrompt(p, "Repository owner", options, 0)
	if err != nil {
		return "", err
	}
	if selected.Value == manualOwnerValue {
		return inputPrompt(p, "Repository owner", "", true)
	}
	return selected.Value, nil
}

// currentGitHubMemberships fetches the authenticated user's login plus
// the organizations they belong to. A short timeout keeps slow networks
// from stalling the generator; any failure (missing token, API error,
// reduced token scopes, timeout) returns a zero value so the caller can
// degrade to a free-form prompt. Org listing failures leave the login
// populated so the selector still shows the personal account.
var currentGitHubMemberships = func(ctx context.Context) ghMemberships {
	client, err := ghapi.NewClientFromEnv()
	if err != nil {
		return ghMemberships{}
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	account, err := client.GetAuthenticatedUser(ctx)
	if err != nil {
		return ghMemberships{}
	}

	orgs, err := client.ListUserOrganizations(ctx)
	if err != nil {
		return ghMemberships{Login: account.Login}
	}

	logins := make([]string, 0, len(orgs))
	for _, org := range orgs {
		if trimmed := strings.TrimSpace(org.Login); trimmed != "" {
			logins = append(logins, trimmed)
		}
	}
	sort.Strings(logins)

	return ghMemberships{Login: account.Login, Orgs: logins}
}

func resolveAWSAccountID(p *promptSession, accountProfile, accountID, preferredEnvironment string) (string, error) {
	return resolveAWSAccountIDWithLabels(p, "AWS account", "AWS account ID", accountProfile, accountID, preferredEnvironment)
}

func resolveAWSAccountIDWithLabels(p *promptSession, accountLabel, accountIDLabel, accountProfile, accountID, preferredEnvironment string) (string, error) {
	accountProfile = strings.TrimSpace(accountProfile)
	accountID = strings.TrimSpace(accountID)

	profiles, err := accounts.LoadProfiles()
	if err != nil {
		return "", err
	}

	if accountProfile != "" {
		profile, ok := accounts.FindProfile(profiles, accountProfile)
		if !ok {
			return "", fmt.Errorf("AWS profile %q was not found in local AWS config", accountProfile)
		}
		if accountID != "" {
			return accountID, nil
		}
		if profile.AccountID == "" {
			return "", fmt.Errorf("AWS profile %q does not expose an account ID; pass --account-id", accountProfile)
		}
		return profile.AccountID, nil
	}

	if accountID != "" {
		return accountID, nil
	}

	if len(profiles) == 0 {
		return inputPrompt(p, accountIDLabel, "", true)
	}

	orderedProfiles, defaultIndex := prioritizeAWSProfiles(profiles, preferredEnvironment)

	options := make([]selectOption, 0, len(orderedProfiles)+1)
	for _, profile := range orderedProfiles {
		label := profile.Name
		if profile.AccountID != "" {
			label += " (" + profile.AccountID + ")"
		} else {
			label += " (account ID unavailable)"
		}
		options = append(options, selectOption{Label: label, Value: profile.Name})
	}
	options = append(options, selectOption{Label: "Enter an account ID manually", Value: "manual"})

	selected, err := selectOnePrompt(p, accountLabel, options, defaultIndex)
	if err != nil {
		return "", err
	}
	if selected.Value == "manual" {
		return inputPrompt(p, accountIDLabel, "", true)
	}

	profile, _ := accounts.FindProfile(orderedProfiles, selected.Value)
	if profile.AccountID != "" {
		return profile.AccountID, nil
	}

	return inputPrompt(p, accountIDLabel, "", true)
}

func prioritizeAWSProfiles(profiles []accounts.Profile, environment string) ([]accounts.Profile, int) {
	environment = strings.TrimSpace(strings.ToLower(environment))
	if environment == "" || len(profiles) == 0 {
		return profiles, 0
	}

	matched := make([]accounts.Profile, 0, len(profiles))
	other := make([]accounts.Profile, 0, len(profiles))
	for _, profile := range profiles {
		if awsProfileMatchesEnvironment(profile.Name, environment) {
			matched = append(matched, profile)
			continue
		}
		other = append(other, profile)
	}

	if len(matched) == 0 {
		return profiles, 0
	}

	ordered := make([]accounts.Profile, 0, len(profiles))
	ordered = append(ordered, matched...)
	ordered = append(ordered, other...)
	return ordered, 0
}

func awsProfileMatchesEnvironment(name, environment string) bool {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" || environment == "" {
		return false
	}
	if name == environment {
		return true
	}

	tokens := strings.FieldsFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	for _, token := range tokens {
		if token == environment {
			return true
		}
	}

	return false
}

func resolveOIDCProviders(p *promptSession, providerKeys []string) ([]oidc.Provider, error) {
	if len(providerKeys) > 0 {
		resolved := make([]oidc.Provider, 0, len(providerKeys))
		seen := map[string]struct{}{}
		for _, providerKey := range providerKeys {
			providerKey = strings.TrimSpace(providerKey)
			if providerKey == "" {
				continue
			}
			if _, ok := seen[providerKey]; ok {
				continue
			}
			provider, ok := oidc.Lookup(providerKey)
			if !ok {
				return nil, fmt.Errorf("unsupported provider %q; use github-actions or hcp-terraform", providerKey)
			}
			seen[providerKey] = struct{}{}
			resolved = append(resolved, provider)
		}
		if len(resolved) > 0 {
			return resolved, nil
		}
	}

	available := oidc.Providers()
	options := make([]selectOption, 0, len(available))
	for _, provider := range available {
		options = append(options, selectOption{Label: provider.Label, Value: provider.Key})
	}

	selected, err := selectManyPrompt(p, "Provisioning systems", options, []int{0})
	if err != nil {
		return nil, err
	}

	resolved := make([]oidc.Provider, 0, len(selected))
	for _, option := range selected {
		provider, _ := oidc.Lookup(option.Value)
		resolved = append(resolved, provider)
	}

	return resolved, nil
}

func resolveProviderTargets(p *promptSession, providers []oidc.Provider, githubRepo, hcpWorkspace string) (map[string]string, error) {
	targets := make(map[string]string, len(providers))

	for _, provider := range providers {
		target, err := resolveProviderTarget(p, provider, githubRepo, hcpWorkspace)
		if err != nil {
			return nil, err
		}
		targets[provider.Key] = target
	}

	return targets, nil
}

func resolveProviderTarget(p *promptSession, provider oidc.Provider, githubRepo, hcpWorkspace string) (string, error) {
	switch provider.Key {
	case "github-actions":
		return resolveRequiredVCSRepo(p, nil, githubRepo)
	case "hcp-terraform":
		return resolveRequiredHCPTFWorkspaceTarget(p, hcpWorkspace)
	default:
		return "", fmt.Errorf("unsupported provider %q", provider.Key)
	}
}

func writeAWSIAMProvisionerManifests(cmd *cobra.Command, directoryName, manifestRootName, environment, accountID, generatorCommand string, providers []oidc.Provider, targets map[string]string, policies []string, outputDir string) error {
	for _, provider := range providers {
		target, ok := targets[provider.Key]
		if !ok {
			return fmt.Errorf("missing target identity for provider %q", provider.Key)
		}

		subject, err := provider.BuildSubject(target)
		if err != nil {
			return err
		}

		manifestBaseName := appendHCPTFEnvironmentSuffix(manifestRootName, environment)
		manifestName := manifestBaseName + "-" + provider.NameSuffix
		roleName, err := buildAWSIAMProvisionerRoleName(directoryName, environment, provider.NameSuffix)
		if err != nil {
			return err
		}
		resourceName := "aws-iam-provisioner-" + provider.NameSuffix
		filename := fmt.Sprintf("aws-iam-provisioner-%s-%s.yaml", environment, provider.NameSuffix)

		contents := renderAWSIAMProvisionerTemplateWithData(awsIAMProvisionerTemplateData{
			GeneratorCommand: generatorCommand,
			ApplicationName:  manifestName,
			RoleName:         roleName,
			AccountID:        accountID,
			OIDCProvider:     provider.Issuer,
			OIDCSubject:      subject,
			ManagedPolicies:  policies,
		})

		if err := writeGeneratedManifestWithFilename(cmd, directoryName, resourceName, filename, outputDir, contents); err != nil {
			return err
		}
	}

	return nil
}

func buildAWSIAMProvisionerRoleName(baseName, environment, providerSuffix string) (string, error) {
	roleSuffix := "-" + providerSuffix + "-provisioner-role"
	if strings.TrimSpace(environment) != "" {
		roleSuffix = "-" + environment + roleSuffix
	}
	maxApplicationLength := schema.AWSIAMRoleNameMaxLength - utf8.RuneCountInString(roleSuffix)
	if maxApplicationLength <= 0 {
		return "", fmt.Errorf("role suffix %q leaves no room for the application name", roleSuffix)
	}

	return truncateRunes(baseName, maxApplicationLength) + roleSuffix, nil
}

func truncateRunes(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}

	runes := []rune(value)
	return string(runes[:maxRunes])
}

func resolveManagedPolicies(p *promptSession, flagValues []string) ([]string, error) {
	if len(flagValues) > 0 {
		return normalizeStringList(flagValues), nil
	}

	defaultText := strings.Join(defaultManagedPolicies, ",")
	raw, err := inputPrompt(p, "Managed policy ARNs (comma-separated)", defaultText, false)
	if err != nil {
		return nil, err
	}
	return parseCSVValues(raw), nil
}

func renderGitHubRepoTemplateWithData(data gitHubRepoTemplateData) string {
	return fmt.Sprintf(`# Generated by %q.
apiVersion: forge/v1
kind: GitHubRepository
metadata:
  # metadata.name scopes the manifest identifier to the owner so the same
  # repository name under a different owner does not collide.
  name: %q
spec:
  # spec.owner is the GitHub user or organization that will own the repository.
  owner: %q
  # spec.name is the GitHub repository name to create or manage.
  name: %q
  # visibility must be either public or private.
  visibility: %s
  # description is optional.
  description: %q
  # topics is optional and should use GitHub topic slugs.
%s
  # default_branch is optional; Forge defaults it to main when omitted.
  default_branch: %s
`, data.GeneratorCommand, data.ManifestName, data.Owner, data.ApplicationName, data.Visibility, data.Description, renderStringListBlock("topics", data.Topics), data.DefaultBranch)
}

func renderHCPTFWorkspaceTemplateWithData(data hcpTFWorkspaceTemplateData) string {
	return fmt.Sprintf(`# Generated by %q.
apiVersion: forge/v1
kind: HCPTerraformWorkspace
metadata:
  # metadata.name is the stable manifest identifier derived from the VCS repo
  # plus the selected environment suffix.
  name: %q
spec:
  # spec.name is the HCP Terraform workspace name derived from the repo name
  # plus the selected environment suffix.
  name: %q
  # environment selects the managed workspace suffix; use dev, pre, prod, or admin.
  environment: %q
  # organization is the HCP Terraform organization slug.
  organization: %q
  # project is optional and can group related workspaces.
  project: %q
  # account_id is written to the workspace as a terraform variable named account_id.
  account_id: %q
  # vcs_repo is required and should use the connected GitHub identifier.
  vcs_repo: %q
  # execution_mode must be remote, local, or agent.
  execution_mode: %s
  # terraform_version is optional and pins the workspace runtime.
  terraform_version: %q
`, data.GeneratorCommand, data.ManifestName, data.WorkspaceName, data.Environment, data.Organization, data.Project, data.AccountID, data.VCSRepo, data.ExecutionMode, data.TerraformVersion)
}

func renderAWSIAMProvisionerTemplateWithData(data awsIAMProvisionerTemplateData) string {
	return fmt.Sprintf(`# Generated by %q.
apiVersion: forge/v1
kind: AWSIAMProvisioner
metadata:
  # metadata.name is the application identifier plus the environment and provider suffix.
  name: %q
spec:
  # spec.name is the AWS IAM role Forge will manage for this application and environment.
  name: %q
  # account_id is the 12-digit AWS account identifier.
  account_id: %q
  # oidc_provider is the trusted OIDC issuer host.
  oidc_provider: %q
  # oidc_subject scopes which workload identity may assume this role.
  oidc_subject: %q
  # managed_policies is optional and attaches AWS managed policy ARNs.
%s
`, data.GeneratorCommand, data.ApplicationName, data.RoleName, data.AccountID, data.OIDCProvider, data.OIDCSubject, renderStringListBlock("managed_policies", data.ManagedPolicies))
}

func renderLaunchAgentTemplateWithData(data launchAgentTemplateData) string {
	return fmt.Sprintf(`# Generated by %q.
apiVersion: forge/v1
kind: LaunchAgent
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
`, data.GeneratorCommand, data.ApplicationName, data.ApplicationName, data.Label, data.Command, data.ScheduleType, renderLaunchAgentSchedule(data), data.RunAtLoad)
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
