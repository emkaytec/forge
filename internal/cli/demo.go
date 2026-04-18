package cli

import (
	"time"

	"github.com/emkaytec/forge/internal/ui"
	"github.com/spf13/cobra"
)

func newDemoCommand() *cobra.Command {
	demo := &cobra.Command{
		Use:     "demo",
		Short:   "Exercise Forge UI primitives",
		GroupID: demoGroupID,
	}

	demo.AddCommand(newDemoBannerCommand(), newDemoSpinnerCommand())
	return demo
}

func newDemoBannerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "banner",
		Short: "Print the Forge brand banner",
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.Banner(cmd.OutOrStdout(), ui.Profile())
			return nil
		},
	}
}

func newDemoSpinnerCommand() *cobra.Command {
	duration := 2 * time.Second

	cmd := &cobra.Command{
		Use:   "spinner",
		Short: "Run the Forge spinner demo",
		RunE: func(cmd *cobra.Command, args []string) error {
			spinner := ui.NewSpinner("Forging the shell...")
			if err := spinner.Run(cmd.OutOrStdout(), duration); err != nil {
				return err
			}

			ui.Success(cmd.OutOrStdout(), "Spinner demo complete")
			return nil
		},
	}

	cmd.Flags().DurationVar(&duration, "duration", duration, "How long to show the spinner")
	_ = cmd.Flags().MarkHidden("duration")

	return cmd
}
