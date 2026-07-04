package cli

import (
	"github.com/spf13/cobra"

	"github.com/Timmyy3000/git-forest/internal/app"
	"github.com/Timmyy3000/git-forest/internal/output"
)

func newStatusCommand(application *app.App) *cobra.Command {
	var opts app.ListOptions
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Group worktrees by what needs attention",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := application.List(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.RenderStatus(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.Agent, "agent", "", "filter by agent")
	cmd.Flags().StringVar(&opts.Phase, "phase", "", "filter by activity phase")
	return cmd
}
