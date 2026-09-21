package ui

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mustafakarakulak/tuigy/internal/git"
)

// The command palette answers two questions at once: what can I do from here,
// and which key does it. The second is why it exists — 0015 spent the last of
// the single-letter keys, and a tool nobody can hold in their head entirely is
// one that needs somewhere to look.
//
// It lists what the keyboard would do from where you are, and nothing else. An
// entry that silently did nothing because the wrong tab was open would be worse
// than no entry at all, so a command belongs either to one tab or to everywhere
// (see docs/decisions/0019-the-palette-offers-what-the-keys-would-do.md).

// paletteCommand is one line of the palette.
//
// It carries its own label rather than reusing the binding's description,
// because what a key does depends on where it is pressed: "delete" removes a
// branch on one tab and drops a stash on another. The view already relabels
// these per tab for the footer, for the same reason.
type paletteCommand struct {
	action string // the key-map action, which is where the key shown comes from
	label  string
	run    func(Model) (tea.Model, tea.Cmd)
}

func (m Model) openPalette() (tea.Model, tea.Cmd) {
	m.err = nil
	m.modal = modalPalette
	m.paletteFilter = ""
	m.paletteCursor, m.paletteOff = 0, 0
	return m, nil
}

// handlePaletteKey drives the list. Every key that is not movement types into
// the filter, because a palette is reached for when you know the word for what
// you want and not the key for it.
//
// Movement is the arrow keys alone, not j and k: those are letters here.
func (m Model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	matches := m.matchingCommands()

	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.modal = modalNone
		return m, nil

	case msg.Type == tea.KeyUp:
		return m.movePaletteCursor(matches, -1), nil

	case msg.Type == tea.KeyDown:
		return m.movePaletteCursor(matches, 1), nil

	case key.Matches(msg, m.keys.PageUp):
		return m.movePaletteCursor(matches, -commandsPerPage), nil

	case key.Matches(msg, m.keys.PageDown):
		return m.movePaletteCursor(matches, commandsPerPage), nil

	case key.Matches(msg, m.keys.Confirm):
		if m.paletteCursor < 0 || m.paletteCursor >= len(matches) {
			return m, nil
		}
		chosen := matches[m.paletteCursor]

		// The palette is gone before the command runs, so that a command
		// opening a dialog of its own is not drawn underneath this one.
		m.modal = modalNone
		return chosen.run(m)
	}

	switch msg.Type {
	case tea.KeyBackspace:
		runes := []rune(m.paletteFilter)
		if len(runes) > 0 {
			m.paletteFilter = string(runes[:len(runes)-1])
		}
	case tea.KeySpace:
		m.paletteFilter += " "
	case tea.KeyRunes:
		m.paletteFilter += string(msg.Runes)
	default:
		return m, nil
	}

	m.paletteCursor, m.paletteOff = 0, 0
	return m, nil
}

func (m Model) movePaletteCursor(matches []paletteCommand, delta int) Model {
	if len(matches) == 0 {
		return m
	}
	m.paletteCursor = clamp(m.paletteCursor+delta, 0, len(matches)-1)
	m.paletteOff = scrollTo(m.paletteCursor, m.paletteOff, commandsPerPage)
	return m
}

// matchingCommands is the list as the filter narrows it, closest match first.
//
// Ordering matters once anything has been typed: a subsequence match is loose
// enough that "st" reaches most of the list, and the one the letters sit
// together in is the one that was meant. With nothing typed there is nothing to
// rank, and the list keeps its order — the tab's own commands first.
func (m Model) matchingCommands() []paletteCommand {
	all := m.commands()
	if m.paletteFilter == "" {
		return all
	}

	type match struct {
		command     paletteCommand
		span, start int
	}

	found := make([]match, 0, len(all))
	for _, c := range all {
		if span, start, ok := matchesCommand(m.paletteFilter, c.label+" "+c.action); ok {
			found = append(found, match{c, span, start})
		}
	}

	// Tightest first, and between two equally tight ones the one that starts
	// earlier: "stage" sits together in both "stage everything" and "commit the
	// staged changes", and only one of them is what was typed.
	//
	// Stable, so commands that tie on both keep the order the list was built in.
	slices.SortStableFunc(found, func(a, b match) int {
		return cmp.Or(a.span-b.span, a.start-b.start)
	})

	out := make([]paletteCommand, 0, len(found))
	for _, f := range found {
		out = append(out, f.command)
	}
	return out
}

