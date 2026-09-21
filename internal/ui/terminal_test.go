package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/mustafakarakulak/tuigy/internal/term"
)

// openShell puts a real shell in the model's terminal pane.
func openShell(t *testing.T, m Model) Model {
	t.Helper()

	t.Setenv("SHELL", "/bin/sh")
	session, err := term.Start(m.repo.Root, m.diffW, m.diffH)
	if err != nil {
		t.Fatalf("term.Start: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	next, _ := m.Update(termStartedMsg{session: session})
	return next.(Model)
}

// waitForShell drives the model until the pane shows want.
// shellReadyMarker is waited for instead of a prompt, because "$" is a user's
// prompt and "#" is root's: CI runs these in a container as root. See the same
// note in internal/term.
const shellReadyMarker = "tuigyready"

// waitReadyInPane makes the shell print the marker and waits for it to appear
// in the pane. Split so that the command echoed on screen does not match it.
func waitReadyInPane(t *testing.T, m Model) Model {
	t.Helper()

	m.shell.SendText("echo tuigy''ready")
	m.shell.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	return waitForShell(t, m, shellReadyMarker)
}

func waitForShell(t *testing.T, m Model, want string) Model {
	t.Helper()

	deadline := time.After(10 * time.Second)
	for {
		if strings.Contains(plain(m.terminalPane()), want) {
			return m
		}
		select {
		case <-deadline:
			t.Fatalf("waiting for %q in the terminal pane, it showed:\n%s", want, plain(m.terminalPane()))
		case <-m.shell.Updates():
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestTerminalOpensFocusedOnTheRepository(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)
	m = openShell(t, m)

	if m.shell == nil || !m.termFocus {
		t.Fatal("the terminal did not open with the keyboard")
	}

	m = m.typeInShell(t, "pwd")
	m = waitForShell(t, m, m.repo.Name())
}

// The terminal takes every key, including the ones that would otherwise quit
// tuigy or switch tabs: a shell that cannot receive ctrl+c is not a shell.
func TestFocusedTerminalTakesEveryKey(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)
	m = openShell(t, m)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		t.Error("ctrl+c quit tuigy instead of reaching the shell")
	}
	m = next.(Model)

	before := m.tab
	m, _ = m.press(t, "3")
	if m.tab != before {
		t.Error("a digit switched tabs instead of being typed into the shell")
	}
}

func TestDetachGivesTheKeyboardBack(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)
	m = openShell(t, m)

	m, _ = m.press(t, "ctrl+o")
	if m.termFocus {
		t.Fatal("ctrl+o did not leave the terminal")
	}
	if m.shell == nil {
		t.Fatal("leaving the terminal closed it; the pane should stay")
	}

	// The keys mean what they used to again.
	m, _ = m.press(t, "3")
	if m.tab != tabBranches {
		t.Errorf("after leaving the terminal, 3 selected %v, want the branches tab", m.tab)
	}

	// And t goes back to it.
	m, _ = m.press(t, "t")
	if !m.termFocus {
		t.Error("t did not return to the terminal")
	}
}

func TestClosingTheTerminalGivesItsRowsBack(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)
	tall := m.listH

	m = openShell(t, m)
	if m.listH >= tall {
		t.Fatalf("the panes are still %d rows with the shell open, want fewer than %d", m.listH, tall)
	}

	m, _ = m.press(t, "ctrl+o")
	m, _ = m.press(t, "T")

	if m.shell != nil || m.termFocus {
		t.Fatal("T did not close the terminal")
	}
	if m.listH != tall {
		t.Errorf("the panes are %d rows after closing the shell, want %d back", m.listH, tall)
	}
}

// The reason the shell is a band and not a pane: what you were reading has to
// still be readable while something runs.
func TestTheFileListStaysVisibleWithTheShellOpen(t *testing.T) {
	m, _ := newTreeModel(t, 120, 40)
	m = openShell(t, m)
	m = waitReadyInPane(t, m)

	body := plain(m.View())
	if !strings.Contains(body, "internal/") {
		t.Errorf("the tree is gone while the shell is open:\n%s", body)
	}
	if !strings.Contains(body, shellReadyMarker) {
		t.Errorf("the shell is not on screen:\n%s", body)
	}
}

// The whole view still has to fit, at every size, with the band in it.
func TestTheViewFitsWithTheShellOpen(t *testing.T) {
	for _, size := range []struct{ w, h int }{{120, 32}, {80, 24}, {200, 60}, {60, 12}, {40, 10}} {
		m, _ := newTestModel(t, size.w, size.h)
		m = openShell(t, m)

		lines := strings.Split(m.View(), "\n")
		if len(lines) != size.h {
			t.Errorf("%dx%d: the view is %d lines, want %d", size.w, size.h, len(lines), size.h)
		}
		for i, line := range lines {
			if w := lipglossWidth(line); w > size.w {
				t.Errorf("%dx%d: line %d is %d columns, want at most %d",
					size.w, size.h, i+1, w, size.w)
			}
		}
	}
}

// Typing "exit" is how you say you are finished with the pane, so the pane
// goes — and the repository is re-read, because something was almost certainly
// run in there.
func TestExitClosesThePane(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)
	tall := m.listH
	m = openShell(t, m)

	m = m.typeInShell(t, "exit")

	session := m.shell
	select {
	case <-session.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("the shell did not finish after exit")
	}

	next, cmd := m.Update(termExitedMsg{session: session})
	m = next.(Model)

	if m.shell != nil || m.termFocus {
		t.Fatal("the pane stayed open after the shell exited")
	}
	if m.listH != tall {
		t.Errorf("the panes are %d rows after the shell left, want %d back", m.listH, tall)
	}
	if cmd == nil {
		t.Error("the repository was not re-read after the shell exited")
	}
}

