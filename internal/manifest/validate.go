package manifest

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/emkaytec/forge/internal/ui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate Anvil manifest files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := discoverManifestFiles(args[0])
			if err != nil {
				return err
			}

			var invalid int
			for _, path := range paths {
				if err := validateManifestFile(path); err != nil {
					invalid++
					ui.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %s", path, describeValidationError(err)))
					continue
				}

				ui.Success(cmd.OutOrStdout(), fmt.Sprintf("%s is valid", path))
			}

			if invalid > 0 {
				return fmt.Errorf("validation failed for %d manifest(s)", invalid)
			}

			return nil
		},
	}
}

func validateManifestFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return validateAnvilManifest(data)
}

func describeValidationError(err error) string {
	var validationErr *anvilValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Error()
	}

	if strings.Contains(err.Error(), "cannot unmarshal") {
		return err.Error() + "; check that the YAML value has the expected Anvil manifest shape"
	}

	return err.Error()
}

type anvilValidationError struct {
	field  string
	reason string
}

func (e *anvilValidationError) Error() string {
	return fmt.Sprintf("%s %s", e.field, e.reason)
}

type anvilValidationManifest struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		CreateTerraformWorkspaces bool `yaml:"createTerraformWorkspaces"`
		Repository                struct {
			Name       string `yaml:"name"`
			Visibility string `yaml:"visibility"`
		} `yaml:"repository"`
		Environments map[string]struct {
			AWS struct {
				AccountID      string `yaml:"accountId"`
				AccountIDSnake string `yaml:"account_id"`
			} `yaml:"aws"`
		} `yaml:"environments"`
	} `yaml:"spec"`
}

func validateAnvilManifest(data []byte) error {
	var manifest anvilValidationManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("decode Anvil manifest: %w", err)
	}

	if manifest.APIVersion != anvilAPIVersion {
		return invalidAnvilField("apiVersion", fmt.Sprintf("must be %q", anvilAPIVersion))
	}
	if manifest.Kind != anvilGitHubRepositoryKind {
		return invalidAnvilField("kind", fmt.Sprintf("must be %q", anvilGitHubRepositoryKind))
	}
	if strings.TrimSpace(manifest.Metadata.Name) == "" {
		return invalidAnvilField("metadata.name", "must not be empty")
	}

	if name := strings.TrimSpace(manifest.Spec.Repository.Name); name != "" {
		if _, err := normalizeRepositoryName(name); err != nil {
			return invalidAnvilField("spec.repository.name", err.Error())
		}
	}

	if visibility := strings.TrimSpace(manifest.Spec.Repository.Visibility); visibility != "" {
		if _, err := resolveVisibility(visibility); err != nil {
			return invalidAnvilField("spec.repository.visibility", err.Error())
		}
	}

	if !manifest.Spec.CreateTerraformWorkspaces {
		return nil
	}

	if len(manifest.Spec.Environments) == 0 {
		return invalidAnvilField("spec.environments", "must include at least one environment when spec.createTerraformWorkspaces is true")
	}

	for environment, config := range manifest.Spec.Environments {
		if _, err := normalizeEnvironmentName(environment); err != nil {
			return invalidAnvilField("spec.environments."+environment, err.Error())
		}

		accountID := strings.TrimSpace(config.AWS.AccountID)
		if accountID == "" {
			accountID = strings.TrimSpace(config.AWS.AccountIDSnake)
		}
		if _, err := validateAWSAccountID(accountID); err != nil {
			return invalidAnvilField("spec.environments."+environment+".aws.accountId", "must be a 12-digit AWS account ID")
		}
	}

	return nil
}

func invalidAnvilField(field, reason string) error {
	return &anvilValidationError{field: field, reason: reason}
}
