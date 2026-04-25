package manifest

import "github.com/spf13/cobra"

// Command returns the configured manifest command group.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Generate and validate Anvil manifests",
	}

	cmd.AddCommand(newGenerateCommand())
	cmd.AddCommand(newValidateCommand())

	return cmd
}
