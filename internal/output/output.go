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
	fmt.Fprintln(tw, "NAME\tAGENT\tPHASE\tINTEGRATION\tUPDATED")
	for _, wt := range result.Worktrees {
		integration := styleIntegration(withIntegrationError(wt))
		if wt.ChecksSkipped {
			integration = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", wt.Name, dash(wt.Agent), dash(wt.Phase), integration, age(wt.Updated))
	}
	return tw.Flush()
}

func RenderStatus(w io.Writer, result app.ListResult) error {
	fmt.Fprintln(w, titleStyle.Render("Forest Git health"))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tGIT\tINTEGRATION\tUPDATED\tNOTE")
	for _, wt := range result.Worktrees {
		gitState := "-"
		if wt.ChecksIncomplete {
			gitState = "unknown"
		} else if !wt.ChecksSkipped {
			gitState = "clean"
			if wt.Dirty {
				gitState = "dirty"
			}
			if !wt.DetailsSkipped && (wt.Ahead > 0 || wt.Behind > 0) {
				gitState = fmt.Sprintf("%s +%d -%d", gitState, wt.Ahead, wt.Behind)
			}
		}
		note := wt.Note
		if wt.CheckError != "" {
			if note != "" {
				note += "; "
			}
			note += wt.CheckError
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", wt.Name, gitState, styleIntegration(withIntegrationError(wt)), age(wt.Updated), dash(note))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if result.Diff != "" {
		fmt.Fprintln(w, "\nDiff")
		fmt.Fprintln(w, result.Diff)
	} else if result.DiffRequested {
		fmt.Fprintln(w, "\nNo tracked changes to diff. Untracked files are reported as dirty but have no Git patch.")
	}
	return nil
}

func withIntegrationError(wt app.WorktreeView) string {
	if wt.IntegrationError == "" {
		return wt.Integration
	}
	return wt.Integration + ": " + wt.IntegrationError
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
