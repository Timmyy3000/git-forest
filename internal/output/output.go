package output

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"

	"github.com/Timmyy3000/git-forest/internal/app"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
)

func RenderList(w io.Writer, result app.ListResult) error {
	fmt.Fprintln(w, titleStyle.Render("🌲 Forest worktrees"))
	if terminalWidth(w) < 96 {
		for _, wt := range result.Worktrees {
			fmt.Fprintf(w, "%s  %s\n", wt.Name, styleIntegration(listIntegration(wt)))
			fmt.Fprintf(w, "  %s · %s · %s\n", dash(wt.Agent), dash(wt.Phase), age(wt.Updated))
		}
		return nil
	}
	return renderListTable(w, result.Worktrees)
}

func RenderRecursiveList(w io.Writer, result app.RecursiveListResult) error {
	return renderRecursiveList(w, result, terminalWidth(w))
}

func renderRecursiveList(w io.Writer, result app.RecursiveListResult, width int) error {
	fmt.Fprintln(w, titleStyle.Render("🌲 Forest worktrees"))
	if width >= 112 {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "REPOSITORY\tNAME\tAGENT\tPHASE\tINTEGRATION\tUPDATED")
		for _, repository := range result.Repositories {
			if repository.Error != "" {
				fmt.Fprintf(tw, "%s\t-\t-\t-\t%s\t-\n", repository.Path, repository.Error)
				continue
			}
			if len(repository.Worktrees) == 0 {
				fmt.Fprintf(tw, "%s\t(no managed worktrees)\t-\t-\t-\t-\n", repository.Path)
				continue
			}
			for _, wt := range repository.Worktrees {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", repository.Path, wt.Name, dash(wt.Agent), dash(wt.Phase), styleIntegration(listIntegration(wt)), age(wt.Updated))
			}
		}
		return tw.Flush()
	}
	for _, repository := range result.Repositories {
		fmt.Fprintln(w, repository.Path)
		if repository.Error != "" {
			fmt.Fprintf(w, "  %s\n", warnStyle.Render(repository.Error))
			continue
		}
		if len(repository.Worktrees) == 0 {
			fmt.Fprintln(w, "  no managed worktrees")
			continue
		}
		for _, wt := range repository.Worktrees {
			fmt.Fprintf(w, "  %s  %s\n", wt.Name, styleIntegration(listIntegration(wt)))
			fmt.Fprintf(w, "    %s · %s · %s\n", dash(wt.Agent), dash(wt.Phase), age(wt.Updated))
		}
	}
	return nil
}

func renderListTable(w io.Writer, worktrees []app.WorktreeView) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tAGENT\tPHASE\tINTEGRATION\tUPDATED")
	for _, wt := range worktrees {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", wt.Name, dash(wt.Agent), dash(wt.Phase), styleIntegration(listIntegration(wt)), age(wt.Updated))
	}
	return tw.Flush()
}

func listIntegration(wt app.WorktreeView) string {
	if wt.ChecksSkipped {
		return "-"
	}
	return withIntegrationError(wt)
}

func terminalWidth(w io.Writer) int {
	file, ok := w.(*os.File)
	if !ok || !term.IsTerminal(file.Fd()) {
		return 0
	}
	width, _, err := term.GetSize(file.Fd())
	if err != nil {
		return 0
	}
	return width
}

func RenderStatus(w io.Writer, result app.ListResult) error {
	fmt.Fprintln(w, titleStyle.Render("Forest Git health"))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tGIT\tINTEGRATION\tUPDATED\tNOTE")
	for _, wt := range result.Worktrees {
		integration := withIntegrationError(wt)
		if wt.ChecksIncomplete {
			integration += " (partial)"
		}
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
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", wt.Name, gitState, styleIntegration(integration), age(wt.Updated), dash(note))
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
