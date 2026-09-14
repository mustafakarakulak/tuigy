package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

// Colouring and buffering a very large diff in full would freeze the UI on an
// eight-thousand-file agent change. Anything past the limit is cut, and the
// user is told that it was.
const (
	maxDiffLines = 4000
	maxDiffBytes = 1 << 20
)

// noHunk means the diff is being shown without a cursor, as in a commit or a
// stash, where there is nothing to stage.
const noHunk = -1

// maxWordDiffTokens bounds the line-comparison work. The algorithm is quadratic
// and a minified bundle on one line would otherwise stall the render.
const maxWordDiffTokens = 400

// renderedDiff is a diff ready to display, plus where its hunks begin so they
// can be jumped between.
type renderedDiff struct {
	content string
	// hunks holds the line offset of each hunk header within content.
	hunks []int
}

// renderDiff colourises raw git diff output.
//
// Lines are not cut to the pane width: the viewport scrolls horizontally, so
// cutting here would throw away the part the user wants to scroll to.
//
// current is the hunk the cursor is on, marked so that staging one piece of a
// file has something visible to act on. Pass noHunk where there is no cursor.
func renderDiff(raw string, current int) renderedDiff {
	if strings.TrimSpace(raw) == "" {
		return renderedDiff{content: styleDim.Render("no changes")}
	}

	truncated := false
	if len(raw) > maxDiffBytes {
		raw = raw[:maxDiffBytes]
		truncated = true
	}

	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	if len(lines) > maxDiffLines {
		lines = lines[:maxDiffLines]
		truncated = true
	}

	out := make([]string, 0, len(lines)+1)
	var hunks []int

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "@@") {
			if len(hunks) == current {
				out = append(out, styleSelected.Render(" ▸ "+expandTabs(line)+" "))
				hunks = append(hunks, len(out)-1)
				continue
			}
			hunks = append(hunks, len(out))
		}

		// A run of removed lines followed by the same number of added ones is
		// almost always the same lines edited, so show what actually changed
		// within them rather than painting both sides solid.
		if removed, added, ok := changedRun(lines[i:]); ok {
			out = append(out, emphasisePairs(removed, added)...)
			i += len(removed) + len(added) - 1
			continue
		}

		out = append(out, styleDiffLine(line).Render(expandTabs(line)))
	}

	if truncated {
		out = append(out, styleDim.Render(fmt.Sprintf("… diff truncated (showing the first %d lines)", len(lines))))
	}

	return renderedDiff{content: strings.Join(out, "\n"), hunks: hunks}
}

// changedRun matches a block of removed lines immediately followed by the same
// number of added ones, which is what an edited line looks like in a diff.
func changedRun(lines []string) (removed, added []string, ok bool) {
	for _, l := range lines {
		if !isRemoved(l) {
			break
		}
		removed = append(removed, l)
	}
	if len(removed) == 0 {
		return nil, nil, false
	}

	for _, l := range lines[len(removed):] {
		if !isAdded(l) {
			break
		}
		added = append(added, l)
	}

	if len(added) != len(removed) {
		return nil, nil, false
	}
	return removed, added, true
}

