package manifest

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/emkaytec/forge/internal/aws/accounts"
	"github.com/emkaytec/forge/internal/ui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	anvilAPIVersion             = "anvil.emkaytec.dev/v1alpha1"
	anvilGitHubRepositoryKind   = "GitHubRepository"
	defaultGitHubVisibility     = "private"
	defaultGitHubDefaultBranch  = "main"
	defaultTerraformProjectName = "*"
	defaultTerraformMode        = "remote"
	defaultTerraformAWSRegion   = "us-east-1"

	// ForgeDirName is the hidden container Anvil reads for desired-state YAML.
	ForgeDirName = ".forge"
)

var (
	githubRepositoryNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	environmentNamePattern      = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	awsAccountIDPattern         = regexp.MustCompile(`^[0-9]{12}$`)
)

type gitHubRepoGenerateOptions struct {
	outputDir        string
	name             string
	visibility       string
	description      string
	homepage         string
	topics           []string
	defaultBranch    string
	terraform        bool
	environment      string
	accountProfile   string
	accountID        string
	projectName      string
	executionMode    string
	terraformVersion string
}

type anvilGitHubRepositoryManifest struct {
	APIVersion string                    `yaml:"apiVersion"`
	Kind       string                    `yaml:"kind"`
	Metadata   anvilMetadata             `yaml:"metadata"`
	Spec       anvilGitHubRepositorySpec `yaml:"spec"`
}

type anvilMetadata struct {
	Name string `yaml:"name"`
}

type anvilGitHubRepositorySpec struct {
	CreateTerraformWorkspaces bool                        `yaml:"createTerraformWorkspaces,omitempty"`
	Repository                anvilRepository             `yaml:"repository"`
	AWS                       *anvilAWS                   `yaml:"aws,omitempty"`
	Environments              map[string]anvilEnvironment `yaml:"environments,omitempty"`
	Workspace                 *anvilWorkspace             `yaml:"workspace,omitempty"`
}

type anvilRepository struct {
	Name          string            `yaml:"name,omitempty"`
	Description   string            `yaml:"description,omitempty"`
	Visibility    string            `yaml:"visibility,omitempty"`
	Homepage      string            `yaml:"homepage,omitempty"`
	Topics        []string          `yaml:"topics,omitempty"`
	AutoInit      *bool             `yaml:"autoInit,omitempty"`
	DefaultBranch string            `yaml:"defaultBranch,omitempty"`
	Features      *anvilFeatures    `yaml:"features,omitempty"`
	MergePolicy   *anvilMergePolicy `yaml:"mergePolicy,omitempty"`
}

type anvilFeatures struct {
	HasIssues      *bool `yaml:"hasIssues,omitempty"`
	HasProjects    *bool `yaml:"hasProjects,omitempty"`
	HasWiki        *bool `yaml:"hasWiki,omitempty"`
	HasDiscussions *bool `yaml:"hasDiscussions,omitempty"`
}

type anvilMergePolicy struct {
	AllowSquashMerge    *bool `yaml:"allowSquashMerge,omitempty"`
	AllowMergeCommit    *bool `yaml:"allowMergeCommit,omitempty"`
	AllowRebaseMerge    *bool `yaml:"allowRebaseMerge,omitempty"`
	DeleteBranchOnMerge *bool `yaml:"deleteBranchOnMerge,omitempty"`
}

type anvilAWS struct {
	Region string `yaml:"region,omitempty"`
}

type anvilEnvironment struct {
	AWS anvilEnvironmentAWS `yaml:"aws"`
}

type anvilEnvironmentAWS struct {
	AccountID string `yaml:"accountId"`
}

type anvilWorkspace struct {
	ProjectName      string `yaml:"projectName,omitempty"`
	ExecutionMode    string `yaml:"executionMode,omitempty"`
	TerraformVersion string `yaml:"terraformVersion,omitempty"`
}

func newGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate Anvil-compatible manifests",
		Long:  "Generate manifests for the supported Anvil Terraform YAML contracts.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newGenerateGitHubRepoCommand())

	return cmd
}

