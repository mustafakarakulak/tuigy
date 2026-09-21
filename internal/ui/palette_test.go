package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/mustafakarakulak/tuigy/internal/keys"
)

// labels is what the palette is currently offering, in order.
func (m Model) labels() []string {
	out := make([]string, 0, len(m.matchingCommands()))
	for _, c := range m.matchingCommands() {
		out = append(out, c.label)
	}
	return out
}

func (m Model) openPaletteFor(t *testing.T) Model {
	t.Helper()

	m, _ = m.press(t, ":")
	if m.modal != modalPalette {
		t.Fatal(`":" did not open the palette`)
	}
	return m
}

func hasLabel(labels []string, want string) bool {
	return slices.ContainsFunc(labels, func(l string) bool { return strings.Contains(l, want) })
}

// Every command names an action that exists, or the key shown beside it would
// be blank and the table would have drifted from the key map without anything
// noticing.
func TestEveryCommandNamesARealAction(t *testing.T) {
	m, _ := newTestModel(t, 120, 40)
	known := keys.ActionNames()

	for _, tab := range allTabs {
		on := m
		on.tab = tab

		for _, c := range on.commands() {
			if !slices.Contains(known, c.action) {
				t.Errorf("tab %d: command %q names unknown action %q", tab, c.label, c.action)
			}
			if stroke := on.commandStroke(c); stroke == "" {
				t.Errorf("tab %d: command %q shows no key", tab, c.label)
			}
			if c.run == nil {
				t.Errorf("tab %d: command %q does nothing", tab, c.label)
			}
		}
	}
}

// The palette offers what the keyboard would do from here, so a command that
// belongs to another tab is not on the list pretending it would work.
func TestPaletteOffersOnlyWhatWorksHere(t *testing.T) {
	m, _ := newTestModel(t, 120, 40)

	changes := m.openPaletteFor(t).labels()
	if !hasLabel(changes, "commit the staged changes") {
		t.Errorf("the changes tab does not offer commit:\n%v", changes)
	}
	if hasLabel(changes, "pop this stash") {
		t.Errorf("the changes tab offers a stash command:\n%v", changes)
	}

	onStashes, _ := m.press(t, "5")
	stashes := onStashes.openPaletteFor(t).labels()
	if !hasLabel(stashes, "pop this stash") {
		t.Errorf("the stashes tab does not offer pop:\n%v", stashes)
	}
	if hasLabel(stashes, "commit the staged changes") {
		t.Errorf("the stashes tab offers commit:\n%v", stashes)
	}

	// What works anywhere is on both.
	for _, want := range []string{"push", "switch to another repository", "quit tuigy"} {
		if !hasLabel(changes, want) || !hasLabel(stashes, want) {
			t.Errorf("%q is not offered on both tabs", want)
		}
	}
}

// The tab's own commands come first: they are the ones that needed the context.
func TestPaletteListsTheTabsCommandsFirst(t *testing.T) {
	m, _ := newTestModel(t, 120, 40)
	labels := m.openPaletteFor(t).labels()

	if len(labels) == 0 {
		t.Fatal("the palette is empty")
	}
	if !strings.Contains(labels[0], "commit the staged changes") {
		t.Errorf("first command is %q, want the changes tab's own", labels[0])
	}
}

// Typing narrows the list by subsequence, which is how a palette is reached
// for: the shortest thing that could name the command.
func TestPaletteFilterIsASubsequence(t *testing.T) {
	m, _ := newTestModel(t, 120, 40)
	m = m.openPaletteFor(t)

	m, _ = m.press(t, "s")
	m, _ = m.press(t, "a")

	labels := m.labels()
	if !hasLabel(labels, "stage everything") {
		t.Errorf(`"sa" does not find "stage everything":\n%v`, labels)
	}
	if hasLabel(labels, "quit tuigy") {
		t.Errorf(`"sa" matched something it should not:\n%v`, labels)
	}
}

func TestMatchesCommand(t *testing.T) {
	for _, c := range []struct {
		query, text string
		want        bool
	}{
		{"", "anything", true},
		{"sa", "stage everything stage-all", true},
		{"stage", "stage everything stage-all", true},
		{"as", "stage everything stage-all", true}, // a…s still appears in order
		{"zz", "stage everything stage-all", false},
		{"push", "pull", false},
		{"CHECK", "check out this branch", true}, // case is ignored
		{"go files", "go to the files tree", true},
	} {
		if _, _, got := matchesCommand(c.query, c.text); got != c.want {
			t.Errorf("matchesCommand(%q, %q) = %v, want %v", c.query, c.text, got, c.want)
		}
	}
}

