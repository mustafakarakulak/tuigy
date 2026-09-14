// Package keys holds every keyboard shortcut in one place.
//
// The help screen and the contextual footer hints are generated from these
// bindings, so adding a shortcut requires no changes anywhere else.
package keys

import "github.com/charmbracelet/bubbles/key"

type Map struct {
	Up       key.Binding
	Down     key.Binding
	Top      key.Binding
	Bottom   key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Left     key.Binding
	Right    key.Binding
	NextHunk key.Binding
	PrevHunk key.Binding

	NextPane key.Binding
	PrevPane key.Binding

	TabChanges  key.Binding
	TabBranches key.Binding
	TabHistory  key.Binding
	TabStashes  key.Binding

	Toggle     key.Binding
	Stage      key.Binding
	Unstage    key.Binding
	StageAll   key.Binding
	UnstageAll key.Binding
	Discard    key.Binding
	Review     key.Binding

	Commit   key.Binding
	Amend    key.Binding
	Submit   key.Binding
	Generate key.Binding

	Checkout  key.Binding
	NewBranch key.Binding
	DeleteRef key.Binding
	Merge     key.Binding
	History   key.Binding
	Fetch     key.Binding
	FetchAll  key.Binding
	Pull      key.Binding
	Push      key.Binding

	Pick       key.Binding
	CherryPick key.Binding

	StashPush  key.Binding
	StashPop   key.Binding
	StashApply key.Binding

	Continue   key.Binding
	Abort      key.Binding
	OpenEditor key.Binding
	Copy       key.Binding
	Filter     key.Binding

	Settings key.Binding
	Refresh  key.Binding
	Help     key.Binding
	Cancel   key.Binding
	Confirm  key.Binding
	Quit     key.Binding
}

func Default() Map {
	return Map{
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Top:      key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "first")),
		Bottom:   key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "last")),
		PageUp:   key.NewBinding(key.WithKeys("ctrl+u", "pgup"), key.WithHelp("ctrl+u", "page up")),
		PageDown: key.NewBinding(key.WithKeys("ctrl+d", "pgdown"), key.WithHelp("ctrl+d", "page down")),
		Left:     key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "scroll left")),
		Right:    key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "scroll right")),
		NextHunk: key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next hunk")),
		PrevHunk: key.NewBinding(key.WithKeys("["), key.WithHelp("[", "previous hunk")),

		NextPane: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next pane")),
		PrevPane: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous pane")),

		TabChanges:  key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "changes")),
		TabBranches: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "branches")),
		TabHistory:  key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "history")),
		TabStashes:  key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "stashes")),

		Toggle:     key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "stage/unstage")),
		Stage:      key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stage")),
		Unstage:    key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "unstage")),
		StageAll:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "stage all")),
		UnstageAll: key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "unstage all")),
		Discard:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "discard")),
		Review:     key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "mark reviewed")),

		Commit:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "commit")),
		Amend:    key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "amend last commit")),
		Submit:   key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "confirm")),
		Generate: key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("ctrl+g", "write the message with AI")),

		Checkout:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "switch to branch")),
		NewBranch: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new branch")),
		DeleteRef: key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete branch")),
		Merge:     key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "merge")),
		History:   key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "show history")),
		Fetch:     key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "fetch")),
		FetchAll:  key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "fetch all")),
		Pull:      key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pull")),
		Push:      key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "push")),

		Pick:       key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select commit")),
		CherryPick: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "cherry-pick")),

		StashPush:  key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "stash changes")),
		StashPop:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "pop")),
		StashApply: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "apply, keeping it")),

		Continue:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "continue")),
		Abort:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "abort")),
		OpenEditor: key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "open in your editor")),
		Copy:       key.NewBinding(key.WithKeys("Y"), key.WithHelp("Y", "copy")),
		Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),

		Settings: key.NewBinding(key.WithKeys(","), key.WithHelp(",", "settings")),
		Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Cancel:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		Confirm:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}
