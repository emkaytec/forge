package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
			paths, err := manifestPaths(args[0])
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

func manifestPaths(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		if !isManifestFile(path) {
			return nil, fmt.Errorf("%s is not a .yaml or .yml file", path)
		}

		return []string{path}, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !isManifestFile(name) {
			continue
		}

		paths = append(paths, filepath.Join(path, name))
	}

	sort.Strings(paths)

	if len(paths) == 0 {
		return nil, fmt.Errorf("%s does not contain any .yaml or .yml manifest files", path)
	}

	return paths, nil
}

func validateManifestFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	_, err = schema.DecodeManifest(data)
	return err
}

func isManifestFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
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