// The closest match leads, or a loose subsequence would bury the obvious answer
// under everything else it happens to reach.
func TestPaletteRanksTheClosestMatchFirst(t *testing.T) {
	m, _ := newTestModel(t, 120, 40)
	m = m.openPaletteFor(t)

	for _, r := range "stage" {
		m, _ = m.press(t, string(r))
	}

	labels := m.labels()
	if len(labels) == 0 {
		t.Fatal(`"stage" matched nothing`)
	}
	if !strings.HasPrefix(labels[0], "stage") {
		t.Errorf("first match is %q, want the one the letters sit together in:\n%v", labels[0], labels)
	}

	// Two equally tight matches are ranked by where they start, which is the
	// difference between the command you typed and one that merely contains it.
	_, early, _ := matchesCommand("stage", "stage everything")
	_, late, _ := matchesCommand("stage", "commit the staged changes")
	if early >= late {
		t.Errorf("match starts at %d, want earlier than %d", early, late)
	}
}

// j and k are letters in a palette, not movement: it is typed at, and a list
// you cannot type "checkout" into is not a palette.
func TestPaletteTypingBeatsNavigation(t *testing.T) {
	m, _ := newTestModel(t, 120, 40)
	m = m.openPaletteFor(t)

	m, _ = m.press(t, "k")
	if m.paletteFilter != "k" {
		t.Errorf("filter = %q, want the letter typed rather than the cursor moved", m.paletteFilter)
	}

	// The arrows are what moves.
	m.paletteFilter = ""
	m.paletteCursor = 0
	m, _ = m.press(t, "down")
	if m.paletteCursor != 1 {
		t.Errorf("cursor = %d after ↓, want 1", m.paletteCursor)
	}
	m, _ = m.press(t, "up")
	if m.paletteCursor != 0 {
		t.Errorf("cursor = %d after ↑, want 0", m.paletteCursor)
	}
}

// Choosing a command does what its key does, which is the only test that
// matters: the palette is a second door onto the same room.
func TestPaletteRunsTheChosenCommand(t *testing.T) {
	m, _ := newTestModel(t, 120, 40)
	m = m.openPaletteFor(t)

	for _, r := range "commit the staged" {
		m, _ = m.press(t, string(r))
	}
	if len(m.matchingCommands()) == 0 {
		t.Fatal("the filter left nothing to run")
	}

	m, _ = m.press(t, "enter")
	if m.modal != modalCommit {
		t.Errorf("modal = %v after running commit, want the commit view", m.modal)
	}
}

// A command that moves to another tab has to arrive with that tab loaded,
// rather than at an empty list the number key would have filled.
func TestPaletteSwitchesTabs(t *testing.T) {
	m, _ := newTestModel(t, 120, 40)
	m = m.openPaletteFor(t)

	for _, r := range "go to the branches" {
		m, _ = m.press(t, string(r))
	}
	next, cmd := m.press(t, "enter")
	m = next.applyCmd(t, cmd)

	if m.tab != tabBranches {
		t.Fatalf("tab = %v, want the branches tab", m.tab)
	}
	if len(m.branchRows) == 0 {
		t.Error("the branches tab was reached without its branches")
	}
}

// The terminal command says what it would do, and closing it is only offered
// when there is something to close.
func TestPaletteTerminalCommandsFollowTheShell(t *testing.T) {
	m, _ := newTestModel(t, 120, 40)

	labels := m.openPaletteFor(t).labels()
	if !hasLabel(labels, "open a terminal") {
		t.Errorf("the terminal command is missing:\n%v", labels)
	}
	if hasLabel(labels, "close the terminal band") {
		t.Errorf("closing is offered with no shell running:\n%v", labels)
	}
}

func TestPaletteEscapeCloses(t *testing.T) {
	m, _ := newTestModel(t, 120, 40)
	m = m.openPaletteFor(t)

	m, _ = m.press(t, "esc")
	if m.modal != modalNone {
		t.Error("esc did not close the palette")
	}
}

// The footer says how to run something and how to leave.
func TestPaletteFooter(t *testing.T) {
	m, _ := newTestModel(t, 120, 40)
	m = m.openPaletteFor(t)

	footer := plain(m.footerView())
	for _, want := range []string{"run it", "esc"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer does not offer %q:\n%s", want, footer)
		}
	}
}

// Every command has to survive being run, including from a tab whose list is
// empty: the palette offers what the key offers, and a key pressed with nothing
// under the cursor does nothing rather than falling over.
//
// Only the command itself is run here. What it returns is a tea.Cmd that is
// deliberately not executed, so nothing fetches, pushes or writes a file.
func TestEveryCommandSurvivesBeingRun(t *testing.T) {
	for _, tab := range allTabs {
		m, _ := newTestModel(t, 120, 40)
		m.tab = tab

		for _, c := range m.commands() {
			// copy is the one command whose effect lands immediately rather
			// than in the tea.Cmd it returns, and a test run has no business
			// overwriting the clipboard of whoever is running it.
			if c.action == "copy" {
				continue
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("tab %d: %q panicked: %v", tab, c.label, r)
					}
				}()
				next, _ := c.run(m)
				if _, ok := next.(Model); !ok {
					t.Errorf("tab %d: %q did not return a model", tab, c.label)
				}
			}()
		}
	}
}
