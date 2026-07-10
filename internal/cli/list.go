package cli

import (
	"github.com/spf13/cobra"

	"github.com/Timmyy3000/git-forest/internal/app"
	"github.com/Timmyy3000/git-forest/internal/output"
)

func newListCommand(application *app.App) *cobra.Command {
	var opts app.ListOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show the Forest worktree dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Recursive {
				result, err := application.ListRecursive(cmd.Context(), opts)
				if err != nil {
					return err
				}
				if outputJSON {
					return printJSON(cmd, result)
				}
				return output.RenderRecursiveList(cmd.OutOrStdout(), result)
			}
			result, err := application.List(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if outputJSON {
				return printJSON(cmd, result)
			}
			return output.RenderList(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.Agent, "agent", "", "filter by agent")
	cmd.Flags().StringVar(&opts.Phase, "phase", "", "filter by activity phase")
	cmd.Flags().BoolVarP(&opts.Recursive, "recursive", "r", false, "list Forest repositories below the current directory")
	cmd.Flags().BoolVar(&opts.Fast, "fast", false, "skip all git checks and show Forest metadata only")
	return cmd
}
