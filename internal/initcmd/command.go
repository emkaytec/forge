package initcmd

import (
	"fmt"

	"github.com/emkaytec/forge/internal/ui"
	"github.com/spf13/cobra"
)

var newOIDCManager = func() oidcManager {
	return newAWSOIDCManager()
}

// Command returns the configured init command group.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap one-time cloud integration setup",
	}

	cmd.AddCommand(newAWSOIDCCommand())
	cmd.AddCommand(newAWSStackSetsCommand())

	return cmd
}

func newAWSOIDCCommand() *cobra.Command {
	var (
		accountProfiles []string
		accountIDs      []string
	)

	cmd := &cobra.Command{
		Use:   "aws-oidc",
		Short: "Create the shared AWS OIDC providers Forge relies on",
		Long: `Create the AWS IAM OIDC providers for GitHub Actions and HCP Terraform.

This command is intentionally narrow: it bootstraps the shared providers once per
AWS account, but it does not create IAM roles. Those remain managed through
AWSIAMProvisioner manifests.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			targets, err := resolveAWSAccountTargets(cmd.InOrStdin(), cmd.OutOrStdout(), accountProfiles, accountIDs)
			if err != nil {
				ui.Error(cmd.ErrOrStderr(), err.Error())
				return err
			}

			manager := newOIDCManager()
			for _, target := range targets {
				resolved, err := manager.ResolveAccount(cmd.Context(), target)
				if err != nil {
					ui.Error(cmd.ErrOrStderr(), err.Error())
					return err
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Target AWS account: %s\n", resolved.display())

				var results []providerResult
				spinner := ui.NewSpinner(fmt.Sprintf("Configuring AWS OIDC providers for %s...", resolved.display()))
				err = spinner.RunWhile(cmd.OutOrStdout(), func() error {
					providerResults, providerErr := manager.EnsureProviders(cmd.Context(), resolved)
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

				if len(targets) > 1 {
					fmt.Fprintln(cmd.OutOrStdout())
				}
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&accountProfiles, "account-profile", nil, "AWS shared-config profile to configure (repeat or comma-separate)")
	cmd.Flags().StringSliceVar(&accountIDs, "account-id", nil, "AWS account ID to configure using the current AWS session or organization access role (repeat or comma-separate)")

	return cmd
}