// matchesCommand is a subsequence match: every character of the query appears,
// in order, somewhere in the text. "sa" finds "stage all".
//
// It also returns how far apart the matched characters ended up and where they
// began, which is what "closest match" is ranked on.
//
// This is deliberately not the substring match that `/` uses on a list. A list
// filter is typed while looking at the list, so a substring is what the eye is
// already reading; a palette is typed at from memory of the words, where the
// shortest thing that could identify a command is what people reach for.
func matchesCommand(query, text string) (span, start int, ok bool) {
	text = strings.ToLower(text)
	at, first, last := 0, -1, 0

	for _, want := range strings.ToLower(query) {
		if want == ' ' {
			continue // spaces are how the words were separated, not part of them
		}
		found := false
		for ; at < len(text); at++ {
			if rune(text[at]) == want {
				if first < 0 {
					first = at
				}
				last, at, found = at, at+1, true
				break
			}
		}
		if !found {
			return 0, 0, false
		}
	}

	if first < 0 {
		return 0, 0, true // an empty query matches everything, equally
	}
	return last - first, first, true
}

// commands is everything the palette offers from where the interface currently
// is: the tab's own commands first, because they are the ones that needed the
// context, then the ones that work anywhere.
func (m Model) commands() []paletteCommand {
	return append(m.tabCommands(), m.globalCommands()...)
}

func (m Model) tabCommands() []paletteCommand {
	switch m.tab {
	case tabFiles:
		return []paletteCommand{
			{"open-editor", "open this file in your editor", Model.openTreeFile},
			{"expand-all", "open the whole tree", func(m Model) (tea.Model, tea.Cmd) { return m.foldTree(true) }},
			{"collapse-all", "close the whole tree", func(m Model) (tea.Model, tea.Cmd) { return m.foldTree(false) }},
		}

	case tabChanges:
		return []paletteCommand{
			{"commit", "commit the staged changes", Model.openCommit},
			{"generate", "write the commit message with your agent", Model.generateCommitMessage},
			{"amend", "amend the last commit", func(m Model) (tea.Model, tea.Cmd) { return m, m.loadAmendMessage() }},
			{"stage", "stage this file", func(m Model) (tea.Model, tea.Cmd) { return m, m.stage() }},
			{"unstage", "unstage this file", Model.unstageSelection},
			{"stage-all", "stage everything", func(m Model) (tea.Model, tea.Cmd) {
				return m, runOp("staging everything", "staged all changes", m.repo.StageAll)
			}},
			{"unstage-all", "unstage everything", func(m Model) (tea.Model, tea.Cmd) {
				return m, runOp("unstaging everything", "unstaged all changes", m.repo.UnstageAll)
			}},
			{"discard", "discard this file's changes", Model.askDiscard},
			{"review", "mark this file reviewed", Model.toggleReviewed},
			{"open-editor", "open this file in your editor", Model.openInEditor},
		}

	case tabBranches:
		return []paletteCommand{
			{"checkout", "check out this branch", func(m Model) (tea.Model, tea.Cmd) { return m, m.checkout() }},
			{"new-branch", "create a branch", Model.openNewBranch},
			{"merge", "merge a branch", Model.openMerge},
			{"show-history", "show this branch's history", Model.showBranchHistory},
			{"delete", "delete this branch", Model.askDeleteBranch},
		}

	case tabHistory:
		return []paletteCommand{
			{"select-commit", "select this commit", Model.togglePick},
			{"cherry-pick", "cherry-pick the selected commits", Model.openCherryPick},
		}

	case tabStashes:
		return []paletteCommand{
			{"stash-pop", "pop this stash", func(m Model) (tea.Model, tea.Cmd) {
				return m.stashOp("popping", "popped", m.repo.StashPop)
			}},
			{"stash-apply", "apply this stash, keeping it", func(m Model) (tea.Model, tea.Cmd) {
				return m.stashOp("applying", "applied", m.repo.StashApply)
			}},
			{"delete", "drop this stash", Model.askDropStash},
		}
	}

	return nil
}

