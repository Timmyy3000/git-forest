package output

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/Timmyy3000/git-forest/internal/app"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
)

func RenderList(w io.Writer, result app.ListResult) error {
	fmt.Fprintln(w, titleStyle.Render("🌲 Forest worktrees"))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tAGENT\tPHASE\tGIT\tINTEGRATION\tUPDATED\tNEXT")
	for _, wt := range result.Worktrees {
		gitState := "clean"
		integration := styleIntegration(wt.Integration)
		next := wt.Next
		if wt.ChecksSkipped {
			gitState = "-"
			integration = "-"
			next = "-"
		} else {
			if wt.Dirty {
				gitState = "dirty"
			}
			if wt.Ahead > 0 || wt.Behind > 0 {
				gitState = fmt.Sprintf("%s +%d -%d", gitState, wt.Ahead, wt.Behind)
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", wt.Name, dash(wt.Agent), dash(wt.Phase), gitState, integration, age(wt.Updated), next)
	}
	return tw.Flush()
}

func RenderStatus(w io.Writer, result app.ListResult) error {
	groups := map[string][]app.WorktreeView{}
	checksSkipped := false
	for _, wt := range result.Worktrees {
		if wt.ChecksSkipped {
			checksSkipped = true
		}
		group := "Active"
		switch {
		case wt.Phase == "blocked":
			group = "Blocked"
		case wt.ChecksSkipped:
			// Dirty and Next were not computed, so the git-based groups
			// (Ready for review / Ready to close) cannot be determined.
			if wt.Phase == "" {
				group = "Unknown activity"
			}
		case wt.Next == "close":
			group = "Ready to close"
		case wt.Dirty:
			group = "Ready for review"
		case wt.Phase == "":
			group = "Unknown activity"
		}
		groups[group] = append(groups[group], wt)
	}
	for _, name := range []string{"Active", "Blocked", "Ready for review", "Ready to close", "Unknown activity"} {
		items := groups[name]
		if len(items) == 0 {
			continue
		}
		fmt.Fprintln(w, titleStyle.Render(name))
		for _, wt := range items {
			fmt.Fprintf(w, "  %s  %s  %s  %s\n", wt.Name, dash(wt.Agent), dash(wt.Phase), wt.Note)
		}
	}
	if checksSkipped {
		fmt.Fprintln(w, warnStyle.Render("git checks skipped (--fast): review/close grouping unavailable"))
	}
	return nil
}

func dash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func styleIntegration(value string) string {
	if value == "merged" || value == "patch-equivalent" {
		return okStyle.Render(value)
	}
	return warnStyle.Render(value)
}

func age(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t).Round(time.Minute)
	if d < time.Minute {
		return "now"
	}
	return d.String() + " ago"
}