func newGenerateGitHubRepoCommand() *cobra.Command {
	var options gitHubRepoGenerateOptions

	cmd := &cobra.Command{
		Use:   "github-repo [name]",
		Short: "Write an Anvil GitHubRepository manifest",
		Long: strings.TrimSpace(`Write an Anvil-compatible GitHubRepository manifest.

Forge writes one YAML file to .forge/<name>.yaml. When Terraform workspace
creation is enabled, Forge prompts for the minimum required environment and AWS
account inputs needed by the Anvil Terraform module workflow.`),
		Example: strings.Join([]string{
			"  forge manifest generate github-repo docs-site",
			"  forge manifest generate github-repo complete-service --terraform --environment admin --account-id 123456789012",
			"  forge manifest generate github-repo --name complete-service --visibility private --topic terraform",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutputDir(options.outputDir); err != nil {
				return err
			}

			p := newPromptSession(cmd.InOrStdin(), cmd.OutOrStdout())
			configureGitHubRepoFlow(p)

			repositoryName, err := resolveRepositoryName(p, args, options.name)
			if err != nil {
				return err
			}

			terraformRepo, err := resolveTerraformRepo(p, cmd.Flags().Changed("terraform"), options)
			if err != nil {
				return err
			}

			visibility, err := resolveVisibility(options.visibility)
			if err != nil {
				return err
			}

			defaultBranch := strings.TrimSpace(options.defaultBranch)
			if defaultBranch == "" {
				defaultBranch = defaultGitHubDefaultBranch
			}

			manifest := anvilGitHubRepositoryManifest{
				APIVersion: anvilAPIVersion,
				Kind:       anvilGitHubRepositoryKind,
				Metadata: anvilMetadata{
					Name: repositoryName,
				},
				Spec: anvilGitHubRepositorySpec{
					Repository: anvilRepository{
						Name:          repositoryName,
						Description:   strings.TrimSpace(options.description),
						Visibility:    visibility,
						Homepage:      strings.TrimSpace(options.homepage),
						Topics:        normalizeStringList(options.topics),
						AutoInit:      boolPtr(true),
						DefaultBranch: defaultBranch,
						Features: &anvilFeatures{
							HasIssues:      boolPtr(true),
							HasProjects:    boolPtr(false),
							HasWiki:        boolPtr(false),
							HasDiscussions: boolPtr(false),
						},
						MergePolicy: &anvilMergePolicy{
							AllowSquashMerge:    boolPtr(true),
							AllowMergeCommit:    boolPtr(false),
							AllowRebaseMerge:    boolPtr(true),
							DeleteBranchOnMerge: boolPtr(true),
						},
					},
				},
			}

			if terraformRepo {
				if err := populateTerraformInputs(p, &manifest, options); err != nil {
					return err
				}
			}

			if p.preludeDone {
				fmt.Fprintln(p.out)
			}

			contents, err := renderAnvilManifest(manifest)
			if err != nil {
				return err
			}

			return writeGeneratedManifest(cmd, repositoryName, options.outputDir, contents)
		},
	}

	cmd.Flags().StringVar(&options.outputDir, "dir", "", "Write the generated manifest under this relative directory")
	cmd.Flags().StringVar(&options.name, "name", "", "GitHub repository name")
	cmd.Flags().StringVar(&options.visibility, "visibility", "", "Repository visibility: public, private, or internal")
	cmd.Flags().StringVar(&options.description, "description", "", "Repository description")
	cmd.Flags().StringVar(&options.homepage, "homepage", "", "Repository homepage URL")
	cmd.Flags().StringSliceVar(&options.topics, "topic", nil, "GitHub topic slug to attach (repeat or comma-separate)")
	cmd.Flags().StringVar(&options.defaultBranch, "default-branch", "", "Default branch name")
	cmd.Flags().BoolVar(&options.terraform, "terraform", false, "Create Terraform workspace and AWS provisioning resources")
	cmd.Flags().StringVar(&options.environment, "environment", "", "Terraform environment key, e.g. admin, dev, pre, or prod")
	cmd.Flags().StringVar(&options.accountProfile, "account-profile", "", "AWS shared-config profile to derive the account ID from")
	cmd.Flags().StringVar(&options.accountID, "account-id", "", "12-digit AWS account ID for the Terraform environment")
	cmd.Flags().StringVar(&options.projectName, "project-name", "", "HCP Terraform project name to use in generated workspace subjects")
	cmd.Flags().StringVar(&options.executionMode, "execution-mode", "", "HCP Terraform execution mode: remote, local, or agent")
	cmd.Flags().StringVar(&options.terraformVersion, "terraform-version", "", "Pinned Terraform version for generated workspaces")

	return cmd
}

func populateTerraformInputs(p *promptSession, manifest *anvilGitHubRepositoryManifest, options gitHubRepoGenerateOptions) error {
	environment, err := resolveTerraformEnvironment(p, options.environment)
	if err != nil {
		return err
	}

	accountID, err := resolveAWSAccountID(p, options.accountProfile, options.accountID, environment)
	if err != nil {
		return err
	}

	projectName := strings.TrimSpace(options.projectName)
	if projectName == "" {
		projectName = defaultTerraformProjectName
	}

	executionMode, err := resolveExecutionMode(options.executionMode)
	if err != nil {
		return err
	}

	manifest.Spec.CreateTerraformWorkspaces = true
	manifest.Spec.AWS = &anvilAWS{Region: defaultTerraformAWSRegion}
	manifest.Spec.Environments = map[string]anvilEnvironment{
		environment: {
			AWS: anvilEnvironmentAWS{AccountID: accountID},
		},
	}
	manifest.Spec.Workspace = &anvilWorkspace{
		ProjectName:      projectName,
		ExecutionMode:    executionMode,
		TerraformVersion: strings.TrimSpace(options.terraformVersion),
	}

	return nil
}

func renderAnvilManifest(manifest anvilGitHubRepositoryManifest) (string, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(manifest); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}

	contents := buf.String()
	if err := validateAnvilManifest([]byte(contents)); err != nil {
		return "", fmt.Errorf("generated manifest is invalid: %w", err)
	}

	return contents, nil
}

