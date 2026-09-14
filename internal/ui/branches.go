package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/mustafakarakulak/tuigy/internal/git"
)

type branchSection int

const (
	bsLocal branchSection = iota
	bsRemote
)

func (s branchSection) title() string {
	if s == bsLocal {
		return "LOCAL"
	}
	return "REMOTE"
}

// branchRow is one line of the branch list: a section header or a branch.
type branchRow struct {
	header bool
	sec    branchSection
	count  int
	branch git.Branch
}

func buildBranchRows(branches []git.Branch, filter string) []branchRow {
	var local, remote []git.Branch
	for _, b := range branches {
		if !matchesFilter(filter, b.Name) {
			continue
		}
		if b.Remote {
			remote = append(remote, b)
		} else {
			local = append(local, b)
		}
	}

	var rows []branchRow
	add := func(sec branchSection, list []git.Branch) {
		if len(list) == 0 {
			return
		}
		rows = append(rows, branchRow{header: true, sec: sec, count: len(list)})
		for _, b := range list {
			rows = append(rows, branchRow{sec: sec, branch: b})
		}
	}

	add(bsLocal, local)
	add(bsRemote, remote)
	return rows
}

// renderBranchList draws the visible window of the branch list.
func renderBranchList(rows []branchRow, cursor, offset, width, height int) string {
	if len(rows) == 0 {
		return styleDim.Render("no branches")
	}

	end := min(offset+height, len(rows))
	lines := make([]string, 0, end-offset)

	for i := offset; i < end; i++ {
		r := rows[i]
		if r.header {
			lines = append(lines, styleSection.Render(fmt.Sprintf("%s (%d)", r.sec.title(), r.count)))
			continue
		}
		lines = append(lines, renderBranchRow(r.branch, i == cursor, width))
	}

	return strings.Join(lines, "\n")
}

func renderBranchRow(b git.Branch, selected bool, width int) string {
	marker := " "
	if b.Current {
		marker = "●"
	}

	// The tracking markers are placed first so the name can be truncated to
	// whatever room is left, rather than pushing them off the row.
	track := trackSummary(b)
	nameWidth := width - 3 - lipgloss.Width(track)
	if track != "" {
		nameWidth--
	}
	name := truncateLeft(b.Name, nameWidth)

	pad := width - 3 - lipgloss.Width(name) - lipgloss.Width(track)
	if pad < 1 {
		pad = 1
	}

	if selected {
		return styleSelected.Width(width).Render(" " + marker + " " + name + strings.Repeat(" ", pad) + track)
	}

	nameStyle := styleBase
	if b.Current {
		nameStyle = styleBranch
	} else if b.Remote {
		nameStyle = styleDim
	}

	return " " + styleAhead.Render(marker) + " " + nameStyle.Render(name) +
		strings.Repeat(" ", pad) + trackStyled(b)
}

// trackSummary is the plain-text ahead/behind marker, used for width maths.
func trackSummary(b git.Branch) string {
	switch {
	case b.Gone:
		return "gone"
	case b.Ahead > 0 && b.Behind > 0:
		return fmt.Sprintf("↑%d ↓%d", b.Ahead, b.Behind)
	case b.Ahead > 0:
		return fmt.Sprintf("↑%d", b.Ahead)
	case b.Behind > 0:
		return fmt.Sprintf("↓%d", b.Behind)
	default:
		return ""
	}
}

func trackStyled(b git.Branch) string {
	switch {
	case b.Gone:
		return styleBehind.Render("gone")
	case b.Ahead > 0 && b.Behind > 0:
		return styleAhead.Render(fmt.Sprintf("↑%d", b.Ahead)) + " " +
			styleBehind.Render(fmt.Sprintf("↓%d", b.Behind))
	case b.Ahead > 0:
		return styleAhead.Render(fmt.Sprintf("↑%d", b.Ahead))
	case b.Behind > 0:
		return styleBehind.Render(fmt.Sprintf("↓%d", b.Behind))
	default:
		return ""
	}
}

// renderBranchDetail describes the selected branch in the right-hand pane.
func renderBranchDetail(b git.Branch, width int) string {
	field := func(label, value string) string {
		if value == "" {
			value = "—"
		}
		return styleDim.Render(padRight(label, 11)) + styleBase.Render(clipStyled(value, max(width-11, 8)))
	}

	status := "up to date"
	switch {
	case b.Gone:
		status = "upstream is gone"
	case b.Upstream == "":
		status = "no upstream"
	case b.Ahead > 0 && b.Behind > 0:
		status = fmt.Sprintf("%d ahead, %d behind", b.Ahead, b.Behind)
	case b.Ahead > 0:
		status = fmt.Sprintf("%d ahead", b.Ahead)
	case b.Behind > 0:
		status = fmt.Sprintf("%d behind", b.Behind)
	}

	title := styleTitle.Render(clipStyled(b.Name, width))
	if b.Current {
		title += "  " + styleDim.Render("(current)")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		field("upstream", b.Upstream),
		field("status", status),
		"",
		field("commit", b.Hash),
		field("subject", b.Subject),
		field("author", b.Author),
		field("when", humanTime(b.Committed)),
	)
}

// humanTime renders a timestamp the way people talk about it. Exact dates are
// rarely what you want when picking a branch; "3 days ago" is.
func humanTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d.Minutes()), "minute") + " ago"
	case d < 24*time.Hour:
		return plural(int(d.Hours()), "hour") + " ago"
	case d < 30*24*time.Hour:
		return plural(int(d.Hours()/24), "day") + " ago"
	default:
		return t.Format("2006-01-02")
	}
}
