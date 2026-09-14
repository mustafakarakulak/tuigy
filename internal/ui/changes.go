package ui

import (
	"fmt"
	"path"
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

// row is one line of the file list: a section heading, a directory heading, or
// a file. Both kinds of heading set header, so the cursor skips them alike.
type row struct {
	header bool
	// dir is set on a directory heading, and names the directory.
	dir string
	// indent marks a file shown underneath a directory heading, where only its
	// name is written.
	indent bool

	sec   section
	count int
	file  git.FileStatus
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
//
// Under a directory heading only the file name is written, since the rest of
// the path is already on the line above.
func (r row) label() string {
	if r.file.OrigPath != "" {
		return r.file.OrigPath + " → " + r.file.Path
	}
	if r.indent {
		return path.Base(r.file.Path)
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
		rows = append(rows, groupByDirectory(sec, matched)...)
	}

	add(secConflict, st.Conflicted())
	add(secStaged, st.Staged())
	add(secUnstaged, st.Unstaged())
	add(secUntracked, st.Untracked())
	return rows
}

// groupByDirectory gives a directory its own line when more than one changed
// file sits in it.
//
// An agent working across a package produces a column of paths that all begin
// the same way; naming the directory once turns that into something scannable.
// A directory holding a single file keeps its full path inline, because a
// heading for one entry is just another line to read.
func groupByDirectory(sec section, files []git.FileStatus) []row {
	var out []row

	for i := 0; i < len(files); {
		dir := directoryOf(files[i])

		j := i
		for j < len(files) && directoryOf(files[j]) == dir {
			j++
		}

		if dir == "" || j-i < 2 {
			for _, f := range files[i:j] {
				out = append(out, row{sec: sec, file: f})
			}
			i = j
			continue
		}

		out = append(out, row{header: true, dir: dir, sec: sec, count: j - i})
		for _, f := range files[i:j] {
			out = append(out, row{sec: sec, file: f, indent: true})
		}
		i = j
	}

	return out
}

// directoryOf is the directory a change belongs under, or empty when there is
// nothing to group it by: a file at the root, an untracked directory that is
// already one line, or a rename whose two paths would not agree.
func directoryOf(f git.FileStatus) string {
	if f.OrigPath != "" || strings.HasSuffix(f.Path, "/") {
		return ""
	}
	dir := path.Dir(f.Path)
	if dir == "." {
		return ""
	}
	return dir + "/"
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
		switch {
		case r.header && r.dir != "":
			lines = append(lines, " "+styleDim.Render(truncateLeft(r.dir, width-1)))
			continue
		case r.header:
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

		// Width budget: the review mark, a space, the status letter, a space,
		// and two more when the file sits under a directory heading.
		indent := ""
		if r.indent {
			indent = "  "
		}
		name := truncateLeft(r.label(), width-4-len(indent))

		if i == cursor {
			lines = append(lines, styleSelected.Width(width).Render(mark+" "+letter+" "+indent+name))
			continue
		}
		lines = append(lines, styleAdded.Render(mark)+" "+
			letterStyle(letter).Render(letter)+" "+indent+styleBase.Render(name))
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
