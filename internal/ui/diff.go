package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colouring and buffering a very large diff in full would freeze the UI on an
// eight-thousand-file agent change. Anything past the limit is cut, and the
// user is told that it was.
const (
	maxDiffLines = 4000
	maxDiffBytes = 1 << 20
)

// renderDiff colourises raw git diff output.
func renderDiff(raw string, width int) string {
	if strings.TrimSpace(raw) == "" {
		return styleDim.Render("no changes")
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
	for _, line := range lines {
		out = append(out, styleDiffLine(line).Render(clipLine(line, width)))
	}
	if truncated {
		out = append(out, styleDim.Render(fmt.Sprintf("… diff truncated (showing the first %d lines)", len(lines))))
	}

	return strings.Join(out, "\n")
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

// clipLine fits a line to the pane width. Diff lines must never wrap: wrapping
// breaks the alignment of added and removed lines and makes the diff unreadable.
func clipLine(line string, width int) string {
	line = strings.ReplaceAll(line, "\t", "    ")
	if width <= 0 {
		return line
	}
	r := []rune(line)
	if len(r) <= width {
		return line
	}
	return string(r[:width-1]) + "›"
}
