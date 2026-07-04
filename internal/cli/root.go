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
	return root.Execute()
}

func newRootCommand(application *app.App) *cobra.Command {
	root := &cobra.Command{
		Use:     "forest",
		Short:   "Manage repo-local Git worktrees for parallel agent work",
		Long:    "Forest keeps parallel worktrees visible under .forest/worktrees and tracks agent activity in .forest/state.",
		Version: buildinfo.Version,
	}
	root.SetVersionTemplate("forest {{.Version}}\n")

	root.AddCommand(newInitCommand(application))
	root.AddCommand(newAddCommand(application))
	root.AddCommand(newListCommand(application))
	root.AddCommand(newStatusCommand(application))
	root.AddCommand(newMarkCommand(application))
	root.AddCommand(newPathCommand(application))
	root.AddCommand(newCloseCommand(application))
	root.AddCommand(newDoctorCommand(application))
	root.AddCommand(newVersionCommand())

	root.SetErr(os.Stderr)
	root.SetOut(os.Stdout)
	return root
}

func printErr(format string, args ...any) error {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	return fmt.Errorf(format, args...)
}
