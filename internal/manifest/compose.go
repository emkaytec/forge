package manifest

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type composeTerraformGitHubRepoOptions struct {
	outputDir        string
	application      string
	owner            string
	visibility       string
	description      string
	topics           []string
	defaultBranch    string
	environments     []string
	accountProfiles  []string
	accountIDs       []string
	organization     string
	project          string
	executionMode    string
	terraformVersion string
	managedPolicies  []string
}

func newComposeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Compose higher-level Forge manifest blueprints",
		Long:  "Compose higher-level blueprints into several primitive Forge manifests.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newComposeTerraformGitHubRepoCommand())

	return cmd
}

func newComposeTerraformGitHubRepoCommand() *cobra.Command {
	var options composeTerraformGitHubRepoOptions

	cmd := &cobra.Command{
		Use:   "terraform-github-repo [application]",
		Short: "Compose a repo stack into github-repo, hcp-tf-workspace, and aws-iam-provisioner manifests",
		Long: strings.TrimSpace(`Compose a repo stack into github-repo, hcp-tf-workspace, and aws-iam-provisioner manifests.

If the required inputs are not provided as flags, Forge prompts for:
  1. the application name and repository owner
  2. the repository visibility, description, topics, and default branch
  3. one or more deployment environments plus the AWS account for each one
  4. the shared HCP Terraform organization, project, execution mode, and Terraform version
  5. the managed policy ARNs to attach to generated provisioner roles

Forge writes:
  - <application>/github-repo.yaml
  - <application>/hcp-tf-workspace-<env>.yml for each selected environment
  - <application>/aws-iam-provisioner-<env>-gha.yaml and
    <application>/aws-iam-provisioner-<env>-tfc.yaml for each selected environment`),
		Example: strings.Join([]string{
			"  forge manifest compose terraform-github-repo forge",
			"  forge manifest compose terraform-github-repo --application forge --owner emkaytec --environment dev --account-id dev=123456789012",
			"  forge manifest compose terraform-github-repo forge --environment dev --environment prod --account-profile dev=emkaytec-dev --account-profile prod=emkaytec-prod",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutputDir(options.outputDir); err != nil {
				return err
			}

			applicationArgs := []string(nil)
			if len(args) == 1 {
				if strings.TrimSpace(args[0]) == "" {
					return fmt.Errorf("application name must not be empty")
				}
				applicationArgs = []string{args[0]}
			}

			return runComposeTerraformGitHubRepo(cmd, applicationArgs, options)
		},
	}

	cmd.Flags().StringVar(&options.outputDir, "dir", "", "Write the generated manifests under this relative directory")
	cmd.Flags().StringVar(&options.application, "application", "", "Application name to use for the repo name and shared output directory")
	cmd.Flags().StringVar(&options.owner, "owner", "", "GitHub user or organization that will own the repository")
	cmd.Flags().StringVar(&options.visibility, "visibility", "", "Repository visibility: public or private")
	cmd.Flags().StringVar(&options.description, "description", "", "Repository description (optional)")
	cmd.Flags().StringSliceVar(&options.topics, "topic", nil, "GitHub topic slug to attach (repeat or comma-separate)")
	cmd.Flags().StringVar(&options.defaultBranch, "default-branch", "", "Default branch name")
	cmd.Flags().StringSliceVar(&options.environments, "environment", nil, "Deployment environment to generate: dev, pre, or prod (repeat or comma-separate)")
	cmd.Flags().StringSliceVar(&options.accountProfiles, "account-profile", nil, "Environment-specific AWS profile mapping in the form env=profile (repeat or comma-separate)")
	cmd.Flags().StringSliceVar(&options.accountIDs, "account-id", nil, "Environment-specific AWS account ID mapping in the form env=123456789012 (repeat or comma-separate)")
	cmd.Flags().StringVar(&options.organization, "organization", "", "HCP Terraform organization slug")
	cmd.Flags().StringVar(&options.project, "project", "", "HCP Terraform project name")
	cmd.Flags().StringVar(&options.executionMode, "execution-mode", "", "Execution mode: remote, local, or agent")
	cmd.Flags().StringVar(&options.terraformVersion, "terraform-version", "", "Pinned Terraform version for the generated workspaces")
	cmd.Flags().StringSliceVar(&options.managedPolicies, "managed-policy", nil, "Managed policy ARN to attach to generated provisioner roles (repeat or comma-separate)")

	return cmd
}

func runComposeTerraformGitHubRepo(cmd *cobra.Command, args []string, options composeTerraformGitHubRepoOptions) error {
	p := newPromptSession(cmd.InOrStdin(), cmd.OutOrStdout())
	configureComposeTerraformGitHubRepoFlow(p)

	applicationName, err := resolveApplicationName(p, args, options.application)
	if err != nil {
		return err
	}

	ownerValue, err := resolveGitHubOwner(cmd.Context(), p, options.owner)
	if err != nil {
		return err
	}

	visibilityValue, err := resolveSelect(p, "Visibility", options.visibility, []selectOption{
		{Label: "Private", Value: "private"},
		{Label: "Public", Value: "public"},
	}, 0)
	if err != nil {
		return err
	}

	descriptionValue, err := resolveOptionalText(p, "Description", options.description, "")
	if err != nil {
		return err
	}

	topicsValue, err := resolveCSV(p, "Topics (comma-separated)", options.topics, nil)
	if err != nil {
		return err
	}

	defaultBranchValue, err := resolveOptionalText(p, "Default branch", options.defaultBranch, defaultGitHubDefaultBranch)
	if err != nil {
		return err
	}

	environmentsValue, err := resolveComposeEnvironments(p, options.environments)
	if err != nil {
		return err
	}

	accountProfilesByEnvironment, err := parseComposeEnvironmentValues(options.accountProfiles, "account-profile")
	if err != nil {
		return err
	}

	accountIDsByEnvironment, err := parseComposeEnvironmentValues(options.accountIDs, "account-id")
	if err != nil {
		return err
	}

	resolvedAccountIDs, err := resolveComposeEnvironmentAccountIDs(p, environmentsValue, accountProfilesByEnvironment, accountIDsByEnvironment)
	if err != nil {
		return err
	}

	organizationValue, err := resolveRequiredText(p, "HCP TF organization", options.organization, defaultHCPTFOrganization)
	if err != nil {
		return err
	}

	projectValue, err := resolveOptionalText(p, "HCP TF project", options.project, defaultHCPTFProject)
	if err != nil {
		return err
	}

	executionModeValue, err := resolveSelect(p, "Execution mode", options.executionMode, []selectOption{
		{Label: "Remote", Value: "remote"},
		{Label: "Local", Value: "local"},
		{Label: "Agent", Value: "agent"},
	}, 0)
	if err != nil {
		return err
	}

	terraformVersionValue, err := resolveOptionalText(p, "Terraform version", options.terraformVersion, defaultHCPTFTerraformVersion)
	if err != nil {
		return err
	}

	managedPoliciesValue, err := resolveManagedPolicies(p, options.managedPolicies)
	if err != nil {
		return err
	}

	if p.preludeDone {
		fmt.Fprintln(p.out)
	}

	vcsRepo := ownerValue + "/" + applicationName
	manifestRootName := scopedManifestName(ownerValue, applicationName)
	generatorCommand := fmt.Sprintf("forge manifest compose terraform-github-repo %s", applicationName)

	if err := writeGeneratedManifest(
		cmd,
		applicationName,
		"github-repo",
		options.outputDir,
		renderGitHubRepoTemplateWithData(gitHubRepoTemplateData{
			GeneratorCommand: generatorCommand,
			ApplicationName:  applicationName,
			ManifestName:     manifestRootName,
			Owner:            ownerValue,
			Visibility:       visibilityValue,
			Description:      descriptionValue,
			Topics:           topicsValue,
			DefaultBranch:    defaultBranchValue,
		}),
	); err != nil {
		return err
	}

	for _, environmentValue := range environmentsValue {
		workspaceName := appendHCPTFEnvironmentSuffix(applicationName, environmentValue)
		manifestName := appendHCPTFEnvironmentSuffix(manifestRootName, environmentValue)

		if err := writeGeneratedManifestWithFilename(
			cmd,
			applicationName,
			"hcp-tf-workspace",
			"hcp-tf-workspace-"+environmentValue+".yml",
			options.outputDir,
			renderHCPTFWorkspaceTemplateWithData(hcpTFWorkspaceTemplateData{
				GeneratorCommand: generatorCommand,
				ManifestName:     manifestName,
				WorkspaceName:    workspaceName,
				Environment:      environmentValue,
				Organization:     organizationValue,
				Project:          projectValue,
				AccountID:        resolvedAccountIDs[environmentValue],
				VCSRepo:          vcsRepo,
				ExecutionMode:    executionModeValue,
				TerraformVersion: terraformVersionValue,
			}),
		); err != nil {
			return err
		}

		targets, err := defaultAWSIAMProvisionerTargets(vcsRepo, environmentValue)
		if err != nil {
			return err
		}

		if err := writeAWSIAMProvisionerManifests(
			cmd,
			applicationName,
			manifestRootName,
			environmentValue,
			resolvedAccountIDs[environmentValue],
			generatorCommand,
			defaultAWSIAMProvisionerProviders(),
			targets,
			managedPoliciesValue,
			options.outputDir,
		); err != nil {
			return err
		}
	}

	return nil
}

func configureComposeTerraformGitHubRepoFlow(p *promptSession) {
	configureFlow(p, "Compose terraform-github-repo", []string{
		"Application name",
		"Repository owner",
		"Visibility",
		"Description",
		"Topics (comma-separated)",
		"Default branch",
		"Environments",
		"Development AWS account",
		"Development AWS account ID",
		"Pre-prod AWS account",
		"Pre-prod AWS account ID",
		"Prod AWS account",
		"Prod AWS account ID",
		"HCP TF organization",
		"HCP TF project",
		"Execution mode",
		"Terraform version",
		"Managed policy ARNs (comma-separated)",
	})
}

func resolveComposeEnvironments(p *promptSession, flagValues []string) ([]string, error) {
	options := hcpTFEnvironmentOptions()
	if len(flagValues) == 0 {
		selected, err := selectManyPrompt(p, "Environments", options, []int{0})
		if err != nil {
			return nil, err
		}

		values := make([]string, 0, len(selected))
		for _, option := range selected {
			values = append(values, option.Value)
		}
		return values, nil
	}

	selectedValues := map[string]struct{}{}
	for _, flagValue := range normalizeStringList(flagValues) {
		flagValue = strings.TrimSpace(flagValue)
		if flagValue == "" {
			continue
		}

		valid := false
		for _, option := range options {
			if option.Value == flagValue {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("invalid environment %q; allowed: dev, pre, prod", flagValue)
		}
		selectedValues[flagValue] = struct{}{}
	}

	environments := make([]string, 0, len(selectedValues))
	for _, option := range options {
		if _, ok := selectedValues[option.Value]; ok {
			environments = append(environments, option.Value)
		}
	}

	if len(environments) == 0 {
		return nil, fmt.Errorf("at least one environment must be selected")
	}

	return environments, nil
}

func parseComposeEnvironmentValues(flagValues []string, flagName string) (map[string]string, error) {
	valuesByEnvironment := make(map[string]string, len(flagValues))
	for _, flagValue := range normalizeStringList(flagValues) {
		parts := strings.SplitN(strings.TrimSpace(flagValue), "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("%s must use env=value, got %q", flagName, flagValue)
		}

		environment := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if value == "" {
			return nil, fmt.Errorf("%s must use env=value, got %q", flagName, flagValue)
		}

		switch environment {
		case "dev", "pre", "prod":
		default:
			return nil, fmt.Errorf("%s uses unsupported environment %q; allowed: dev, pre, prod", flagName, environment)
		}

		if _, exists := valuesByEnvironment[environment]; exists {
			return nil, fmt.Errorf("%s already set for environment %q", flagName, environment)
		}

		valuesByEnvironment[environment] = value
	}

	return valuesByEnvironment, nil
}

func resolveComposeEnvironmentAccountIDs(p *promptSession, environments []string, accountProfilesByEnvironment, accountIDsByEnvironment map[string]string) (map[string]string, error) {
	resolved := make(map[string]string, len(environments))
	for _, environment := range environments {
		environmentLabel := hcpTFEnvironmentLabel(environment)
		accountID, err := resolveAWSAccountIDWithLabels(
			p,
			environmentLabel+" AWS account",
			environmentLabel+" AWS account ID",
			accountProfilesByEnvironment[environment],
			accountIDsByEnvironment[environment],
			environment,
		)
		if err != nil {
			return nil, err
		}
		resolved[environment] = accountID
	}

	return resolved, nil
}
