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
