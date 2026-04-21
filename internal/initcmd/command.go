package initcmd

import (
	"fmt"

	"github.com/emkaytec/forge/internal/ui"
	"github.com/spf13/cobra"
)

// GroupID is the cobra group that hosts init subcommands in help output.
const GroupID = "init"

var newManager = func() oidcManager {
	return newAWSOIDCManager()
}

// Command returns the configured init command group.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Bootstrap one-time cloud integration setup",
		GroupID: GroupID,
	}

	cmd.AddCommand(newAWSOIDCCommand())

	return cmd
}

func newAWSOIDCCommand() *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "aws-oidc",
		Short: "Create the shared AWS OIDC providers Forge relies on",
		Long: `Create the AWS IAM OIDC providers for GitHub Actions and HCP Terraform.

This command is intentionally narrow: it bootstraps the shared providers once per
AWS account, but it does not create IAM roles. Those remain managed through
AWSIAMProvisioner manifests.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := newManager()
			resolvedAccountID, err := manager.ResolveAccountID(cmd.Context(), accountID)
			if err != nil {
				ui.Error(cmd.ErrOrStderr(), err.Error())
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Target AWS account: %s\n", resolvedAccountID)

			var results []providerResult
			spinner := ui.NewSpinner("Configuring AWS OIDC providers...")
			err = spinner.RunWhile(cmd.OutOrStdout(), func() error {
				providerResults, providerErr := manager.EnsureProviders(cmd.Context(), resolvedAccountID)
				results = providerResults
				return providerErr
			})
			if err != nil {
				ui.Error(cmd.ErrOrStderr(), err.Error())
				return err
			}

			for _, result := range results {
				if result.Created {
					ui.Success(cmd.OutOrStdout(), fmt.Sprintf("Created %s OIDC provider (%s)", result.Provider.Label, result.Provider.Issuer))
					continue
				}

				ui.Success(cmd.OutOrStdout(), fmt.Sprintf("%s OIDC provider already exists (%s)", result.Provider.Label, result.Provider.Issuer))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&accountID, "account-id", "", "Target AWS account ID; defaults to the current AWS session account")

	return cmd
}
