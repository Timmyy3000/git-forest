package cli

import (
	"github.com/spf13/cobra"

	"github.com/oluwatimilehin/git-forest/internal/app"
	"github.com/oluwatimilehin/git-forest/internal/output"
)

func newListCommand(application *app.App) *cobra.Command {
	var opts app.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show the Forest worktree dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := application.List(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.RenderList(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.Agent, "agent", "", "filter by agent")
	cmd.Flags().StringVar(&opts.Phase, "phase", "", "filter by activity phase")
	cmd.Flags().BoolVar(&opts.Verbose, "verbose", false, "show extra details")
	return cmd
}
