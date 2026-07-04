package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oluwatimilehin/git-forest/internal/app"
)

func newAddCommand(application *app.App) *cobra.Command {
	var opts app.AddOptions
	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Create a repo-local worktree",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("expected at most one name")
			}
			if len(args) == 0 && opts.Branch == "" {
				return fmt.Errorf("provide a name or --branch")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.Name = args[0]
			}
			result, err := application.Add(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if opts.Quiet {
				fmt.Fprintln(cmd.OutOrStdout(), result.Path)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n  branch: %s\n  path: %s\n", result.Name, result.Branch, result.Path)
			for _, copied := range result.Copied {
				fmt.Fprintf(cmd.OutOrStdout(), "  copied: %s\n", copied)
			}
			for _, warning := range result.Warnings {
				fmt.Fprintf(cmd.OutOrStdout(), "  warning: %s\n", warning)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&opts.Branch, "branch", "b", "", "branch to use; also becomes the worktree identity")
	cmd.Flags().StringVar(&opts.From, "from", "", "base ref for a new branch")
	cmd.Flags().StringVar(&opts.Agent, "agent", "", "agent creating the worktree")
	cmd.Flags().BoolVar(&opts.Fetch, "fetch", false, "fetch remotes before resolving the branch")
	cmd.Flags().BoolVar(&opts.Quiet, "quiet", false, "print only the worktree path")
	return cmd
}
