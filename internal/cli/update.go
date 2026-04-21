package cli

import (
	"context"
	"fmt"

	"github.com/emkaytec/forge/internal/ui"
	selfupdate "github.com/emkaytec/forge/internal/update"
	"github.com/spf13/cobra"
)

type updateRunner interface {
	Run(ctx context.Context, opts selfupdate.Options) (selfupdate.Result, error)
}

var newUpdateRunner = func(version string) updateRunner {
	return selfupdate.New(selfupdate.Config{CurrentVersion: version})
}

func newUpdateCommand(version string) *cobra.Command {
	var checkOnly bool
	var requestedVersion string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and install released Forge binaries",
		RunE: func(cmd *cobra.Command, args []string) error {
			message := "Checking for Forge updates..."
			if !checkOnly {
				message = "Installing Forge update..."
			}

			var result selfupdate.Result
			spinner := ui.NewSpinner(message)
			err := spinner.RunWhile(cmd.OutOrStdout(), func() error {
				r, runErr := runUpdateCheck(cmd.Context(), version, selfupdate.Options{
					Check:   checkOnly,
					Version: requestedVersion,
				})
				result = r
				return runErr
			})
			if err != nil {
				ui.Error(cmd.ErrOrStderr(), err.Error())
				return err
			}

			switch {
			case result.UpToDate && checkOnly:
				ui.Success(cmd.OutOrStdout(), fmt.Sprintf("Forge is up to date at %s", result.TargetVersion))
			case result.UpToDate:
				ui.Success(cmd.OutOrStdout(), fmt.Sprintf("Forge is already up to date at %s", result.TargetVersion))
			case checkOnly:
				ui.Warn(cmd.OutOrStdout(), fmt.Sprintf("Update available: %s -> %s", displayVersion(result.CurrentVersion), result.TargetVersion))
			case result.Updated:
				ui.Success(cmd.OutOrStdout(), fmt.Sprintf("Updated Forge from %s to %s", displayVersion(result.CurrentVersion), result.TargetVersion))
			default:
				ui.Warn(cmd.OutOrStdout(), fmt.Sprintf("No update applied for %s", result.TargetVersion))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "Check whether a newer Forge release is available")
	cmd.Flags().StringVar(&requestedVersion, "version", "", "Install a specific Forge release tag")

	return cmd
}

func displayVersion(version string) string {
	if version == "" {
		return "unknown"
	}

	return version
}

func runUpdateCheck(ctx context.Context, version string, opts selfupdate.Options) (selfupdate.Result, error) {
	runner := newUpdateRunner(version)
	return runner.Run(ctx, opts)
}
