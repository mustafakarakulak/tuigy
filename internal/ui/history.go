package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/mustafakarakulak/tuigy/internal/git"
)

// renderCommitList draws the visible window of the history.
//
// Picked commits carry a marker so a cherry-pick selection stays visible while
// scrolling through hundreds of commits.
func renderCommitList(commits []git.Commit, cursor, offset int, picked map[string]bool, width, height int) string {
	if len(commits) == 0 {
		return styleDim.Render("no commits")
	}

	end := min(offset+height, len(commits))
	lines := make([]string, 0, end-offset)

	for i := offset; i < end; i++ {
		lines = append(lines, renderCommitRow(commits[i], i == cursor, picked[commits[i].Hash], width))
	}
	return strings.Join(lines, "\n")
}

func renderCommitRow(c git.Commit, selected, isPicked bool, width int) string {
	marker := " "
	if isPicked {
		marker = "✓"
	}

	age := shortAge(c.Date)
	// Lay out the fixed parts first, then give the subject whatever is left.
	fixed := 1 + 1 + 1 + len(c.Short) + 1 + 1 + len(age)
	subject := truncateRight(c.Subject, width-fixed)

	pad := width - fixed - len([]rune(subject))
	if pad < 1 {
		pad = 1
	}
	gap := strings.Repeat(" ", pad)

	if selected {
		return styleSelected.Width(width).Render(
			" " + marker + " " + c.Short + " " + subject + gap + age)
	}

	subjectStyle := styleBase
	if c.IsMerge() {
		subjectStyle = styleDim
	}

	return " " + styleAdded.Render(marker) + " " +
		styleHunk.Render(c.Short) + " " +
		subjectStyle.Render(subject) + gap + styleDim.Render(age)
}

// renderCommitDetail shows everything about one commit in a single scrollable
// pane: metadata, the full message, the files it touched, then the diff.
func renderCommitDetail(d *git.CommitDetail, width int) string {
	if d == nil {
		return styleDim.Render("loading…")
	}

	field := func(label, value string) string {
		return styleDim.Render(padRight(label, 9)) + styleBase.Render(clipStyled(value, max(width-9, 8)))
	}

	sections := []string{
		styleHunk.Render(d.Commit.Short) + "  " + styleTitle.Render(clipStyled(d.Commit.Subject, max(width-10, 8))),
		"",
		field("author", d.Commit.Author),
		field("when", humanTime(d.Commit.Date)),
	}
	if d.Commit.Refs != "" {
		sections = append(sections, field("refs", d.Commit.Refs))
	}
	if d.Commit.IsMerge() {
		sections = append(sections, field("merge", fmt.Sprintf("%d parents", len(d.Commit.Parents))))
	}

	// The body repeats the subject on its first line; show only what is extra.
	if body := strings.TrimSpace(strings.TrimPrefix(d.Body, d.Commit.Subject)); body != "" {
		sections = append(sections, "", styleBase.Render(body))
	}

	sections = append(sections, "", styleSection.Render(fmt.Sprintf("FILES (%d)", len(d.Files))))
	for _, f := range d.Files {
		name := f.Path
		if f.OrigPath != "" {
			name = f.OrigPath + " → " + f.Path
		}
		letter := f.Status.String()
		sections = append(sections,
			" "+letterStyle(letter).Render(letter)+" "+styleBase.Render(truncateLeft(name, max(width-3, 8))))
	}

	sections = append(sections, "", styleSection.Render("DIFF"), renderDiff(d.Diff, noHunk).content)
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// truncateRight shortens text from the end, keeping the beginning readable.
// Commit subjects lead with what matters, unlike paths.
func truncateRight(s string, width int) string {
	if width <= 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	return string(r[:width-1]) + "…"
}

// shortAge is a compact relative age for a dense list, where humanTime's
// "3 days ago" would cost more columns than the subject can spare.
func shortAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", max(int(d.Minutes()), 0))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dy", int(d.Hours()/24/365))
	}
}