func writeGeneratedManifest(cmd *cobra.Command, manifestName, outputDir, contents string) error {
	path, err := generatedManifestOutputPath(manifestName, outputDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return err
	}

	ui.Success(cmd.OutOrStdout(), fmt.Sprintf("Wrote github-repo manifest to %s", path))
	return nil
}

func generatedManifestOutputPath(name, dir string) (string, error) {
	baseDir, err := resolveBaseOutputDir(dir)
	if err != nil {
		return "", err
	}

	return filepath.Join(baseDir, ForgeDirName, name+".yaml"), nil
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

// runPrelude emits the flow-level prelude exactly once, right before the first
// prompt writes output.
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
		"Repository name",
		"Is this a Terraform repo?",
		"Environment",
		"AWS account",
		"AWS account ID",
	})
}

func resolveRepositoryName(p *promptSession, args []string, flagValue string) (string, error) {
	normalizedFlag, err := normalizeRepositoryName(flagValue)
	if err != nil && strings.TrimSpace(flagValue) != "" {
		return "", err
	}

	if len(args) > 0 {
		normalizedArg, err := normalizeRepositoryName(args[0])
		if err != nil {
			return "", err
		}
		if normalizedFlag != "" && normalizedFlag != normalizedArg {
			return "", fmt.Errorf("repository name %q does not match --name %q", normalizedArg, normalizedFlag)
		}
		return normalizedArg, nil
	}

	if normalizedFlag != "" {
		return normalizedFlag, nil
	}

	rawValue, err := inputPrompt(p, "Repository name", "", true)
	if err != nil {
		return "", err
	}

	return normalizeRepositoryName(rawValue)
}

func normalizeRepositoryName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("repository name must not be empty")
	}
	if !githubRepositoryNamePattern.MatchString(value) {
		return "", fmt.Errorf("repository name must contain only letters, numbers, dots, underscores, and hyphens")
	}
	return value, nil
}

func resolveTerraformRepo(p *promptSession, flagChanged bool, options gitHubRepoGenerateOptions) (bool, error) {
	if flagChanged {
		return options.terraform, nil
	}
	if hasTerraformInputs(options) {
		return true, nil
	}
	return resolveYesNo(p, "Is this a Terraform repo?", false, false, false)
}

func hasTerraformInputs(options gitHubRepoGenerateOptions) bool {
	return strings.TrimSpace(options.environment) != "" ||
		strings.TrimSpace(options.accountProfile) != "" ||
		strings.TrimSpace(options.accountID) != "" ||
		strings.TrimSpace(options.projectName) != "" ||
		strings.TrimSpace(options.executionMode) != "" ||
		strings.TrimSpace(options.terraformVersion) != ""
}