func isRemoved(l string) bool { return strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---") }
func isAdded(l string) bool   { return strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++") }

// emphasisePairs renders a changed block with the differing words picked out.
// Removed lines stay together above the added ones, as git writes them.
func emphasisePairs(removed, added []string) []string {
	left := make([]string, len(removed))
	right := make([]string, len(added))

	for i := range removed {
		before, after := expandTabs(removed[i]), expandTabs(added[i])
		leftParts, rightParts := wordDiff(before[1:], after[1:])

		left[i] = styleRemoved.Render("-") + render(leftParts, styleRemoved, styleRemovedEmph)
		right[i] = styleAdded.Render("+") + render(rightParts, styleAdded, styleAddedEmph)
	}

	return append(left, right...)
}

// part is a run of text that either matches the other side or does not.
type part struct {
	text    string
	changed bool
}

func render(parts []part, plain, emphasised lipgloss.Style) string {
	var b strings.Builder
	for _, p := range parts {
		if p.changed {
			b.WriteString(emphasised.Render(p.text))
			continue
		}
		b.WriteString(plain.Render(p.text))
	}
	return b.String()
}

// wordDiff splits two lines into the parts they share and the parts they do not.
//
// Whole-line colouring hides a one-character change in a long line: both sides
// are simply red and green. This is what makes such a change visible.
func wordDiff(before, after string) (left, right []part) {
	a, b := tokenise(before), tokenise(after)
	if len(a) > maxWordDiffTokens || len(b) > maxWordDiffTokens {
		return []part{{text: before}}, []part{{text: after}}
	}

	common := longestCommon(a, b)
	return mark(a, common), mark(b, common)
}

// tokenise splits a line into runs of one kind of character, so a changed
// identifier is one part rather than a scatter of letters — and so that
// changing "=" to ":=" marks the operator without dragging in the spaces
// around it.
func tokenise(s string) []string {
	var out []string
	var current strings.Builder
	var kind rune

	flush := func() {
		if current.Len() > 0 {
			out = append(out, current.String())
			current.Reset()
		}
	}

	for _, r := range s {
		var k rune
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			k = 'w'
		case unicode.IsSpace(r):
			k = 's'
		default:
			k = 'p'
		}

		if current.Len() > 0 && k != kind {
			flush()
		}
		kind = k
		current.WriteRune(r)
	}
	flush()
	return out
}

// longestCommon is the longest common subsequence of two token lists.
func longestCommon(a, b []string) []string {
	grid := make([][]int, len(a)+1)
	for i := range grid {
		grid[i] = make([]int, len(b)+1)
	}

	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				grid[i][j] = grid[i+1][j+1] + 1
				continue
			}
			grid[i][j] = max(grid[i+1][j], grid[i][j+1])
		}
	}

	var out []string
	for i, j := 0, 0; i < len(a) && j < len(b); {
		switch {
		case a[i] == b[j]:
			out = append(out, a[i])
			i, j = i+1, j+1
		case grid[i+1][j] >= grid[i][j+1]:
			i++
		default:
			j++
		}
	}
	return out
}

// mark walks a token list against the shared subsequence, gathering runs.
func mark(tokens, common []string) []part {
	var out []part
	next := 0

	add := func(text string, changed bool) {
		if text == "" {
			return
		}
		if n := len(out); n > 0 && out[n-1].changed == changed {
			out[n-1].text += text
			return
		}
		out = append(out, part{text: text, changed: changed})
	}

	for _, token := range tokens {
		if next < len(common) && token == common[next] {
			add(token, false)
			next++
			continue
		}
		add(token, true)
	}
	return out
}

func styleDiffLine(line string) lipgloss.Style {
	switch {
	case strings.HasPrefix(line, "@@"):
		return styleHunk
	case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
		return styleMeta
	case strings.HasPrefix(line, "+"):
		return styleAdded
	case strings.HasPrefix(line, "-"):
		return styleRemoved
	case strings.HasPrefix(line, "diff --git"),
		strings.HasPrefix(line, "index "),
		strings.HasPrefix(line, "new file"),
		strings.HasPrefix(line, "deleted file"),
		strings.HasPrefix(line, "old mode"),
		strings.HasPrefix(line, "new mode"),
		strings.HasPrefix(line, "similarity index"),
		strings.HasPrefix(line, "rename "),
		strings.HasPrefix(line, "Binary files"),
		strings.HasPrefix(line, `\ No newline`):
		return styleMeta
	default:
		return styleBase
	}
}

// expandTabs keeps columns lining up. Terminals vary in how they render a tab
// inside a bordered pane, and a diff that does not line up is hard to read.
func expandTabs(line string) string { return strings.ReplaceAll(line, "\t", "    ") }
