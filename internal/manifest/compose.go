package manifest

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newComposeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "compose [blueprint] [name]",
		Short: "Compose a blueprint into several primitive manifests",
		Long: strings.TrimSpace(`Compose a higher-level blueprint into several primitive Forge manifests.

Blueprints are the authoring layer for stack-style workflows that share inputs
across several generated manifests. The generated output remains atomic so the
individual manifest files stay easy to review and reconcile.`),
		Example: strings.Join([]string{
			"  forge manifest compose terraform-github-repo forge",
			"  forge manifest compose terraform-github-repo",
		}, "\n"),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			blueprint := strings.TrimSpace(args[0])
			if blueprint == "" {
				return fmt.Errorf("blueprint name must not be empty")
			}

			if len(args) == 2 && strings.TrimSpace(args[1]) == "" {
				return fmt.Errorf("stack name must not be empty")
			}

			return fmt.Errorf("blueprint composition is not implemented yet: %s", blueprint)
		},
	}
}
