package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// clipStyled truncates styled text to a visible width.
//
// Truncating by runes would cut an ANSI escape sequence in half and leave the
// terminal in a broken colour state, so clipping is always ANSI aware.
func clipStyled(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "")
}

// lipglossWidth measures visible width, ignoring ANSI escape sequences.
func lipglossWidth(s string) int { return lipgloss.Width(s) }

// fitPane trims content to exactly the given box.
//
// lipgloss pads a box up to a size but never trims content that exceeds it, so
// one over-long line or one extra row silently stretches the whole layout and
// pushes the footer off the screen. Every pane and dialog goes through here.
func fitPane(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:max(height, 0)]
	}
	for i, line := range lines {
		lines[i] = clipStyled(line, width)
	}
	return strings.Join(lines, "\n")
}
