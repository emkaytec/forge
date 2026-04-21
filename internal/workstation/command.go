package workstation

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/emkaytec/forge/internal/ui"
	"github.com/spf13/cobra"
)

type manager interface {
	List(ctx context.Context) ([]Workstation, []string, error)
	Start(ctx context.Context, name string) (Workstation, []string, error)
	Stop(ctx context.Context, name string) (Workstation, []string, error)
	Connect(ctx context.Context, name string, stdin io.Reader, stdout, stderr io.Writer) ([]string, error)
	ReloadConfig(ctx context.Context, name string, stdin io.Reader, stdout, stderr io.Writer) ([]string, error)
}

var newManager = func() manager {
	return NewManager()
}

// Command returns the configured workstation command group.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workstation",
		Short:   "Operate tagged AWS and GCP workstations",
		GroupID: GroupID,
	}

	cmd.AddCommand(
		newListCommand(),
		newStartCommand(),
		newStopCommand(),
		newConnectCommand(),
		newReloadConfigCommand(),
	)

	return cmd
}

func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tagged workstations across AWS and GCP",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := newManager()
			workstations, warnings, err := manager.List(cmd.Context())
			renderWarnings(cmd.ErrOrStderr(), warnings)
			if err != nil {
				return err
			}

			renderWorkstations(cmd.OutOrStdout(), workstations)
			return nil
		},
	}
}

func newStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start <name>",
		Short: "Start a stopped workstation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := newManager()
			workstation, warnings, err := manager.Start(cmd.Context(), args[0])
			renderWarnings(cmd.ErrOrStderr(), warnings)
			if err != nil {
				return err
			}

			if workstation.isRunning() {
				ui.Success(cmd.OutOrStdout(), fmt.Sprintf("Workstation %s is already running", workstation.Name))
				return nil
			}

			ui.Success(cmd.OutOrStdout(), fmt.Sprintf("Started workstation %s (%s)", workstation.Name, workstation.displayProvider()))
			return nil
		},
	}
}

func newStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running workstation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := newManager()
			workstation, warnings, err := manager.Stop(cmd.Context(), args[0])
			renderWarnings(cmd.ErrOrStderr(), warnings)
			if err != nil {
				return err
			}

			if workstation.isStopped() {
				ui.Success(cmd.OutOrStdout(), fmt.Sprintf("Workstation %s is already stopped", workstation.Name))
				return nil
			}

			ui.Success(cmd.OutOrStdout(), fmt.Sprintf("Stopped workstation %s (%s)", workstation.Name, workstation.displayProvider()))
			return nil
		},
	}
}

func newConnectCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <name>",
		Short: "Open an SSH session to a workstation over Tailscale",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := newManager()
			warnings, err := manager.Connect(cmd.Context(), args[0], os.Stdin, cmd.OutOrStdout(), cmd.ErrOrStderr())
			renderWarnings(cmd.ErrOrStderr(), warnings)
			return err
		},
	}
}

func newReloadConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "reload-config [name]",
		Short: "Re-run the workstation Ansible playbook for one host or all hosts",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			if len(args) == 1 {
				name = args[0]
			}

			manager := newManager()
			warnings, err := manager.ReloadConfig(cmd.Context(), name, os.Stdin, cmd.OutOrStdout(), cmd.ErrOrStderr())
			renderWarnings(cmd.ErrOrStderr(), warnings)
			return err
		},
	}
}

func renderWarnings(w io.Writer, warnings []string) {
	for _, warning := range warnings {
		ui.Warn(w, warning)
	}
}

func renderWorkstations(w io.Writer, workstations []Workstation) {
	fmt.Fprintln(w, ui.RenderHeading(w, "Workstations"))
	if len(workstations) == 0 {
		fmt.Fprintln(w, "  "+ui.RenderMuted(w, "no tagged workstations found"))
		return
	}

	nameWidth := len("Name")
	providerWidth := len("Provider")
	statusWidth := len("Status")
	hostWidth := len("Tailscale")
	for _, workstation := range workstations {
		if len(workstation.Name) > nameWidth {
			nameWidth = len(workstation.Name)
		}
		if len(workstation.displayProvider()) > providerWidth {
			providerWidth = len(workstation.displayProvider())
		}
		if len(workstation.displayStatus()) > statusWidth {
			statusWidth = len(workstation.displayStatus())
		}
		host := firstNonEmpty(workstation.TailscaleHostname, "-")
		if len(host) > hostWidth {
			hostWidth = len(host)
		}
	}

	fmt.Fprintf(
		w,
		"  %-*s  %-*s  %-*s  %-*s\n",
		nameWidth,
		"Name",
		providerWidth,
		"Provider",
		statusWidth,
		"Status",
		hostWidth,
		"Tailscale",
	)
	for _, workstation := range workstations {
		fmt.Fprintf(
			w,
			"  %-*s  %-*s  %-*s  %-*s\n",
			nameWidth,
			workstation.Name,
			providerWidth,
			workstation.displayProvider(),
			statusWidth,
			workstation.displayStatus(),
			hostWidth,
			firstNonEmpty(workstation.TailscaleHostname, "-"),
		)
	}
}
