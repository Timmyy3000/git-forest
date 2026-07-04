package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oluwatimilehin/git-forest/internal/app"
)

func newInitCommand(application *app.App) *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Forest in the current repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := application.Init(cmd.Context())
			if err != nil {
				return err
			}
			if quiet {
				fmt.Fprintln(cmd.OutOrStdout(), result.ForestDir)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Forest initialized at %s\n", result.ForestDir)
			return nil
		},
	}
	cmd.Flags().BoolVar(&quiet, "quiet", false, "print only the Forest directory path")
	return cmd
}
