package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mustafakarakulak/tuigy/internal/git"
)

type section int

const (
	secConflict section = iota
	secStaged
	secUnstaged
	secUntracked
)

func (s section) title() string {
	switch s {
	case secConflict:
		return "CONFLICTS"
	case secStaged:
		return "STAGED"
	case secUnstaged:
		return "UNSTAGED"
	default:
		return "UNTRACKED"
	}
}

// row is one line of the file list: either a section header or a file.
type row struct {
	header bool
	sec    section
	count  int
	file   git.FileStatus
}

// staged reports whether the row represents the index side of the file.
//
// The same file can appear under both STAGED and UNSTAGED, so this comes from
// the section the row sits in rather than from the file itself.
func (r row) staged() bool { return r.sec == secStaged }

// letter is the single-character status code shown on the row.
func (r row) letter() string {
	switch {
	case r.file.Unmerged:
		return "!"
	case r.staged():
		return r.file.Index.String()
	default:
		return r.file.Worktree.String()
	}
}

// label is the name shown in the list, including the old path for a rename.
func (r row) label() string {
	if r.file.OrigPath != "" {
		return r.file.OrigPath + " → " + r.file.Path
	}
	return r.file.Path
}

func buildRows(st *git.Status, filter string) []row {
	if st == nil {
		return nil
	}

	var rows []row
	add := func(sec section, files []git.FileStatus) {
		matched := make([]git.FileStatus, 0, len(files))
		for _, f := range files {
			if matchesFilter(filter, f.Path) || matchesFilter(filter, f.OrigPath) {
				matched = append(matched, f)
			}
		}
		if len(matched) == 0 {
			return
		}

		rows = append(rows, row{header: true, sec: sec, count: len(matched)})
		for _, f := range matched {
			rows = append(rows, row{sec: sec, file: f})
		}
	}

	add(secConflict, st.Conflicted())
	add(secStaged, st.Staged())
	add(secUnstaged, st.Unstaged())
	add(secUntracked, st.Untracked())
	return rows
}

func letterStyle(letter string) lipgloss.Style {
	switch letter {
	case "A", "?":
		return styleAdded
	case "D":
		return styleRemoved
	case "!":
		return styleDirty
	case "R", "C":
		return styleHunk
	default:
		return styleBase
	}
}

// renderList draws the visible window of the list. Scrolling is handled here
// rather than with a viewport so that it lives beside the header-skipping
// cursor logic it has to agree with.
func renderList(rows []row, cursor, offset, width, height int, reviewed map[string]fileStamp) string {
	if len(rows) == 0 {
		return styleDim.Render("no changes")
	}

	end := min(offset+height, len(rows))
	lines := make([]string, 0, end-offset)

	for i := offset; i < end; i++ {
		r := rows[i]
		if r.header {
			lines = append(lines, styleSection.Render(fmt.Sprintf("%s (%d)", r.sec.title(), r.count)))
			continue
		}

		letter := r.letter()

		// A reviewed file is marked in its own column, so the mark survives
		// scrolling and is visible without selecting anything.
		mark := " "
		if _, ok := reviewed[r.file.Path]; ok {
			mark = "✓"
		}

		// Width budget: the review mark, a space, the status letter, a space.
		name := truncateLeft(r.label(), width-4)

		if i == cursor {
			lines = append(lines, styleSelected.Width(width).Render(mark+" "+letter+" "+name))
			continue
		}
		lines = append(lines, styleAdded.Render(mark)+" "+
			letterStyle(letter).Render(letter)+" "+styleBase.Render(name))
	}

	return strings.Join(lines, "\n")
}

// truncateLeft shortens long paths from the front so the file name stays visible.
func truncateLeft(s string, width int) string {
	if width <= 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	return "…" + string(r[len(r)-(width-1):])
}
