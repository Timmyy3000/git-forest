package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Timmyy3000/git-forest/internal/app"
)

func newDoctorCommand(application *app.App) *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose Forest setup and state",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := application.Doctor(cmd.Context(), fix)
			if err != nil {
				return err
			}
			for _, check := range result.Checks {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", check.Name, check.Status)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "apply safe fixes")
	return cmd
}
