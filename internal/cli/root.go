package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Timmyy3000/git-forest/internal/app"
	"github.com/Timmyy3000/git-forest/internal/buildinfo"
)

func Execute() error {
	root := newRootCommand(app.New())
	if err := root.Execute(); err != nil {
		if outputJSON {
			if writeErr := writeJSON(os.Stdout, errorPayload(err)); writeErr != nil {
				fmt.Fprintf(os.Stderr, "Error writing JSON error output: %s\n", writeErr)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
		return err
	}
	return nil
}

func newRootCommand(application *app.App) *cobra.Command {
	root := &cobra.Command{
		Use:           "forest",
		Short:         "Manage repo-local Git worktrees for parallel agent work",
		Long:          fmt.Sprintf("Forest keeps parallel worktrees visible under .forest/worktrees and tracks agent activity in .forest/state.\n\nAgent guide: %s", AgentGuideURL),
		Version:       buildinfo.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("forest {{.Version}}\n")
	root.PersistentFlags().BoolVar(&outputJSON, "json", false, "print machine-readable JSON")

	root.AddCommand(newInitCommand(application))
	root.AddCommand(newAddCommand(application))
	root.AddCommand(newListCommand(application))
	root.AddCommand(newStatusCommand(application))
	root.AddCommand(newMarkCommand(application))
	root.AddCommand(newPathCommand(application))
	root.AddCommand(newCloseCommand(application))
	root.AddCommand(newDoctorCommand(application))
	root.AddCommand(newAgentsCommand())
	root.AddCommand(newVersionCommand())

	root.SetErr(os.Stderr)
	root.SetOut(os.Stdout)
	return root
}

func printErr(format string, args ...any) error {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	return fmt.Errorf(format, args...)
}
