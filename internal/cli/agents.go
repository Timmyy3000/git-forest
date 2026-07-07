package cli

import (
	// Required by go:embed.
	_ "embed"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const AgentGuideURL = "https://forest.timi.click/agents.md"

//go:embed agents_guide.md
var agentGuide string

func newAgentsCommand() *cobra.Command {
	var urlOnly bool
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Print Forest instructions for coding agents",
		Long:  "Print embedded Forest instructions for coding agents. Use --url to print only the canonical online guide URL.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputJSON {
				value := map[string]string{"url": AgentGuideURL}
				if !urlOnly {
					value["guide"] = agentGuide
				}
				return printJSON(cmd, value)
			}
			if urlOnly {
				fmt.Fprintln(cmd.OutOrStdout(), AgentGuideURL)
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), agentGuide)
			if !strings.HasSuffix(agentGuide, "\n") {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&urlOnly, "url", false, "print only the canonical online guide URL")
	return cmd
}
