package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oluwatimilehin/git-forest/internal/app"
)

func newCloseCommand(application *app.App) *cobra.Command {
	var opts app.CloseOptions
	cmd := &cobra.Command{
		Use:   "close [name]",
		Short: "Safely remove a managed worktree",
		Args: func(cmd *cobra.Command, args []string) error {
			if opts.Merged {
				return nil
			}
			if len(args) != 1 {
				return fmt.Errorf("provide a worktree name or --merged")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.Name = args[0]
			}
			result, err := application.Close(cmd.Context(), opts)
			if err != nil {
				return err
			}
			for _, closed := range result.Closed {
				fmt.Fprintf(cmd.OutOrStdout(), "Closed %s\n", closed)
			}
			for _, skipped := range result.Skipped {
				fmt.Fprintf(cmd.OutOrStdout(), "Skipped %s: %s\n", skipped.Name, skipped.Reason)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.Merged, "merged", false, "close all safely integrated worktrees")
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "execute without confirmation")
	cmd.Flags().BoolVar(&opts.IncludeDirty, "include-dirty", false, "allow closing dirty worktrees")
	cmd.Flags().BoolVar(&opts.IncludeUnmerged, "include-unmerged", false, "allow closing unmerged worktrees")
	cmd.Flags().BoolVar(&opts.DeleteBranch, "delete-branch", false, "delete the branch after removing the worktree")
	return cmd
}
