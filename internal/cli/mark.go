package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oluwatimilehin/git-forest/internal/app"
)

func newMarkCommand(application *app.App) *cobra.Command {
	var opts app.MarkOptions
	cmd := &cobra.Command{
		Use:   "mark [name]",
		Short: "Update agent activity for a worktree",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("expected at most one name")
			}
			if opts.Phase == "" {
				return fmt.Errorf("--phase is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.Name = args[0]
			}
			result, err := application.Mark(cmd.Context(), opts)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Marked %s as %s\n", result.Name, result.Phase)
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Phase, "phase", "", "activity phase")
	cmd.Flags().StringVar(&opts.Agent, "agent", "", "agent name")
	cmd.Flags().StringVar(&opts.Note, "note", "", "activity note")
	return cmd
}
