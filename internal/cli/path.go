package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Timmyy3000/git-forest/internal/app"
)

func newPathCommand(application *app.App) *cobra.Command {
	var current bool
	cmd := &cobra.Command{
		Use:   "path [name]",
		Short: "Print a managed worktree path",
		Args: func(cmd *cobra.Command, args []string) error {
			if current {
				return nil
			}
			if len(args) != 1 {
				return fmt.Errorf("provide a worktree name or --current")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			path, err := application.Path(cmd.Context(), name, current)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&current, "current", false, "resolve the current worktree")
	return cmd
}