// A signal from a session that has already been closed must not redraw a pane
// that is showing something else.
func TestOutputFromAClosedSessionIsIgnored(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)
	m = openShell(t, m)

	stale := m.shell
	m.closeTerminal()

	if _, cmd := m.Update(termOutputMsg{session: stale}); cmd != nil {
		t.Error("output from a closed session asked for more work")
	}
}

func TestTerminalStartupFailureIsReported(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	next, _ := m.Update(termStartedMsg{err: errStub("no pseudo-terminals left")})
	if next.(Model).err == nil {
		t.Error("a shell that would not start was not reported")
	}
}

// ---------------------------------------------------------------- key encoding

func TestKeysReachTheShellAsTheyWereTyped(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  tea.KeyMsg
		want uv.Key
	}{
		{"enter", tea.KeyMsg{Type: tea.KeyEnter}, uv.Key{Code: uv.KeyEnter}},
		{"ctrl+c", tea.KeyMsg{Type: tea.KeyCtrlC}, uv.Key{Code: 'c', Mod: uv.ModCtrl}},
		{"ctrl+d", tea.KeyMsg{Type: tea.KeyCtrlD}, uv.Key{Code: 'd', Mod: uv.ModCtrl}},
		{"tab", tea.KeyMsg{Type: tea.KeyTab}, uv.Key{Code: uv.KeyTab}},
		{"up", tea.KeyMsg{Type: tea.KeyUp}, uv.Key{Code: uv.KeyUp}},
		{"ctrl+left", tea.KeyMsg{Type: tea.KeyCtrlLeft}, uv.Key{Code: uv.KeyLeft, Mod: uv.ModCtrl}},
		{"f5", tea.KeyMsg{Type: tea.KeyF5}, uv.Key{Code: uv.KeyF5}},
		{"ctrl+backslash", tea.KeyMsg{Type: tea.KeyCtrlBackslash}, uv.Key{Code: '\\', Mod: uv.ModCtrl}},
		{"alt+b", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b"), Alt: true},
			uv.Key{Code: 'b', Text: "b", Mod: uv.ModAlt}},
	} {
		got, ok := keyEvent(tc.msg)
		if !ok {
			t.Errorf("%s produced no key event", tc.name)
			continue
		}
		if uv.Key(got) != tc.want {
			t.Errorf("%s = %+v, want %+v", tc.name, uv.Key(got), tc.want)
		}
	}
}

// Characters go through as text so that a paste arrives whole.
func TestTypedCharactersGoThroughAsText(t *testing.T) {
	text, ok := literalText(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("git status")})
	if !ok || text != "git status" {
		t.Errorf("literalText = (%q, %v), want (\"git status\", true)", text, ok)
	}

	if _, ok := literalText(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b"), Alt: true}); ok {
		t.Error("alt+b was treated as typed text rather than as a modified key")
	}
	if text, ok := literalText(tea.KeyMsg{Type: tea.KeySpace}); !ok || text != " " {
		t.Errorf("space = (%q, %v), want (\" \", true)", text, ok)
	}
}

// ---------------------------------------------------------------- the cursor

func TestTheCursorIsDrawnWhereTheShellPutIt(t *testing.T) {
	line := withCursor("abc", 1, 20)

	if plain(line) != "abc" {
		t.Errorf("the cursor changed the text: %q", plain(line))
	}
	if line == "abc" {
		t.Error("the cursor left no mark on the row")
	}
}

// A cursor past the end of what was printed still has to be drawn, which is
// where it sits at an empty prompt.
func TestTheCursorIsDrawnPastTheEndOfTheText(t *testing.T) {
	line := withCursor("ab", 5, 20)

	if got := plain(line); len(got) < 6 {
		t.Errorf("the row was not padded out to the cursor: %q", got)
	}
}

func TestTheCursorOutsideThePaneIsNotDrawn(t *testing.T) {
	if got := withCursor("abc", 40, 20); got != "abc" {
		t.Errorf("a cursor beyond the pane changed the row: %q", got)
	}
}

// ---------------------------------------------------------------- helpers

func (m Model) typeInShell(t *testing.T, line string) Model {
	t.Helper()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(line)})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return next.(Model)
}

// Pressing t has to start a real shell, not merely set a flag.
func TestTKeyStartsAShell(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	m, _ := newTestModel(t, 120, 32)

	next, cmd := m.press(t, "t")
	if cmd == nil {
		t.Fatal("t did not start anything")
	}

	started, ok := cmd().(termStartedMsg)
	if !ok {
		t.Fatalf("t produced %T, want a termStartedMsg", cmd())
	}
	if started.err != nil {
		t.Fatalf("starting the shell failed: %v", started.err)
	}
	t.Cleanup(func() { _ = started.session.Close() })

	after, _ := next.Update(started)
	m = after.(Model)
	if m.shell == nil || !m.termFocus {
		t.Fatal("the shell did not take the pane and the keyboard")
	}

	m = waitReadyInPane(t, m)

	// Shutdown is what main calls on the way out.
	m.Shutdown()
	if !m.shell.Exited() {
		t.Error("Shutdown left the shell running")
	}
}
