package keys

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestApplyRebindsAnAction(t *testing.T) {
	m := Default()

	if err := m.Apply(map[string][]string{"commit": {"ctrl+k"}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got := m.Commit.Keys(); !slices.Equal(got, []string{"ctrl+k"}) {
		t.Errorf("Commit keys = %v, want [ctrl+k]", got)
	}
	// The description belongs to the action, not the key, so it survives.
	if got := m.Commit.Help(); got.Key != "ctrl+k" || got.Desc != "commit" {
		t.Errorf("Commit help = %+v, want the new key and the original description", got)
	}
	// Untouched actions keep their defaults.
	if got := m.Quit.Keys(); !slices.Equal(got, []string{"q", "ctrl+c"}) {
		t.Errorf("Quit keys = %v, want the default", got)
	}
}

func TestApplyAcceptsSeveralKeys(t *testing.T) {
	m := Default()

	if err := m.Apply(map[string][]string{"up": {"w", "up", "ctrl+p"}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, stroke := range []string{"w", "up", "ctrl+p"} {
		if !key.Matches(fakeKey(stroke), m.Up) {
			t.Errorf("%q does not trigger up", stroke)
		}
	}
	if got := m.Up.Help().Key; got != "w" {
		t.Errorf("help shows %q, want the first key", got)
	}
}

// Space is a real key — it is the default for staging — and it can be written
// either literally or by the name the help text gives it.
func TestApplyBindsTheSpaceKey(t *testing.T) {
	for _, written := range []string{" ", "space"} {
		m := Default()

		if err := m.Apply(map[string][]string{"stage": {written}}); err != nil {
			t.Fatalf("Apply(%q): %v", written, err)
		}
		if got := m.Stage.Keys(); !slices.Equal(got, []string{" "}) {
			t.Errorf("Apply(%q): keys = %q, want a space", written, got)
		}
		if got := m.Stage.Help().Key; got != "space" {
			t.Errorf("Apply(%q): help shows %q, want %q", written, got, "space")
		}
	}
}

func TestApplyRejectsUnknownActions(t *testing.T) {
	m := Default()

	err := m.Apply(map[string][]string{"comit": {"c"}})
	if err == nil {
		t.Fatal("an unknown action should be an error")
	}
	if !strings.Contains(err.Error(), "comit") {
		t.Errorf("error = %q, want it to name the action", err)
	}
}

func TestApplyRejectsEmptyKeys(t *testing.T) {
	m := Default()

	if err := m.Apply(map[string][]string{"commit": {""}}); err == nil {
		t.Error("an action with no usable keys should be an error")
	}
}

// Every action must be reachable by name, or a config file cannot rebind it.
func TestActionNamesCoverEveryBinding(t *testing.T) {
	names := ActionNames()
	if len(names) < 40 {
		t.Errorf("only %d actions are nameable, which looks too few", len(names))
	}
	if !slices.IsSorted(names) {
		t.Error("ActionNames() should be sorted so the listing is stable")
	}

	m := Default()
	for _, name := range names {
		if err := m.Apply(map[string][]string{name: {"ctrl+z"}}); err != nil {
			t.Errorf("Apply(%q): %v", name, err)
		}
	}
}

// fakeKey builds the message bubbletea would send for a keystroke.
func fakeKey(stroke string) stubKey { return stubKey(stroke) }

type stubKey string

func (s stubKey) String() string { return string(s) }

// The settings screen reads and writes one action at a time, which is a
// different shape from the config file's "here is everything at once".
func TestGetSetAndReset(t *testing.T) {
	m := Default()

	original, ok := m.Get("commit")
	if !ok {
		t.Fatal("commit is not a known action")
	}
	if !slices.Contains(original.Keys(), "c") {
		t.Fatalf("commit ships on %v, want c among them", original.Keys())
	}
	if !m.IsDefault("commit") {
		t.Error("an untouched binding does not report itself as the default")
	}

	if err := m.Set("commit", []string{"ctrl+k"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	changed, _ := m.Get("commit")
	if !slices.Equal(changed.Keys(), []string{"ctrl+k"}) {
		t.Errorf("commit = %v after Set, want only ctrl+k", changed.Keys())
	}
	// The description has to survive, or the help screen starts describing the
	// wrong thing.
	if changed.Help().Desc != original.Help().Desc {
		t.Errorf("description = %q, want %q", changed.Help().Desc, original.Help().Desc)
	}
	if m.IsDefault("commit") {
		t.Error("a changed binding still reports itself as the default")
	}

	if err := m.Reset("commit"); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if back, _ := m.Get("commit"); !slices.Equal(back.Keys(), original.Keys()) {
		t.Errorf("commit = %v after Reset, want %v", back.Keys(), original.Keys())
	}
	if !m.IsDefault("commit") {
		t.Error("a reset binding does not report itself as the default")
	}
}

func TestUnknownActionsAreRejected(t *testing.T) {
	m := Default()

	if _, ok := m.Get("teleport"); ok {
		t.Error("an action that does not exist was found")
	}
	if err := m.Set("teleport", []string{"x"}); err == nil {
		t.Error("setting an unknown action should fail")
	}
	if err := m.Reset("teleport"); err == nil {
		t.Error("resetting an unknown action should fail")
	}
	// Nothing that does not exist can be non-default.
	if !m.IsDefault("teleport") {
		t.Error("an unknown action reported itself as changed")
	}
}

// Sharing a key is legal where the two actions are never on screen together,
// so this reports rather than refuses — but it has to report accurately.
func TestConflictsNamesTheOtherActions(t *testing.T) {
	m := Default()

	others := m.Conflicts("enter", "confirm")
	if len(others) == 0 {
		t.Fatal("enter is on several actions but none were reported")
	}
	if slices.Contains(others, "confirm") {
		t.Error("the action being asked about was reported as clashing with itself")
	}
	for _, want := range []string{"checkout", "stash-pop"} {
		if !slices.Contains(others, want) {
			t.Errorf("%q also uses enter but was not reported: %v", want, others)
		}
	}

	if got := m.Conflicts("ctrl+alt+shift+z", ""); len(got) != 0 {
		t.Errorf("a key nothing uses reported %v", got)
	}
}
