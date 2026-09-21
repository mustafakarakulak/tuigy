package ui

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// TestMain forces a colour profile and keeps the tests out of the user's own
// configuration.
//
// Without a terminal lipgloss renders every style as plain text, which would
// make every assertion about how something looks quietly vacuous. The config
// directory is redirected because the interface writes to it — opening a
// repository records it in the project list — and a test run must not leave
// anything behind in ~/.config.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	dir, err := os.MkdirTemp("", "tuigy-test-config")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// plain strips the styling from rendered output.
//
// Assertions about what the interface says have to look past the escape
// sequences: "? help" is written as "?" then a colour change then " help", so a
// plain substring check would miss text that is plainly on the screen.
func plain(s string) string { return ansi.Strip(s) }