func (m Model) globalCommands() []paletteCommand {
	cmds := []paletteCommand{
		{"tab-files", "go to the files tree", tabCommand(tabFiles)},
		{"tab-changes", "go to the changes", tabCommand(tabChanges)},
		{"tab-branches", "go to the branches", tabCommand(tabBranches)},
		{"tab-history", "go to the history", tabCommand(tabHistory)},
		{"tab-stashes", "go to the stashes", tabCommand(tabStashes)},

		{"stash-push", "stash the working tree", Model.openStashPush},

		{"fetch", "fetch", func(m Model) (tea.Model, tea.Cmd) {
			return m, runOp("fetching", "fetched", func(ctx context.Context) error {
				return m.repo.Fetch(ctx, false)
			})
		}},
		{"fetch-all", "fetch every remote", func(m Model) (tea.Model, tea.Cmd) {
			return m, runOp("fetching every remote", "fetched all remotes", func(ctx context.Context) error {
				return m.repo.Fetch(ctx, true)
			})
		}},
		{"pull", "pull", func(m Model) (tea.Model, tea.Cmd) {
			return m, runOp("pulling", "pulled", m.repo.Pull)
		}},
		{"push", "push", Model.push},

		{"copy", "copy what the cursor is on", Model.copySelection},
	}

	// An unfinished merge or cherry-pick is the only thing the merge key does
	// from anywhere; on the branches tab it starts one, which is listed there.
	if m.opState != git.OpNone {
		cmds = append(cmds, paletteCommand{
			"merge", "finish or abandon the " + string(m.opState), Model.openOperation,
		})
	}

	cmds = append(cmds, paletteCommand{"terminal", m.terminalLabel(), Model.openTerminal})
	if m.shell != nil {
		cmds = append(cmds, paletteCommand{
			"terminal-close", "close the terminal band", Model.closeTerminalPane,
		})
	}

	return append(cmds,
		paletteCommand{"projects", "switch to another repository", Model.openProjects},
		paletteCommand{"settings", "settings: the theme and the key bindings", Model.openSettings},
		paletteCommand{"refresh", "refresh", Model.refreshEverything},
		paletteCommand{"help", "help", Model.openHelp},
		paletteCommand{"quit", "quit tuigy", func(m Model) (tea.Model, tea.Cmd) { return m, tea.Quit }},
	)
}

// terminalLabel says what the terminal command would do, which is not the same
// thing once a shell is already running in the band.
func (m Model) terminalLabel() string {
	if m.shell == nil {
		return "open a terminal in the repository root"
	}
	return "back to the terminal"
}

// tabCommand is the palette's version of pressing a tab's number.
func tabCommand(t tab) func(Model) (tea.Model, tea.Cmd) {
	return func(m Model) (tea.Model, tea.Cmd) { return m.showTab(t) }
}

// ---------------------------------------------------------------- view

// commandsPerPage is how many commands the palette shows at once, fixed rather
// than derived from the height so the cursor does not jump between terminals.
const commandsPerPage = 12

func (m Model) paletteBox() string {
	matches := m.matchingCommands()

	query := "> " + m.paletteFilter + "▏"
	lines := []string{
		styleTitle.Render("Commands"),
		"",
		styleKey.Render(query) + "  " +
			styleDim.Render(fmt.Sprintf("%d of %d", len(matches), len(m.commands()))),
		"",
	}

	if len(matches) == 0 {
		return strings.Join(append(lines,
			styleDim.Render("nothing matches"),
			"",
			m.paletteFootnote(),
		), "\n")
	}

	// Columns: what it does, and the key that does it. The label column is
	// capped rather than filling the terminal: a wide screen would otherwise
	// push the keys an inch away from the words they belong to.
	strokeW := 12
	labelW := clamp(m.width-10-strokeW-4, 12, 48)

	start := clamp(m.paletteOff, 0, max(len(matches)-commandsPerPage, 0))
	if start > 0 {
		lines = append(lines, styleDim.Render("  ↑ more"))
	} else {
		lines = append(lines, "")
	}

	for i := start; i < min(start+commandsPerPage, len(matches)); i++ {
		c := matches[i]
		text := padRight(clipLine(c.label, labelW), labelW) + padRight(m.commandStroke(c), strokeW)

		if i == m.paletteCursor {
			lines = append(lines, styleSelected.Render("▸ "+text))
			continue
		}
		lines = append(lines, "  "+styleBase.Render(padRight(clipLine(c.label, labelW), labelW))+
			styleKey.Render(padRight(m.commandStroke(c), strokeW)))
	}

	if start+commandsPerPage < len(matches) {
		lines = append(lines, styleDim.Render("  ↓ more"))
	} else {
		lines = append(lines, "")
	}

	return strings.Join(append(lines, "", m.paletteFootnote()), "\n")
}

// commandStroke is the key the command is on, which is the other half of what
// the palette is for: using it once should make it unnecessary next time.
func (m Model) commandStroke(c paletteCommand) string {
	binding, ok := m.keys.Get(c.action)
	if !ok || len(binding.Keys()) == 0 {
		return ""
	}
	return displayStrokes(binding.Keys()[:1])[0]
}

func (m Model) paletteFootnote() string {
	return styleDim.Render("only what works from here is listed; the key beside each one does the same thing")
}

// paletteHints is what the footer offers while the palette is open.
func (m Model) paletteHints() hintSet {
	return hintSet{
		[]key.Binding{m.runCommandBinding()},
		[]key.Binding{m.keys.Cancel},
	}
}

func (m Model) runCommandBinding() key.Binding {
	return key.NewBinding(
		key.WithKeys(m.keys.Confirm.Keys()...),
		key.WithHelp(m.keys.Confirm.Help().Key, "run it"),
	)
}
