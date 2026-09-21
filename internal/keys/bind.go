package keys

import (
	"fmt"
	"slices"

	"github.com/charmbracelet/bubbles/key"
)

// bindings maps the name an action is known by in a config file to the binding
// it sets. Written out rather than derived by reflection so that the names are
// a deliberate, greppable part of the interface.
func (m *Map) bindings() map[string]*key.Binding {
	return map[string]*key.Binding{
		"up":            &m.Up,
		"down":          &m.Down,
		"first":         &m.Top,
		"last":          &m.Bottom,
		"page-up":       &m.PageUp,
		"page-down":     &m.PageDown,
		"scroll-left":   &m.Left,
		"scroll-right":  &m.Right,
		"next-hunk":     &m.NextHunk,
		"previous-hunk": &m.PrevHunk,

		"next-pane":     &m.NextPane,
		"previous-pane": &m.PrevPane,

		"tab-files":    &m.TabFiles,
		"tab-changes":  &m.TabChanges,
		"tab-branches": &m.TabBranches,
		"tab-history":  &m.TabHistory,
		"tab-stashes":  &m.TabStashes,

		"toggle-stage": &m.Toggle,
		"stage":        &m.Stage,
		"unstage":      &m.Unstage,
		"stage-all":    &m.StageAll,
		"unstage-all":  &m.UnstageAll,
		"discard":      &m.Discard,
		"review":       &m.Review,
		"open-editor":  &m.OpenEditor,
		"copy":         &m.Copy,
		"filter":       &m.Filter,

		"commit":   &m.Commit,
		"amend":    &m.Amend,
		"generate": &m.Generate,
		"submit":   &m.Submit,

		"checkout":     &m.Checkout,
		"new-branch":   &m.NewBranch,
		"delete":       &m.DeleteRef,
		"merge":        &m.Merge,
		"show-history": &m.History,

		"select-commit": &m.Pick,
		"cherry-pick":   &m.CherryPick,

		"stash-push":  &m.StashPush,
		"stash-pop":   &m.StashPop,
		"stash-apply": &m.StashApply,

		"continue": &m.Continue,
		"abort":    &m.Abort,

		"fetch":     &m.Fetch,
		"fetch-all": &m.FetchAll,
		"pull":      &m.Pull,
		"push":      &m.Push,

		"expand":           &m.Expand,
		"collapse":         &m.Collapse,
		"expand-subtree":   &m.ExpandSubtree,
		"collapse-subtree": &m.CollapseSubtree,
		"expand-all":       &m.ExpandEverything,
		"collapse-all":     &m.CollapseTree,

		"terminal":       &m.Terminal,
		"terminal-close": &m.TerminalClose,
		"terminal-leave": &m.Detach,

		"projects": &m.Projects,
		"palette":  &m.Palette,

		"settings": &m.Settings,
		"refresh":  &m.Refresh,
		"help":     &m.Help,
		"cancel":   &m.Cancel,
		"confirm":  &m.Confirm,
		"quit":     &m.Quit,
	}
}

// ActionNames lists every action a config file may rebind, sorted.
func ActionNames() []string {
	var m Map
	names := make([]string, 0, len(m.bindings()))
	for name := range m.bindings() {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Get returns the binding an action currently carries, so that a settings
// screen can show what a key does without knowing how the map is built.
func (m *Map) Get(name string) (key.Binding, bool) {
	binding, ok := m.bindings()[name]
	if !ok {
		return key.Binding{}, false
	}
	return *binding, true
}

// Set rebinds one action, which is Apply for the case the settings screen has:
// a single action and the keystroke the user just pressed.
func (m *Map) Set(name string, strokes []string) error {
	return m.Apply(map[string][]string{name: strokes})
}

// Reset puts one action back to the keys it ships with.
func (m *Map) Reset(name string) error {
	defaults := Default()
	original, ok := defaults.Get(name)
	if !ok {
		return fmt.Errorf("unknown action %q", name)
	}

	binding, ok := m.bindings()[name]
	if !ok {
		return fmt.Errorf("unknown action %q", name)
	}
	*binding = original
	return nil
}

// IsDefault reports whether an action still has the keys it ships with.
func (m *Map) IsDefault(name string) bool {
	defaults := Default()
	original, ok := defaults.Get(name)
	if !ok {
		return true
	}
	current, ok := m.Get(name)
	if !ok {
		return true
	}
	return slices.Equal(current.Keys(), original.Keys())
}

// Conflicts lists the other actions a keystroke is already bound to.
//
// Sharing a key is not an error in itself — enter checks out a branch, pops a
// stash and confirms a dialog, because no two of those are ever on screen at
// once — so this reports rather than refuses.
func (m *Map) Conflicts(stroke, exclude string) []string {
	var names []string
	for _, name := range ActionNames() {
		if name == exclude {
			continue
		}
		binding, ok := m.Get(name)
		if !ok {
			continue
		}
		if slices.Contains(binding.Keys(), stroke) {
			names = append(names, name)
		}
	}
	return names
}

// Apply rebinds actions by name, keeping each action's description so that the
// help screen and the footer stay correct for whatever keys were chosen.
func (m *Map) Apply(overrides map[string][]string) error {
	if len(overrides) == 0 {
		return nil
	}

	byName := m.bindings()
	// Applied in a fixed order so that a config with several mistakes always
	// reports the same one first.
	for _, name := range slices.Sorted(mapKeys(overrides)) {
		binding, ok := byName[name]
		if !ok {
			return fmt.Errorf("unknown action %q", name)
		}

		strokes := make([]string, 0, len(overrides[name]))
		for _, s := range overrides[name] {
			switch s {
			case "":
				// nothing to bind
			case "space":
				// The help text calls it "space", so accept it written that way
				// rather than as a lone space nobody can see in a config file.
				strokes = append(strokes, " ")
			default:
				strokes = append(strokes, s)
			}
		}
		if len(strokes) == 0 {
			return fmt.Errorf("action %q was given no keys", name)
		}

		*binding = key.NewBinding(
			key.WithKeys(strokes...),
			key.WithHelp(displayKey(strokes[0]), binding.Help().Desc),
		)
	}

	return nil
}

// displayKey is how a keystroke is written in the help text, where a literal
// space would be invisible.
func displayKey(stroke string) string {
	if stroke == " " {
		return "space"
	}
	return stroke
}

func mapKeys(m map[string][]string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}
