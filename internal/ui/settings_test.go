package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mustafakarakulak/tuigy/internal/config"
	"github.com/mustafakarakulak/tuigy/internal/keys"
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
	if got := plain(m.helpContent()); !strings.Contains(got, "settings") {
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
	if !strings.Contains(plain(m.footerView()), "theme saved") {
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

// ---------------------------------------------------------------- key bindings

// Which key does what has to be changeable from inside tuigy, not only by
// knowing that a YAML file exists.
func TestSettingsHasAKeyBindingsPage(t *testing.T) {
	useConfigDir(t)
	m, _ := newTestModel(t, 120, 40)

	m, _ = m.press(t, ",")
	m, _ = m.press(t, "tab")

	if m.settingsPage != pageKeys {
		t.Fatal("tab should reach the key bindings page")
	}

	view := plain(m.View())
	for _, want := range []string{"Key bindings", "commit", "change this key"} {
		if !strings.Contains(view, want) {
			t.Errorf("the page does not show %q:\n%s", want, view)
		}
	}

	// Every action is listed, not a hand-picked few.
	if got := len(m.keyRows()); got != len(keys.ActionNames()) {
		t.Errorf("the page lists %d actions, want all %d", got, len(keys.ActionNames()))
	}
}

func TestRebindingAnAction(t *testing.T) {
	dir := useConfigDir(t)
	m, _ := newTestModel(t, 120, 40)

	m = m.openKeys(t, "commit")
	m, _ = m.press(t, "enter")
	if !m.capturing {
		t.Fatal("enter should wait for the key to bind")
	}

	next, cmd := m.press(t, "ctrl+k")
	m = next.applyCmd(t, cmd)

	// The running map changes at once, so the footer and help are right now
	// rather than after a restart.
	binding, ok := m.keys.Get("commit")
	if !ok || !slices.Contains(binding.Keys(), "ctrl+k") {
		t.Fatalf("commit is on %v, want ctrl+k", binding.Keys())
	}
	if binding.Help().Desc != "commit" {
		t.Errorf("the description was lost: %q", binding.Help().Desc)
	}

	// And it is on disk, so it survives a restart.
	body, err := os.ReadFile(filepath.Join(dir, "tuigy", "config.yml"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(body), "commit:") || !strings.Contains(string(body), "ctrl+k") {
		t.Errorf("the binding was not written:\n%s", body)
	}
	if !strings.Contains(plain(m.footerView()), "commit saved") {
		t.Errorf("the footer does not confirm the save:\n%s", m.footerView())
	}
}

// The new key has to actually do the thing, which is the only test that matters.
func TestARebboundKeyWorks(t *testing.T) {
	useConfigDir(t)
	m, _ := newTestModel(t, 120, 40)

	m = m.openKeys(t, "commit")
	m, _ = m.press(t, "enter")
	next, cmd := m.press(t, "ctrl+k")
	m = next.applyCmd(t, cmd)

	m, _ = m.press(t, "esc")
	m, _ = m.press(t, "ctrl+k")

	if m.modal != modalCommit {
		t.Error("the rebound key did not open the commit view")
	}
}

func TestResettingABindingPutsTheDefaultBack(t *testing.T) {
	dir := useConfigDir(t)
	m, _ := newTestModel(t, 120, 40)

	m = m.openKeys(t, "commit")
	m, _ = m.press(t, "enter")
	next, cmd := m.press(t, "ctrl+k")
	m = next.applyCmd(t, cmd)

	if m.keyRows()[m.keyCursor].changed != true {
		t.Fatal("setup: the binding is not marked as changed")
	}

	next, cmd = m.press(t, "r")
	m = next.applyCmd(t, cmd)

	if binding, _ := m.keys.Get("commit"); !slices.Contains(binding.Keys(), "c") {
		t.Errorf("commit is on %v after a reset, want the default c", binding.Keys())
	}

	body, _ := os.ReadFile(filepath.Join(dir, "tuigy", "config.yml"))
	if strings.Contains(string(body), "ctrl+k") {
		t.Errorf("the override was left in the file:\n%s", body)
	}
}

// Escape is the only way out of a mode that swallows every other key, so it
// cannot itself be captured.
func TestEscapeLeavesCaptureWithoutBinding(t *testing.T) {
	useConfigDir(t)
	m, _ := newTestModel(t, 120, 40)

	m = m.openKeys(t, "commit")
	m, _ = m.press(t, "enter")
	m, _ = m.press(t, "esc")

	if m.capturing {
		t.Fatal("esc did not leave capture")
	}
	if m.modal != modalSettings {
		t.Error("esc left the whole dialog rather than the capture")
	}
	if binding, _ := m.keys.Get("commit"); !slices.Contains(binding.Keys(), "c") {
		t.Errorf("commit changed to %v, want the default", binding.Keys())
	}
}

// ctrl+c quits before the key map is consulted, so accepting it would make a
// shortcut that silently never fires.
func TestCtrlCCannotBeGivenAway(t *testing.T) {
	useConfigDir(t)
	m, _ := newTestModel(t, 120, 40)

	m = m.openKeys(t, "commit")
	m, _ = m.press(t, "enter")
	m, _ = m.press(t, "ctrl+c")

	if binding, _ := m.keys.Get("commit"); slices.Contains(binding.Keys(), "ctrl+c") {
		t.Error("ctrl+c was accepted as a binding")
	}
	if !strings.Contains(plain(m.View()), "always quits") {
		t.Errorf("the page does not say why it was refused:\n%s", plain(m.View()))
	}
}

// Sharing a key is legal — enter does three different things — but it is worth
// being told, because the case that is a mistake looks the same until you are.
func TestASharedKeyIsReported(t *testing.T) {
	useConfigDir(t)
	m, _ := newTestModel(t, 120, 40)

	m = m.openKeys(t, "commit")
	m, _ = m.press(t, "enter")
	next, cmd := m.press(t, "d") // already discard
	m = next.applyCmd(t, cmd)

	if !strings.Contains(m.keyNote, "discard") {
		t.Errorf("the clash with discard was not reported: %q", m.keyNote)
	}
	// Reported, not refused.
	if binding, _ := m.keys.Get("commit"); !slices.Contains(binding.Keys(), "d") {
		t.Errorf("the binding was refused: %v", binding.Keys())
	}
}

// 63 actions is more than anyone scrolls through.
func TestFilteringTheActionList(t *testing.T) {
	useConfigDir(t)
	m, _ := newTestModel(t, 120, 40)

	m, _ = m.press(t, ",")
	m, _ = m.press(t, "tab")
	m, _ = m.press(t, "/")

	for _, r := range "stash" {
		m, _ = m.press(t, string(r))
	}

	rows := m.keyRows()
	if len(rows) == 0 || len(rows) == len(keys.ActionNames()) {
		t.Fatalf("the filter matched %d of %d actions", len(rows), len(keys.ActionNames()))
	}
	for _, r := range rows {
		if !strings.Contains(r.action, "stash") && !strings.Contains(strings.ToLower(r.desc), "stash") {
			t.Errorf("%q does not match the filter", r.action)
		}
	}
}

func TestSettingsPagesFitTerminal(t *testing.T) {
	useConfigDir(t)

	for _, size := range []struct{ w, h int }{{120, 40}, {80, 24}, {60, 12}, {40, 10}} {
		m, _ := newTestModel(t, size.w, size.h)
		m, _ = m.press(t, ",")
		m, _ = m.press(t, "tab")

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

// openKeys reaches the key bindings page with the cursor on one action.
func (m Model) openKeys(t *testing.T, action string) Model {
	t.Helper()

	m, _ = m.press(t, ",")
	m, _ = m.press(t, "tab")

	for i, r := range m.keyRows() {
		if r.action == action {
			m.keyCursor = i
			m.keyOff = scrollTo(m.keyCursor, m.keyOff, keysPerPage)
			return m
		}
	}

	t.Fatalf("%q is not in the action list", action)
	return m
}

func TestMovingThroughTheActionList(t *testing.T) {
	useConfigDir(t)
	m, _ := newTestModel(t, 120, 40)

	m, _ = m.press(t, ",")
	m, _ = m.press(t, "tab")

	first := m.keyRows()[m.keyCursor].action
	m, _ = m.press(t, "j")
	if m.keyRows()[m.keyCursor].action == first {
		t.Fatal("j did not move down the list")
	}
	m, _ = m.press(t, "k")
	if got := m.keyRows()[m.keyCursor].action; got != first {
		t.Errorf("k landed on %q, want back on %q", got, first)
	}

	// Paging keeps the cursor inside the window it is drawn in.
	m, _ = m.press(t, "ctrl+d")
	if m.keyCursor < m.keyOff || m.keyCursor >= m.keyOff+keysPerPage {
		t.Errorf("after a page down the cursor is at %d, outside the window at %d", m.keyCursor, m.keyOff)
	}

	// The end of the list is the end, not an index out of range.
	for range len(keys.ActionNames()) + 5 {
		m, _ = m.press(t, "j")
	}
	if m.keyCursor != len(m.keyRows())-1 {
		t.Errorf("the cursor is at %d, want the last of %d rows", m.keyCursor, len(m.keyRows()))
	}
}

func TestLeavingTheActionFilter(t *testing.T) {
	useConfigDir(t)
	m, _ := newTestModel(t, 120, 40)

	m, _ = m.press(t, ",")
	m, _ = m.press(t, "tab")
	m, _ = m.press(t, "/")
	for _, r := range "stash" {
		m, _ = m.press(t, string(r))
	}
	m, _ = m.press(t, "backspace")
	if m.keyFilter != "stas" {
		t.Errorf("filter = %q after backspace, want %q", m.keyFilter, "stas")
	}

	// enter accepts it: the keys go back to being the list's.
	m, _ = m.press(t, "enter")
	if m.keyFiltering {
		t.Error("enter did not accept the filter")
	}
	if m.keyFilter != "stas" {
		t.Error("accepting the filter dropped it")
	}

	// esc clears it and shows everything again.
	m, _ = m.press(t, "/")
	m, _ = m.press(t, "esc")
	if m.keyFilter != "" || m.keyFiltering {
		t.Errorf("esc left the filter as %q", m.keyFilter)
	}
	if len(m.keyRows()) != len(keys.ActionNames()) {
		t.Error("clearing the filter did not bring the whole list back")
	}
}
