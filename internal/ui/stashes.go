package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mustafakarakulak/tuigy/internal/git"
)

// renderStashList draws the visible window of the stash list.
func renderStashList(stashes []git.Stash, cursor, offset, width, height int) string {
	if len(stashes) == 0 {
		return styleDim.Render("no stashes")
	}

	end := min(offset+height, len(stashes))
	lines := make([]string, 0, end-offset)

	for i := offset; i < end; i++ {
		s := stashes[i]
		age := shortAge(s.Date)

		// The index is what people say out loud ("pop the second one"), and it
		// is much narrower than the stash@{n} git wants.
		label := fmt.Sprintf("%d", i)
		fixed := 1 + len(label) + 1 + 1 + len(age)
		message := truncateRight(s.Message, width-fixed)

		pad := max(width-fixed-len([]rune(message)), 1)
		gap := strings.Repeat(" ", pad)

		if i == cursor {
			lines = append(lines, styleSelected.Width(width).Render(" "+label+" "+message+gap+age))
			continue
		}
		lines = append(lines, " "+styleHunk.Render(label)+" "+
			styleBase.Render(message)+gap+styleDim.Render(age))
	}

	return strings.Join(lines, "\n")
}

// renderStashDetail shows what a stash holds: where it came from, then its diff.
func renderStashDetail(s git.Stash, diff string, width int) string {
	field := func(label, value string) string {
		if value == "" {
			value = "—"
		}
		return styleDim.Render(padRight(label, 9)) + styleBase.Render(clipStyled(value, max(width-9, 8)))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		styleTitle.Render(clipStyled(s.Message, width)),
		"",
		field("ref", s.Ref),
		field("branch", s.Branch),
		field("when", humanTime(s.Date)),
		"",
		styleSection.Render("DIFF"),
		renderDiff(diff).content,
	)
}
