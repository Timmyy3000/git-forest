package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Timmyy3000/git-forest/internal/buildinfo"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Forest version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputJSON {
				return printJSON(cmd, map[string]string{"version": buildinfo.Version, "commit": buildinfo.Commit, "date": buildinfo.Date})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "forest %s\ncommit %s\nbuilt %s\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
			return nil
		},
	}
}
