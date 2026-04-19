package manifest

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/emkaytec/forge/internal/aws/accounts"
	"github.com/emkaytec/forge/internal/aws/oidc"
	"github.com/emkaytec/forge/internal/ui"
	"github.com/emkaytec/forge/pkg/schema"
	"github.com/spf13/cobra"
)

var defaultManagedPolicies = []string{"arn:aws:iam::aws:policy/ReadOnlyAccess"}

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
	ApplicationName string
	RoleName        string
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
		newGenerateAWSIAMProvisionerCommand(),
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
			var (
				name     string
				contents string
				err      error
			)

			if len(args) == 0 {
				name, contents, err = template.promptRender(cmd)
			} else {
				name = strings.TrimSpace(args[0])
				if name == "" {
					return fmt.Errorf("manifest name must not be empty")
				}
				contents = template.render(name)
			}
			if err != nil {
				return err
			}

			return writeGeneratedManifest(cmd, name, template.filename, outputDir, contents, defaultOutputPath)
		},
	}

	cmd.Flags().StringVar(&outputDir, "dir", "", "Write the generated manifest into this relative directory")

	return cmd
}

func newGenerateAWSIAMProvisionerCommand() *cobra.Command {
	var (
		outputDir      string
		application    string
		accountProfile string
		accountID      string
		providerKeys   []string
		githubRepo     string
		hcpWorkspace   string
		managedPolicy  []string
	)

	cmd := &cobra.Command{
		Use:   "aws-iam-provisioner [application]",
		Short: "Write a starter aws-iam-provisioner manifest",
		Long: strings.TrimSpace(`Write a starter aws-iam-provisioner manifest.

If the required inputs are not provided as flags, Forge prompts for:
  1. the application name
  2. the AWS account to target
  3. one or more provisioning systems (GitHub Actions and/or HCP Terraform)
  4. the repository or workspace identities used in the OIDC subjects
  5. the managed policy ARNs to attach

By default, Forge writes the manifest to <application>/aws-iam-provisioner.yaml.
If multiple provisioning systems are selected, Forge writes one manifest per
system under the application directory.`),
		Example: strings.Join([]string{
			"  forge manifest generate aws-iam-provisioner forge",
			"  forge manifest generate aws-iam-provisioner --application forge --account-id 123456789012 --provider github-actions --github-repo emkaytec/forge --managed-policy arn:aws:iam::aws:policy/ReadOnlyAccess",
			"  forge manifest generate aws-iam-provisioner --application forge --account-profile prod-admin --provider hcp-terraform --hcp-workspace emkaytec/platform/forge",
			"  forge manifest generate aws-iam-provisioner --application forge --provider github-actions --provider hcp-terraform --github-repo emkaytec/forge --hcp-workspace emkaytec/platform/forge",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := newPromptSession(cmd.InOrStdin(), cmd.OutOrStdout())

			applicationName, err := resolveApplicationName(p, args, application)
			if err != nil {
				return err
			}

			accountIDValue, err := resolveAWSAccountID(p, accountProfile, accountID)
			if err != nil {
				return err
			}

			providers, err := resolveOIDCProviders(p, providerKeys)
			if err != nil {
				return err
			}

			targets, err := resolveProviderTargets(p, providers, githubRepo, hcpWorkspace)
			if err != nil {
				return err
			}

			policies, err := resolveManagedPolicies(p, managedPolicy)
			if err != nil {
				return err
			}

			return writeAWSIAMProvisionerManifests(cmd, applicationName, accountIDValue, providers, targets, policies, outputDir)
		},
	}

	cmd.Flags().StringVar(&outputDir, "dir", "", "Write the generated manifest under this relative directory")
	cmd.Flags().StringVar(&application, "application", "", "Application name to use for metadata.name and the output directory")
	cmd.Flags().StringVar(&accountProfile, "account-profile", "", "AWS config profile to resolve the target account from")
	cmd.Flags().StringVar(&accountID, "account-id", "", "12-digit AWS account ID to write into spec.account_id")
	cmd.Flags().StringSliceVar(&providerKeys, "provider", nil, "Provisioning system to trust: github-actions or hcp-terraform (repeat to generate multiple provisioners)")
	cmd.Flags().StringVar(&githubRepo, "github-repo", "", "GitHub repository path to trust for GitHub Actions, such as emkaytec/forge")
	cmd.Flags().StringVar(&hcpWorkspace, "hcp-workspace", "", "HCP Terraform workspace path to trust, such as emkaytec/platform/forge")
	cmd.Flags().StringSliceVar(&managedPolicy, "managed-policy", nil, "Managed policy ARN to attach (repeat or comma-separate)")

	return cmd
}

type outputPathResolver func(manifestName, resource, dir string) (string, error)

func writeGeneratedManifest(cmd *cobra.Command, manifestName, resource, outputDir, contents string, resolve outputPathResolver) error {
	if err := validateGeneratedManifest(contents); err != nil {
		return err
	}

	path, err := resolve(manifestName, resource, outputDir)
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

func defaultOutputPath(name, _resource, dir string) (string, error) {
	baseDir, err := resolveBaseOutputDir(dir)
	if err != nil {
		return "", err
	}

	return filepath.Join(baseDir, name+".yaml"), nil
}

func applicationDirectoryOutputPath(name, resource, dir string) (string, error) {
	baseDir, err := resolveBaseOutputDir(dir)
	if err != nil {
		return "", err
	}

	return filepath.Join(baseDir, name, resource+".yaml"), nil
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
	in     io.Reader
	reader *bufio.Reader
	out    io.Writer
}

func newPromptSession(in io.Reader, out io.Writer) *promptSession {
	return &promptSession{
		in:     in,
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
	seen := map[string]struct{}{}
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		values = append(values, trimmed)
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

func resolveApplicationName(p *promptSession, args []string, flagValue string) (string, error) {
	flagValue = strings.TrimSpace(flagValue)
	if len(args) > 0 {
		argValue := strings.TrimSpace(args[0])
		if flagValue != "" && flagValue != argValue {
			return "", fmt.Errorf("application name %q does not match --application %q", argValue, flagValue)
		}
		if argValue == "" {
			return "", fmt.Errorf("application name must not be empty")
		}
		return argValue, nil
	}

	if flagValue != "" {
		return flagValue, nil
	}

	return p.required("Application name", "")
}

func resolveAWSAccountID(p *promptSession, accountProfile, accountID string) (string, error) {
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
		return p.required("AWS account ID", "")
	}

	options := make([]selectOption, 0, len(profiles)+1)
	for _, profile := range profiles {
		label := profile.Name
		if profile.AccountID != "" {
			label += " (" + profile.AccountID + ")"
		} else {
			label += " (account ID unavailable)"
		}
		options = append(options, selectOption{Label: label, Value: profile.Name})
	}
	options = append(options, selectOption{Label: "Enter an account ID manually", Value: "manual"})

	selected, err := selectOnePrompt(p, "AWS account", options, 0)
	if err != nil {
		return "", err
	}
	if selected.Value == "manual" {
		return p.required("AWS account ID", "")
	}

	profile, _ := accounts.FindProfile(profiles, selected.Value)
	if profile.AccountID != "" {
		return profile.AccountID, nil
	}

	return p.required("AWS account ID", "")
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
	githubRepo = strings.TrimSpace(githubRepo)
	hcpWorkspace = strings.TrimSpace(hcpWorkspace)

	switch provider.Key {
	case "github-actions":
		if githubRepo != "" {
			return githubRepo, nil
		}
		return p.required(provider.TargetLabel, "")
	case "hcp-terraform":
		if hcpWorkspace != "" {
			return hcpWorkspace, nil
		}
		return p.required(provider.TargetLabel, "")
	default:
		return "", fmt.Errorf("unsupported provider %q", provider.Key)
	}
}

func writeAWSIAMProvisionerManifests(cmd *cobra.Command, applicationName, accountID string, providers []oidc.Provider, targets map[string]string, policies []string, outputDir string) error {
	multiProvider := len(providers) > 1
	for _, provider := range providers {
		target, ok := targets[provider.Key]
		if !ok {
			return fmt.Errorf("missing target identity for provider %q", provider.Key)
		}

		subject, err := provider.BuildSubject(target)
		if err != nil {
			return err
		}

		manifestName := applicationName
		roleName := applicationName + "-provisioner-role"
		resourceName := "aws-iam-provisioner"
		if multiProvider {
			manifestName = applicationName + "-" + provider.NameSuffix
			roleName = applicationName + "-" + provider.NameSuffix + "-provisioner-role"
			resourceName = resourceName + "-" + provider.NameSuffix
		}

		contents := renderAWSIAMProvisionerTemplateWithData(awsIAMProvisionerTemplateData{
			ApplicationName: manifestName,
			RoleName:        roleName,
			AccountID:       accountID,
			OIDCProvider:    provider.Issuer,
			OIDCSubject:     subject,
			ManagedPolicies: policies,
		})

		if err := writeGeneratedManifest(cmd, applicationName, resourceName, outputDir, contents, applicationDirectoryOutputPath); err != nil {
			return err
		}
	}

	return nil
}

func resolveManagedPolicies(p *promptSession, flagValues []string) ([]string, error) {
	if len(flagValues) > 0 {
		values := make([]string, 0, len(flagValues))
		seen := map[string]struct{}{}
		for _, value := range flagValues {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			values = append(values, trimmed)
		}
		return values, nil
	}

	return p.csv("Managed policy ARNs (comma-separated)", defaultManagedPolicies)
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

func renderAWSIAMProvisionerTemplateWithData(data awsIAMProvisionerTemplateData) string {
	return fmt.Sprintf(`# Generated by "forge manifest generate aws-iam-provisioner %s".
apiVersion: forge/v1
kind: aws-iam-provisioner
metadata:
  # metadata.name is the application identifier Forge reports on.
  name: %q
spec:
  # spec.name is the AWS IAM role Forge will manage for this application.
  name: %q
  # account_id is the 12-digit AWS account identifier.
  account_id: %q
  # oidc_provider is the trusted OIDC issuer host.
  oidc_provider: %q
  # oidc_subject scopes which workload identity may assume this role.
  oidc_subject: %q
  # managed_policies is optional and attaches AWS managed policy ARNs.
%s
`, data.ApplicationName, data.ApplicationName, data.RoleName, data.AccountID, data.OIDCProvider, data.OIDCSubject, renderStringListBlock("managed_policies", data.ManagedPolicies))
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
