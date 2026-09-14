package ui

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// TestMain forces a colour profile.
//
// Without a terminal lipgloss renders every style as plain text, which would
// make every assertion about how something looks quietly vacuous.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

// plain strips the styling from rendered output.
//
// Assertions about what the interface says have to look past the escape
// sequences: "? help" is written as "?" then a colour change then " help", so a
// plain substring check would miss text that is plainly on the screen.
func plain(s string) string { return ansi.Strip(s) }
