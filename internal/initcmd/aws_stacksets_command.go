package initcmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/emkaytec/forge/internal/ui"
	"github.com/spf13/cobra"
)

func newAWSStackSetsCommand() *cobra.Command {
	var (
		accountProfiles          []string
		accountIDs               []string
		administrationRoleName   string
		executionRoleName        string
		executionManagedPolicies []string
	)

	cmd := &cobra.Command{
		Use:   "aws-stacksets",
		Short: "Create AWS StackSet roles for Anvil Terraform prerequisites",
		Long: strings.TrimSpace(`Create the self-managed CloudFormation StackSet IAM roles used by Anvil's Terraform module.

Run this command while authenticated to the AWS management account that will
own the StackSets. Forge creates or updates the StackSet administration role in
that current account, then creates or updates the StackSet execution role in
each selected target account.`),
		Example: strings.Join([]string{
			"  forge init aws-stacksets",
			"  forge init aws-stacksets --account-profile dev-admin --account-profile prod-admin",
			"  forge init aws-stacksets --account-id 111111111111 --account-id 222222222222",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			targets, err := resolveAWSAccountTargets(cmd.InOrStdin(), cmd.OutOrStdout(), accountProfiles, accountIDs)
			if err != nil {
				ui.Error(cmd.ErrOrStderr(), err.Error())
				return err
			}

			manager := newStackSetManager()
			managementAccountID, err := manager.ResolveManagementAccount(cmd.Context())
			if err != nil {
				ui.Error(cmd.ErrOrStderr(), err.Error())
				return err
			}

			resolvedTargets := make([]awsAccountTarget, 0, len(targets))
			targetAccountIDs := make([]string, 0, len(targets))
			for _, target := range targets {
				resolved, err := manager.ResolveAccount(cmd.Context(), target)
				if err != nil {
					ui.Error(cmd.ErrOrStderr(), err.Error())
					return err
				}
				resolvedTargets = append(resolvedTargets, resolved)
				targetAccountIDs = append(targetAccountIDs, resolved.AccountID)
			}

			setup := stackSetSetup{
				ManagementAccountID:      managementAccountID,
				TargetAccountIDs:         targetAccountIDs,
				AdministrationRoleName:   administrationRoleName,
				ExecutionRoleName:        executionRoleName,
				ExecutionManagedPolicies: executionManagedPolicies,
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Management AWS account: %s\n", managementAccountID)
			fmt.Fprintln(cmd.OutOrStdout(), "Target AWS accounts:")
			for _, target := range resolvedTargets {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", target.display())
			}

			var administrationResult stackSetRoleResult
			spinner := ui.NewSpinner("Configuring StackSet administration role...")
			err = spinner.RunWhile(cmd.OutOrStdout(), func() error {
				result, roleErr := manager.EnsureAdministrationRole(cmd.Context(), setup)
				administrationResult = result
				return roleErr
			})
			if err != nil {
				ui.Error(cmd.ErrOrStderr(), err.Error())
				return err
			}
			printStackSetRoleResult(cmd.OutOrStdout(), "StackSet administration role", administrationResult)

			for _, target := range resolvedTargets {
				var executionResult stackSetRoleResult
				spinner := ui.NewSpinner(fmt.Sprintf("Configuring StackSet execution role for %s...", target.display()))
				err = spinner.RunWhile(cmd.OutOrStdout(), func() error {
					result, roleErr := manager.EnsureExecutionRole(cmd.Context(), setup, target)
					executionResult = result
					return roleErr
				})
				if err != nil {
					ui.Error(cmd.ErrOrStderr(), err.Error())
					return err
				}
				printStackSetRoleResult(cmd.OutOrStdout(), fmt.Sprintf("StackSet execution role in %s", target.display()), executionResult)
			}

			administrationRoleARN := administrationResult.ARN
			if strings.TrimSpace(administrationRoleARN) == "" {
				administrationRoleARN = stackSetAdministrationRoleARN(managementAccountID, administrationRoleName)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Anvil Terraform variables:")
			fmt.Fprintf(cmd.OutOrStdout(), "  stack_set_administration_role_arn = %q\n", administrationRoleARN)
			fmt.Fprintf(cmd.OutOrStdout(), "  stack_set_execution_role_name     = %q\n", executionRoleName)

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&accountProfiles, "account-profile", nil, "Target AWS shared-config profile to configure (repeat or comma-separate)")
	cmd.Flags().StringSliceVar(&accountIDs, "account-id", nil, "Target AWS account ID to configure using the current AWS session or organization access role (repeat or comma-separate)")
	cmd.Flags().StringVar(&administrationRoleName, "administration-role-name", defaultStackSetAdministrationRoleName, "StackSet administration role name to create in the current account")
	cmd.Flags().StringVar(&executionRoleName, "execution-role-name", defaultStackSetExecutionRoleName, "StackSet execution role name to create in each target account")
	cmd.Flags().StringSliceVar(&executionManagedPolicies, "execution-policy-arn", []string{defaultStackSetExecutionPolicyARN}, "Managed policy ARN to attach to each execution role (repeat or comma-separate)")

	return cmd
}

func printStackSetRoleResult(w io.Writer, label string, result stackSetRoleResult) {
	switch {
	case result.Created:
		ui.Success(w, fmt.Sprintf("Created %s (%s)", label, result.Name))
	case result.UpdatedTrustPolicy || result.UpdatedInlinePolicy || len(result.AttachedPolicyARNs) > 0:
		ui.Success(w, fmt.Sprintf("Updated %s (%s)", label, result.Name))
	default:
		ui.Success(w, fmt.Sprintf("%s already configured (%s)", label, result.Name))
	}
}
