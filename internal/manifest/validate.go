package manifest

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/internal/ui"
	"github.com/emkaytec/forge/pkg/schema"
	"github.com/spf13/cobra"
)

func newValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate Forge manifest files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := reconcile.DiscoverManifests(args[0])
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

	_, err = schema.DecodeManifest(data)
	return err
}

func describeValidationError(err error) string {
	var validationErr *schema.ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Error()
	}

	var versionErr *schema.UnsupportedVersionError
	if errors.As(err, &versionErr) {
		return fmt.Sprintf("%s; use apiVersion %q", versionErr.Error(), schema.APIVersionV1)
	}

	var kindErr *schema.UnsupportedKindError
	if errors.As(err, &kindErr) {
		return fmt.Sprintf("%s; supported kinds are github-repo, hcp-tf-workspace, aws-iam-provisioner, and launch-agent", kindErr.Error())
	}

	if strings.Contains(err.Error(), "field ") {
		return err.Error() + "; remove unknown fields or rename them to a supported schema field"
	}

	return err.Error()
}