func resolveVisibility(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultGitHubVisibility, nil
	}
	switch value {
	case "public", "private", "internal":
		return value, nil
	default:
		return "", fmt.Errorf("invalid visibility %q; allowed: public, private, internal", value)
	}
}

func terraformEnvironmentOptions() []selectOption {
	return []selectOption{
		{Label: "Admin", Value: "admin"},
		{Label: "Development", Value: "dev"},
		{Label: "Pre-prod", Value: "pre"},
		{Label: "Production", Value: "prod"},
	}
}

func resolveTerraformEnvironment(p *promptSession, flagValue string) (string, error) {
	flagValue = strings.TrimSpace(flagValue)
	if flagValue != "" {
		return normalizeEnvironmentName(flagValue)
	}

	selected, err := selectOnePrompt(p, "Environment", terraformEnvironmentOptions(), 0)
	if err != nil {
		return "", err
	}
	return selected.Value, nil
}

func normalizeEnvironmentName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("environment must not be empty")
	}
	if !environmentNamePattern.MatchString(value) {
		return "", fmt.Errorf("environment must start with a lowercase letter and contain only lowercase letters, numbers, and hyphens")
	}
	return value, nil
}

func resolveExecutionMode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultTerraformMode, nil
	}
	switch value {
	case "remote", "local", "agent":
		return value, nil
	default:
		return "", fmt.Errorf("invalid execution mode %q; allowed: remote, local, agent", value)
	}
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

func resolveAWSAccountID(p *promptSession, accountProfile, accountID, preferredEnvironment string) (string, error) {
	accountProfile = strings.TrimSpace(accountProfile)
	accountID = strings.TrimSpace(accountID)

	if accountProfile == "" && accountID != "" {
		return validateAWSAccountID(accountID)
	}

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
			return validateAWSAccountID(accountID)
		}
		if profile.AccountID == "" {
			return "", fmt.Errorf("AWS profile %q does not expose an account ID; pass --account-id", accountProfile)
		}
		return validateAWSAccountID(profile.AccountID)
	}

	if len(profiles) == 0 {
		value, err := inputPrompt(p, "AWS account ID", "", true)
		if err != nil {
			return "", err
		}
		return validateAWSAccountID(value)
	}

	orderedProfiles, defaultIndex := accounts.PrioritizeProfiles(profiles, preferredEnvironment)
	options := make([]selectOption, 0, len(orderedProfiles)+1)
	for _, profile := range orderedProfiles {
		options = append(options, selectOption{Label: accounts.Label(profile), Value: profile.Name})
	}
	options = append(options, selectOption{Label: "Enter an account ID manually", Value: "manual"})

	selected, err := selectOnePrompt(p, "AWS account", options, defaultIndex)
	if err != nil {
		return "", err
	}
	if selected.Value == "manual" {
		value, err := inputPrompt(p, "AWS account ID", "", true)
		if err != nil {
			return "", err
		}
		return validateAWSAccountID(value)
	}

	profile, _ := accounts.FindProfile(orderedProfiles, selected.Value)
	if profile.AccountID != "" {
		return validateAWSAccountID(profile.AccountID)
	}

	value, err := inputPrompt(p, "AWS account ID", "", true)
	if err != nil {
		return "", err
	}
	return validateAWSAccountID(value)
}

func validateAWSAccountID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !awsAccountIDPattern.MatchString(value) {
		return "", fmt.Errorf("AWS account ID must be a 12-digit number")
	}
	return value, nil
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			t := strings.TrimSpace(part)
			if t == "" {
				continue
			}
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			result = append(result, t)
		}
	}
	return result
}

func boolPtr(v bool) *bool {
	return &v
}

func discoverManifestFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		if !isYAMLFile(path) {
			return nil, fmt.Errorf("%s is not a .yaml or .yml manifest file", path)
		}
		return []string{path}, nil
	}

	var files []string
	if err := filepath.WalkDir(path, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if isYAMLFile(path) {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("%s does not contain any .yaml or .yml manifest files", path)
	}
	return files, nil
}

func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}
