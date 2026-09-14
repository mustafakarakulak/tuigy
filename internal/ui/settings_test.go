package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mustafakarakulak/tuigy/internal/config"
)

// useConfigDir points config reads and writes at a temp directory, and puts the
// default theme back afterwards since styles are process-wide.
func useConfigDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Cleanup(func() {
		if err := ApplyTheme("default", nil); err != nil {
			t.Fatalf("restoring the default theme: %v", err)
		}
	})
	return dir
}

// Changing how tuigy looks has to be reachable from inside tuigy, not only by
// knowing a file exists.
func TestSettingsIsReachableAndNamesTheFile(t *testing.T) {
	useConfigDir(t)
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, ",")
	if m.modal != modalSettings {
		t.Fatal(", should open the settings view")
	}

	view := m.View()
	for _, want := range append(ThemeNames(), "config.yml", "--init-config") {
		if !strings.Contains(view, want) {
			t.Errorf("the settings view does not mention %q", want)
		}
	}

	// The help screen has to point at it, or it is just another hidden key.
	m, _ = m.press(t, "esc")
	m, _ = m.press(t, "?")
	if got := m.helpContent(); !strings.Contains(got, "settings") {
		t.Errorf("the help screen does not mention the settings view:\n%s", got)
	}
}

// A colour scheme cannot be judged from its name, so moving the cursor applies it.
func TestSettingsPreviewsThemesWhileMoving(t *testing.T) {
	useConfigDir(t)
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, ",")
	start := currentPalette

	next, cmd := m.press(t, "j")
	m = next.applyCmd(t, cmd)

	if currentPalette == start {
		t.Error("moving the cursor did not repaint anything")
	}
	if got := ThemeNames()[m.themeCursor]; currentPalette.Accent != themes[got].Accent {
		t.Errorf("the palette does not match the highlighted theme %q", got)
	}
}

// Abandoning the view has to leave the colours exactly as they were.
func TestSettingsRevertsOnCancel(t *testing.T) {
	useConfigDir(t)

	if _, err := config.SetTheme("nord"); err != nil {
		t.Fatalf("SetTheme: %v", err)
	}
	if err := ApplyTheme("nord", nil); err != nil {
		t.Fatalf("ApplyTheme: %v", err)
	}

	m, _ := newTestModel(t, 120, 32, WithTheme("nord"))
	before := currentPalette

	m, _ = m.press(t, ",")
	m = m.selectTheme(t, "dracula")
	if currentPalette == before {
		t.Fatal("setup: nothing was previewed")
	}

	next, cmd := m.press(t, "esc")
	m = next.applyCmd(t, cmd)

	if m.modal != modalNone {
		t.Error("esc should close the settings view")
	}
	if currentPalette != before {
		t.Error("esc left a previewed theme applied")
	}
}

func TestSettingsSavesTheChoice(t *testing.T) {
	dir := useConfigDir(t)
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, ",")
	m = m.selectTheme(t, "gruvbox")

	next, cmd := m.press(t, "enter")
	m = next.applyCmd(t, cmd)

	if m.modal != modalNone {
		t.Error("saving should close the settings view")
	}
	if m.theme != "gruvbox" {
		t.Errorf("theme = %q, want the choice recorded", m.theme)
	}
	if !strings.Contains(m.footerView(), "theme saved") {
		t.Errorf("the footer does not confirm the save:\n%s", m.footerView())
	}

	body, err := os.ReadFile(filepath.Join(dir, "tuigy", "config.yml"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(body), "theme: gruvbox") {
		t.Errorf("the file does not hold the choice:\n%s", body)
	}

	// And it survives a restart, which is the whole point of saving it.
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Theme != "gruvbox" {
		t.Errorf("Theme = %q after reloading", cfg.Theme)
	}
}

// The view opens on the theme in use rather than at the top of the list.
func TestSettingsOpensOnTheCurrentTheme(t *testing.T) {
	useConfigDir(t)
	m, _ := newTestModel(t, 120, 32, WithTheme("nord"))

	m, _ = m.press(t, ",")
	if got := ThemeNames()[m.themeCursor]; got != "nord" {
		t.Errorf("the cursor is on %q, want the theme in use", got)
	}
}

func TestSettingsFitsTerminal(t *testing.T) {
	useConfigDir(t)

	for _, size := range []struct{ w, h int }{{120, 32}, {80, 24}, {60, 12}, {40, 10}} {
		m, _ := newTestModel(t, size.w, size.h)
		m, _ = m.press(t, ",")

		lines := strings.Split(m.View(), "\n")
		if len(lines) != size.h {
			t.Errorf("%dx%d: view is %d lines, want %d", size.w, size.h, len(lines), size.h)
		}
		for i, line := range lines {
			if w := lipglossWidth(line); w > size.w {
				t.Errorf("%dx%d: line %d is %d columns, want at most %d", size.w, size.h, i+1, w, size.w)
			}
		}
	}
}

// selectTheme moves the settings cursor onto a theme using the keys.
func (m Model) selectTheme(t *testing.T, name string) Model {
	t.Helper()

	want := slices.Index(ThemeNames(), name)
	if want < 0 {
		t.Fatalf("there is no theme called %q", name)
	}

	stroke := "j"
	if want < m.themeCursor {
		stroke = "k"
	}

	for range len(ThemeNames()) {
		if m.themeCursor == want {
			return m
		}
		next, cmd := m.press(t, stroke)
		m = next.applyCmd(t, cmd)
	}

	t.Fatalf("could not reach theme %q", name)
	return m
}
