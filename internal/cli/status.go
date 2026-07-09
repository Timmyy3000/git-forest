package cli

import (
	"github.com/spf13/cobra"

	"github.com/Timmyy3000/git-forest/internal/app"
	"github.com/Timmyy3000/git-forest/internal/output"
)

func newStatusCommand(application *app.App) *cobra.Command {
	var opts app.StatusOptions
	cmd := &cobra.Command{
		Use:   "status [name]",
		Short: "Show detailed Git health for managed worktrees",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.Name = args[0]
			}
			result, err := application.Status(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if outputJSON {
				return printJSON(cmd, result)
			}
			return output.RenderStatus(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.Agent, "agent", "", "filter by agent")
	cmd.Flags().StringVar(&opts.Phase, "phase", "", "filter by activity phase")
	cmd.Flags().BoolVar(&opts.Fast, "fast", false, "skip all git checks and show Forest metadata only")
	cmd.Flags().BoolVar(&opts.Diff, "diff", false, "print the staged and unstaged patch for one named worktree")
	return cmd
}
