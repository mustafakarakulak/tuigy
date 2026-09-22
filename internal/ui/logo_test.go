package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// The art is a constant that is easy to edit and easy to edit wrongly: a row
// one cell short leaves a notch nothing else would report.
func TestTheLogoIsARectangle(t *testing.T) {
	lines := strings.Split(logoArt, "\n")

	if len(lines) != logoHeight {
		t.Errorf("the art is %d rows, but logoHeight says %d", len(lines), logoHeight)
	}
	for i, line := range lines {
		// Trailing blanks are trimmed in the source, so a row is allowed to be
		// short — never longer, which is what would break a layout.
		if w := lipgloss.Width(line); w > logoWidth {
			t.Errorf("row %d is %d columns, wider than the %d logoWidth claims", i+1, w, logoWidth)
		}
	}
}

// Before the first status there is nothing to say about the repository, which
// is the one moment tuigy has the screen and no work to show in it.
func TestTheSplashIsShownUntilTheRepositoryArrives(t *testing.T) {
	m, _ := newTestModel(t, 100, 30)

	loading := m
	loading.status, loading.opened = nil, false
	if !strings.Contains(plain(loading.View()), "▄███") {
		t.Errorf("the mark is not on the first screen:\n%s", plain(loading.View()))
	}

	// Once a repository has been drawn, the interface is what there is to show.
	if got := plain(m.View()); strings.Contains(got, "▄███") {
		t.Errorf("the splash outlived the first status:\n%s", got)
	}
}

// Switching projects empties the same fields the first paint had empty. A mark
// thrown over the interface on every switch is the splash screen people are
// right to complain about.
func TestTheSplashDoesNotComeBackWhenSwitchingProjects(t *testing.T) {
	m, _ := newTestModel(t, 100, 30)

	switching := m
	switching.status = nil // what adoptRepo does
	if strings.Contains(plain(switching.View()), "▄███") {
		t.Error("switching projects brought the splash back")
	}
}

// A first load that fails has something to say, and saying it beats a logo.
func TestAFailedFirstLoadShowsTheInterface(t *testing.T) {
	m, _ := newTestModel(t, 100, 30)

	failed := m
	failed.status, failed.opened = nil, false
	failed.err = errors.New("not a git repository")

	if got := plain(failed.View()); strings.Contains(got, "▄███") {
		t.Errorf("the splash hid an error:\n%s", got)
	}
}

// Neither screen may break a layout to show a picture.
func TestTheLogoGivesWayOnANarrowTerminal(t *testing.T) {
	for _, size := range []struct{ w, h int }{{100, 30}, {80, 24}, {60, 16}, {40, 10}} {
		m, _ := newTestModel(t, size.w, size.h)

		loading := m
		loading.status, loading.opened = nil, false
		for i, line := range strings.Split(loading.View(), "\n") {
			if w := lipglossWidth(line); w > size.w {
				t.Errorf("%dx%d: splash line %d is %d columns", size.w, size.h, i+1, w)
			}
		}

		withHelp, _ := m.press(t, "?")
		for i, line := range strings.Split(withHelp.View(), "\n") {
			if w := lipglossWidth(line); w > size.w {
				t.Errorf("%dx%d: help line %d is %d columns", size.w, size.h, i+1, w)
			}
		}

		// Narrow enough and the name in plain text says more than a smear.
		if size.w < logoWidth {
			if !strings.Contains(plain(loading.View()), "tuigy") {
				t.Errorf("%dx%d: the name is missing from the splash", size.w, size.h)
			}
			if !strings.Contains(plain(withHelp.View()), "Shortcuts") {
				t.Errorf("%dx%d: the help screen lost its title", size.w, size.h)
			}
		}
	}
}

// The help screen is the one place opened to be read rather than worked in, so
// it is where the mark is actually seen.
func TestTheHelpScreenCarriesTheMark(t *testing.T) {
	m, _ := newTestModel(t, 120, 40)
	m, _ = m.press(t, "?")

	view := plain(m.View())
	if !strings.Contains(view, "▄███") {
		t.Errorf("the help screen has no mark:\n%s", view)
	}
	// It replaces the title rather than being added above it.
	if strings.Contains(view, "Shortcuts") {
		t.Error("the help screen shows both the mark and the title it replaces")
	}
	// And the shortcuts themselves survive it.
	if !strings.Contains(view, "cherry-pick") {
		t.Errorf("the mark pushed the shortcuts off the screen:\n%s", view)
	}
}

// The splash is drawn before any window size has arrived too, which is the one
// moment there is nothing to centre in.
func TestTheSplashBeforeAnySize(t *testing.T) {
	m, _ := newTestModel(t, 100, 30)

	unsized := m
	unsized.ready, unsized.status, unsized.opened = false, nil, false
	if got := unsized.View(); !strings.Contains(got, "tuigy") {
		t.Errorf("View before a window size = %q", got)
	}
}
