package ui

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mustafakarakulak/tuigy/internal/ai"
	"github.com/mustafakarakulak/tuigy/internal/config"
	"github.com/mustafakarakulak/tuigy/internal/git"
)

// amendPrefillMsg carries the last commit message loaded before opening amend.
type amendPrefillMsg struct{ message string }

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+c always quits, even from inside a modal.
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	// While a filter is being typed the list keys are letters, so everything
	// goes to the filter until it is accepted or abandoned.
	if m.filtering {
		return m.handleFilterKey(msg)
	}

	switch m.modal {
	case modalCommit:
		return m.handleCommitKey(msg)
	case modalNewBranch:
		return m.handleNewBranchKey(msg)
	case modalMerge:
		return m.handleMergeKey(msg)
	case modalCherryPick:
		return m.handleCherryPickKey(msg)
	case modalStash:
		return m.handleStashPushKey(msg)
	case modalSettings:
		return m.handleSettingsKey(msg)
	case modalOperation:
		return m.handleOperationKey(msg)
	case modalConfirm:
		return m.handleConfirmKey(msg)
	case modalHelp:
		if key.Matches(msg, m.keys.Cancel, m.keys.Help, m.keys.Quit) {
			m.modal = modalNone
			return m, nil
		}
		var cmd tea.Cmd
		m.help, cmd = m.help.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		return m.openHelp()

	case key.Matches(msg, m.keys.Settings):
		return m.openSettings()

	case key.Matches(msg, m.keys.Refresh):
		m.statusFP = "" // clear the fingerprint so the next result is always applied
		return m, m.reload()

	case key.Matches(msg, m.keys.TabChanges):
		m.clearFilter()
		m.tab = tabChanges
		return m, m.applyFilter()

	case key.Matches(msg, m.keys.TabBranches):
		m.clearFilter()
		m.tab = tabBranches
		return m, m.loadBranches()

	case key.Matches(msg, m.keys.TabHistory):
		m.clearFilter()
		m.tab = tabHistory
		return m, m.showHistory("")

	case key.Matches(msg, m.keys.TabStashes):
		m.clearFilter()
		m.tab = tabStashes
		return m, m.loadStashes()

	case key.Matches(msg, m.keys.Filter):
		// Filtering narrows a list, so it belongs to the list, not the pane
		// showing one item's contents.
		if m.focus == paneList {
			m.filtering = true
			return m, nil
		}

	case key.Matches(msg, m.keys.Cancel):
		// Esc clears a filter that has been accepted; inside the detail pane it
		// still means "go back", which the tab handlers deal with.
		if m.focus == paneList && m.filter != "" {
			m.clearFilter()
			return m, m.applyFilter()
		}

	case key.Matches(msg, m.keys.StashPush):
		return m.openStashPush()

	case key.Matches(msg, m.keys.Fetch):
		return m, runOp("fetching", "fetched", func(ctx context.Context) error {
			return m.repo.Fetch(ctx, false)
		})

	case key.Matches(msg, m.keys.FetchAll):
		return m, runOp("fetching every remote", "fetched all remotes", func(ctx context.Context) error {
			return m.repo.Fetch(ctx, true)
		})

	case key.Matches(msg, m.keys.Copy):
		return m.copySelection()

	case key.Matches(msg, m.keys.Merge):
		// While an operation is unfinished, the only merge question worth
		// asking is what to do about that one.
		if m.opState != git.OpNone {
			return m.openOperation()
		}
		if m.tab == tabBranches {
			return m.openMerge()
		}
		return m, nil

	case key.Matches(msg, m.keys.Pull):
		return m, runOp("pulling", "pulled", m.repo.Pull)

	case key.Matches(msg, m.keys.Push):
		return m.push()
	}

	// The branch list is a single pane; everywhere else tab moves the focus.
	if key.Matches(msg, m.keys.NextPane, m.keys.PrevPane) {
		if m.tab != tabBranches {
			if m.focus == paneList {
				m.focus = paneDetail
			} else {
				m.focus = paneList
			}
		}
		return m, nil
	}

	switch m.tab {
	case tabBranches:
		return m.handleBranchKey(msg)
	case tabHistory:
		return m.handleHistoryKey(msg)
	case tabStashes:
		return m.handleStashKey(msg)
	}

	if m.focus == paneDetail {
		return m.handleDiffKey(msg)
	}
	return m.handleFilesKey(msg)
}

// copySelection puts whatever the cursor is on into the clipboard: the thing
// you would otherwise retype into another terminal.
func (m Model) copySelection() (tea.Model, tea.Cmd) {
	label, value := m.selectionForClipboard()
	if value == "" {
		return m, nil
	}

	if err := clipboard.WriteAll(value); err != nil {
		// On Linux this needs xclip, xsel or wl-copy; say so rather than
		// failing silently.
		m.err = fmt.Errorf("could not copy: %w", err)
		return m, nil
	}

	m.err = nil
	return m, m.setFlash("copied " + label + " " + value)
}

func (m Model) selectionForClipboard() (label, value string) {
	switch m.tab {
	case tabBranches:
		if b, ok := m.selectedBranch(); ok {
			return "branch", b.Name
		}
	case tabHistory:
		if c, ok := m.selectedCommit(); ok {
			return "commit", c.Hash
		}
	case tabStashes:
		if s, ok := m.selectedStash(); ok {
			return "stash", s.Ref
		}
	default:
		if r, ok := m.selected(); ok {
			return "path", r.file.Path
		}
	}
	return "", ""
}

// handleFilterKey edits the filter. It runs before everything else, so an
// ordinary letter narrows the list instead of triggering an action.
func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.clearFilter()
		return m, m.applyFilter()

	case key.Matches(msg, m.keys.Confirm):
		// Accepted: the filter stays, and the keys go back to being actions.
		m.filtering = false
		return m, nil
	}

	switch msg.Type {
	case tea.KeyBackspace:
		runes := []rune(m.filter)
		if len(runes) == 0 {
			return m, nil
		}
		m.filter = string(runes[:len(runes)-1])
		return m, m.applyFilter()

	case tea.KeySpace:
		m.filter += " "
		return m, m.applyFilter()

	case tea.KeyRunes:
		m.filter += string(msg.Runes)
		return m, m.applyFilter()
	}

	return m, nil
}

// ---------------------------------------------------------------- changes tab

// ---------------------------------------------------------------- settings

// openSettings shows the theme picker. Themes are applied as the cursor moves:
// a colour scheme is not something anyone can judge from its name.
func (m Model) openSettings() (tea.Model, tea.Cmd) {
	m.err = nil
	m.themeCursor = max(slices.Index(ThemeNames(), m.theme), 0)
	m.modal = modalSettings
	return m, nil
}

func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	names := ThemeNames()

	switch {
	case key.Matches(msg, m.keys.Cancel):
		// Leaving puts back what the user actually had.
		m.modal = modalNone
		return m, m.previewTheme(m.theme)

	case key.Matches(msg, m.keys.Up):
		m.themeCursor = max(m.themeCursor-1, 0)
		return m, m.previewTheme(names[m.themeCursor])

	case key.Matches(msg, m.keys.Down):
		m.themeCursor = min(m.themeCursor+1, len(names)-1)
		return m, m.previewTheme(names[m.themeCursor])

	case key.Matches(msg, m.keys.Confirm):
		chosen := names[m.themeCursor]
		m.theme = chosen
		m.modal = modalNone

		return m, func() tea.Msg {
			path, err := config.SetTheme(chosen)
			return themeSavedMsg{path: path, err: err}
		}
	}

	return m, nil
}

// previewTheme repaints everything in a theme without recording it as the
// choice, so moving the cursor shows what a theme actually looks like.
func (m Model) previewTheme(name string) tea.Cmd {
	if err := ApplyTheme(name, nil); err != nil {
		return func() tea.Msg { return errMsg{err} }
	}
	return nil
}

func (m Model) openHelp() (tea.Model, tea.Cmd) {
	m.modal = modalHelp
	m.help.SetContent(m.helpContent())
	m.help.GotoTop()
	return m, nil
}

func (m Model) handleDiffKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.focus = paneList
		return m, nil

	// Long lines are not cut to the pane, so they can be scrolled to rather
	// than lost.
	case key.Matches(msg, m.keys.Left):
		m.diff.ScrollLeft(horizontalStep)
		return m, nil

	case key.Matches(msg, m.keys.Right):
		m.diff.ScrollRight(horizontalStep)
		return m, nil

	case key.Matches(msg, m.keys.NextHunk):
		m.moveHunkCursor(1)
		return m, nil

	case key.Matches(msg, m.keys.PrevHunk):
		m.moveHunkCursor(-1)
		return m, nil

	case key.Matches(msg, m.keys.Toggle), key.Matches(msg, m.keys.Stage):
		return m.applyHunk()
	}

	var cmd tea.Cmd
	m.diff, cmd = m.diff.Update(msg)
	return m, cmd
}

// applyHunk moves the hunk under the cursor across, in whichever direction the
// file it belongs to is sitting.
//
// This is the reason a diff is worth navigating rather than only reading: a
// coding agent rarely produces a file whose every change you want in the same
// commit.
func (m Model) applyHunk() (tea.Model, tea.Cmd) {
	r, ok := m.selected()
	if !ok {
		return m, nil
	}

	if r.file.Worktree == git.StatusUntracked {
		m.err = errors.New("this file is not tracked yet; stage the whole file first (space)")
		return m, nil
	}
	if len(m.diffHunks) == 0 || m.diffRaw == "" {
		m.err = errors.New("there is no hunk here to stage")
		return m, nil
	}

	repo, diff, index := m.repo, m.diffRaw, m.hunkCursor
	position := fmt.Sprintf("hunk %d of %d in %s", index+1, len(m.diffHunks), r.file.Path)
	m.err = nil

	if r.staged() {
		return m, runOp("unstaging "+position, "unstaged "+position, func(ctx context.Context) error {
			return repo.UnstageHunk(ctx, diff, index)
		})
	}
	return m, runOp("staging "+position, "staged "+position, func(ctx context.Context) error {
		return repo.StageHunk(ctx, diff, index)
	})
}

// nextHunk is the first hunk below the current position, or the last one when
// there is nothing further down.
func nextHunk(hunks []int, current int) int {
	for _, at := range hunks {
		if at > current {
			return at
		}
	}
	if len(hunks) == 0 {
		return current
	}
	return hunks[len(hunks)-1]
}

func previousHunk(hunks []int, current int) int {
	previous := 0
	for _, at := range hunks {
		if at >= current {
			break
		}
		previous = at
	}
	return previous
}

func (m Model) handleFilesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		return m, m.moveCursor(-1)

	case key.Matches(msg, m.keys.Down):
		return m, m.moveCursor(1)

	case key.Matches(msg, m.keys.Top):
		m.cursor = m.firstFileRow()
		m.ensureVisible()
		return m, m.syncDiff()

	case key.Matches(msg, m.keys.Bottom):
		m.cursor = m.lastFileRow()
		m.ensureVisible()
		return m, m.syncDiff()

	case key.Matches(msg, m.keys.Confirm):
		m.focus = paneDetail
		return m, nil

	case key.Matches(msg, m.keys.Toggle):
		if r, ok := m.selected(); ok && r.staged() {
			return m, m.unstage(r)
		}
		return m, m.stage()

	case key.Matches(msg, m.keys.Stage):
		return m, m.stage()

	case key.Matches(msg, m.keys.Unstage):
		if r, ok := m.selected(); ok {
			return m, m.unstage(r)
		}
		return m, nil

	case key.Matches(msg, m.keys.StageAll):
		return m, runOp("staging everything", "staged all changes", m.repo.StageAll)

	case key.Matches(msg, m.keys.UnstageAll):
		return m, runOp("unstaging everything", "unstaged all changes", m.repo.UnstageAll)

	case key.Matches(msg, m.keys.Discard):
		return m.askDiscard()

	case key.Matches(msg, m.keys.Review):
		return m.toggleReviewed()

	case key.Matches(msg, m.keys.Commit):
		return m.openCommit()

	case key.Matches(msg, m.keys.Generate):
		// Outside the commit view this is a whole action rather than a
		// modifier: open the view and start writing the message.
		next, cmd := m.openCommit()
		opened := next.(Model)
		if opened.modal != modalCommit {
			return opened, cmd
		}

		generating, generate := opened.generateMessage()
		return generating, tea.Batch(cmd, generate)

	case key.Matches(msg, m.keys.Amend):
		return m, m.loadAmendMessage()

	case key.Matches(msg, m.keys.OpenEditor):
		return m.openInEditor()
	}

	return m, nil
}

// openInEditor hands the selected file to the user's editor. Conflicts are
// meant to be resolved outside tuigy, and this is the shortest path there.
func (m Model) openInEditor() (tea.Model, tea.Cmd) {
	r, ok := m.selected()
	if !ok {
		return m, nil
	}
	if r.file.Worktree == git.StatusDeleted || r.file.Index == git.StatusDeleted {
		m.err = errors.New("that file has been deleted")
		return m, nil
	}

	m.err = nil
	return m, openEditor(m.editor, filepath.Join(m.repo.Root, r.file.Path))
}

// openEditor suspends the TUI and gives the terminal to the editor.
//
// The editor setting may carry arguments of its own, so it is run through a
// shell, but the path is passed as a positional argument: a file name holding
// a space or a quote can never turn into shell syntax.
func openEditor(editor, path string) tea.Cmd {
	// editor comes from git, which already consulted core.editor and the usual
	// variables. The fallbacks are for when that lookup was skipped or empty.
	editor = cmp.Or(
		editor,
		os.Getenv("GIT_EDITOR"),
		os.Getenv("VISUAL"),
		os.Getenv("EDITOR"),
		"vi",
	)

	cmd := exec.Command("sh", "-c", editor+` "$1"`, "sh", path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return editorDoneMsg{err: err} })
}

func (m Model) stage() tea.Cmd {
	r, ok := m.selected()
	if !ok {
		return nil
	}

	doing, done := "staging "+r.file.Path, "staged "+r.file.Path
	if r.file.Unmerged {
		// Staging a conflicted file is how git marks it resolved.
		doing, done = "marking "+r.file.Path+" resolved", "marked resolved: "+r.file.Path
	}

	repo, paths := m.repo, r.paths()
	return runOp(doing, done, func(ctx context.Context) error { return repo.Stage(ctx, paths...) })
}

func (m Model) unstage(r row) tea.Cmd {
	repo, paths := m.repo, r.paths()
	return runOp("unstaging "+r.file.Path, "unstaged "+r.file.Path, func(ctx context.Context) error {
		return repo.Unstage(ctx, paths...)
	})
}

// toggleReviewed marks the selected file as looked at, and steps down so that
// working through an agent's changes is one key repeated.
func (m Model) toggleReviewed() (tea.Model, tea.Cmd) {
	r, ok := m.selected()
	if !ok {
		return m, nil
	}

	// Rebuilt rather than mutated: the map is shared with the model this was
	// called on.
	reviewed := maps.Clone(m.reviewed)
	if reviewed == nil {
		reviewed = map[string]fileStamp{}
	}

	if _, already := reviewed[r.file.Path]; already {
		delete(reviewed, r.file.Path)
	} else {
		stamp, err := stampFile(filepath.Join(m.repo.Root, r.file.Path))
		if err != nil {
			m.err = err
			return m, nil
		}
		reviewed[r.file.Path] = stamp
	}

	m.reviewed = reviewed
	m.err = nil
	return m, m.moveCursor(1)
}

func (m Model) askDiscard() (tea.Model, tea.Cmd) {
	r, ok := m.selected()
	if !ok {
		return m, nil
	}
	if r.staged() {
		m.err = errors.New("cannot discard a staged change: unstage it first (u)")
		return m, nil
	}

	detail := r.file.Path
	if r.file.Worktree == git.StatusUntracked {
		detail += "\n\nThis untracked file will be deleted from disk."
	} else {
		detail += "\n\nThe file will be restored to its state at HEAD."
	}

	repo, file := m.repo, r.file
	m.modal = modalConfirm
	m.confirm = confirmation{
		title:  "Discard changes",
		detail: detail + "\nThis cannot be undone.",
		run: func() tea.Cmd {
			return runOp("discarding "+file.Path, "discarded "+file.Path, func(ctx context.Context) error {
				return repo.Discard(ctx, file)
			})
		},
	}
	return m, nil
}

// ---------------------------------------------------------------- branches tab

func (m Model) handleBranchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.moveBranchCursor(-1)
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.moveBranchCursor(1)
		return m, nil

	case key.Matches(msg, m.keys.Top):
		m.branchCur = firstBranchRow(m.branchRows)
		m.ensureBranchVisible()
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		for i := len(m.branchRows) - 1; i >= 0; i-- {
			if !m.branchRows[i].header {
				m.branchCur = i
				break
			}
		}
		m.ensureBranchVisible()
		return m, nil

	case key.Matches(msg, m.keys.Checkout):
		return m, m.checkout()

	case key.Matches(msg, m.keys.NewBranch):
		return m.openNewBranch()

	case key.Matches(msg, m.keys.DeleteRef):
		return m.askDeleteBranch()

	case key.Matches(msg, m.keys.History):
		b, ok := m.selectedBranch()
		if !ok {
			return m, nil
		}
		m.tab = tabHistory
		return m, m.showHistory(b.Name)
	}

	return m, nil
}

// ---------------------------------------------------------------- history tab

func (m Model) handleHistoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.focus == paneDetail {
		if key.Matches(msg, m.keys.Cancel) {
			m.focus = paneList
			return m, nil
		}
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(msg, m.keys.Up):
		return m, m.moveCommitCursor(-1)

	case key.Matches(msg, m.keys.Down):
		return m, m.moveCommitCursor(1)

	case key.Matches(msg, m.keys.PageUp):
		return m, m.moveCommitCursor(-m.listH)

	case key.Matches(msg, m.keys.PageDown):
		return m, m.moveCommitCursor(m.listH)

	case key.Matches(msg, m.keys.Top):
		return m, m.moveCommitCursor(-len(m.commits))

	case key.Matches(msg, m.keys.Bottom):
		return m, m.moveCommitCursor(len(m.commits))

	case key.Matches(msg, m.keys.Confirm):
		m.focus = paneDetail
		return m, nil

	case key.Matches(msg, m.keys.Pick):
		return m.togglePick()

	case key.Matches(msg, m.keys.CherryPick):
		return m.openCherryPick()
	}

	return m, nil
}

// togglePick selects or deselects the commit under the cursor, then steps down
// so that holding space marks a run of commits.
func (m Model) togglePick() (tea.Model, tea.Cmd) {
	c, ok := m.selectedCommit()
	if !ok {
		return m, nil
	}

	// The map is shared with the model this method was called on, so the
	// selection is rebuilt rather than mutated in place.
	picked := maps.Clone(m.picked)
	if picked == nil {
		picked = map[string]bool{}
	}
	if picked[c.Hash] {
		delete(picked, c.Hash)
	} else {
		picked[c.Hash] = true
	}
	m.picked = picked

	return m, m.moveCommitCursor(1)
}

// ---------------------------------------------------------------- stashes tab

func (m Model) handleStashKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.focus == paneDetail {
		if key.Matches(msg, m.keys.Cancel) {
			m.focus = paneList
			return m, nil
		}
		var cmd tea.Cmd
		m.stashView, cmd = m.stashView.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(msg, m.keys.Up):
		return m, m.moveStashCursor(-1)

	case key.Matches(msg, m.keys.Down):
		return m, m.moveStashCursor(1)

	case key.Matches(msg, m.keys.Top):
		return m, m.moveStashCursor(-len(m.stashes))

	case key.Matches(msg, m.keys.Bottom):
		return m, m.moveStashCursor(len(m.stashes))

	case key.Matches(msg, m.keys.StashPop):
		return m.stashOp("popping", "popped", m.repo.StashPop)

	case key.Matches(msg, m.keys.StashApply):
		return m.stashOp("applying", "applied", m.repo.StashApply)

	case key.Matches(msg, m.keys.DeleteRef):
		return m.askDropStash()
	}

	return m, nil
}

func (m Model) stashOp(doing, done string, fn func(context.Context, string) error) (tea.Model, tea.Cmd) {
	s, ok := m.selectedStash()
	if !ok {
		return m, nil
	}

	ref, message := s.Ref, s.Message
	return m, runOp(doing+" "+message, done+" "+message, func(ctx context.Context) error {
		return fn(ctx, ref)
	})
}

func (m Model) askDropStash() (tea.Model, tea.Cmd) {
	s, ok := m.selectedStash()
	if !ok {
		return m, nil
	}

	repo, ref, message := m.repo, s.Ref, s.Message
	m.modal = modalConfirm
	m.confirm = confirmation{
		title: "Drop stash",
		detail: message + "\n\nThe changes it holds are discarded without being applied.\n" +
			"This cannot be undone from tuigy.",
		run: func() tea.Cmd {
			return runOp("dropping "+message, "dropped "+message, func(ctx context.Context) error {
				return repo.StashDrop(ctx, ref)
			})
		},
	}
	return m, nil
}

// openStashPush asks for a message before saving the working tree away.
func (m Model) openStashPush() (tea.Model, tea.Cmd) {
	if m.status == nil {
		return m, nil
	}
	if len(m.status.Staged()) == 0 && len(m.status.Unstaged()) == 0 {
		m.err = errors.New("there is nothing to stash")
		return m, nil
	}

	m.err = nil
	m.nameInput.SetValue("")
	m.modal = modalStash
	return m, m.nameInput.Focus()
}

func (m Model) handleStashPushKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.closeTextInput()
		return m, nil

	case key.Matches(msg, m.keys.Confirm, m.keys.Submit):
		repo, message := m.repo, strings.TrimSpace(m.nameInput.Value())
		m.closeTextInput()
		m.err = nil

		done := "stashed your changes"
		if message != "" {
			done = "stashed: " + message
		}
		return m, runOp("stashing your changes", done, func(ctx context.Context) error {
			return repo.Stash(ctx, message)
		})
	}

	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

// ---------------------------------------------------------------- cherry-pick

func (m Model) openCherryPick() (tea.Model, tea.Cmd) {
	commits := m.pickedCommits()
	if len(commits) == 0 {
		m.err = errors.New("select a commit first")
		return m, nil
	}

	targets := m.localBranchNames("")
	if len(targets) == 0 {
		m.err = errors.New("there is no local branch to apply them to")
		return m, nil
	}

	m.err = nil
	m.pickCommits = commits
	m.targets = targets
	m.targetIndex = 0

	if m.status != nil {
		for i, name := range targets {
			if name == m.status.Branch {
				m.targetIndex = i
				break
			}
		}
	}

	m.modal = modalCherryPick
	return m, nil
}

func (m Model) handleCherryPickKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.closeCherryPick()
		return m, nil

	case key.Matches(msg, m.keys.Up):
		m.targetIndex = max(m.targetIndex-1, 0)
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.targetIndex = min(m.targetIndex+1, len(m.targets)-1)
		return m, nil

	case key.Matches(msg, m.keys.Confirm):
		repo, target := m.repo, m.currentTarget()

		var current string
		if m.status != nil {
			current = m.status.Branch
		}

		// The history lists commits newest first, but they have to be replayed
		// in the order they were written.
		hashes := make([]string, 0, len(m.pickCommits))
		for i := len(m.pickCommits) - 1; i >= 0; i-- {
			hashes = append(hashes, m.pickCommits[i].Hash)
		}

		doing := fmt.Sprintf("cherry-picking %s onto %s", plural(len(hashes), "commit"), target)
		done := fmt.Sprintf("cherry-picked %s onto %s", plural(len(hashes), "commit"), target)
		m.closeCherryPick()
		m.picked = nil
		m.err = nil

		return m, runOp(doing, done, func(ctx context.Context) error {
			return repo.CherryPickInto(ctx, target, current, hashes...)
		})
	}
	return m, nil
}

func (m *Model) closeCherryPick() {
	m.modal = modalNone
	m.pickCommits = nil
	m.targets = nil
	m.targetIndex = 0
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func (m Model) checkout() tea.Cmd {
	b, ok := m.selectedBranch()
	if !ok || b.Current {
		return nil
	}

	repo, name, remote := m.repo, b.Name, b.Remote
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
		defer cancel()

		var err error
		if remote {
			err = repo.CheckoutRemote(ctx, name)
		} else {
			err = repo.Checkout(ctx, name)
		}
		if err != nil {
			return checkoutFailedMsg{branch: name, err: err}
		}
		return opDoneMsg{label: "switched to " + name}
	}
}

// offerStashAndSwitch turns a blocked branch switch into a choice.
//
// Stashing is never done on the user's behalf without asking: a silent stash
// makes work disappear from the working tree with no obvious way back.
func (m Model) offerStashAndSwitch(msg checkoutFailedMsg) (tea.Model, tea.Cmd) {
	// Only offer a stash when local changes could plausibly be the cause.
	if m.status == nil || m.status.IsClean() {
		m.err = msg.err
		return m, nil
	}

	// The dialog repeats the error, so the footer does not need to as well.
	m.err = nil

	repo, branch := m.repo, msg.branch
	m.modal = modalConfirm
	m.confirm = confirmation{
		title:  "Switch blocked by local changes",
		detail: firstLine(msg.err.Error()) + "\n\nStash the changes and switch to " + branch + "?",
		run: func() tea.Cmd {
			return runOp("stashing, then switching to "+branch, "stashed, then switched to "+branch, func(ctx context.Context) error {
				if err := repo.Stash(ctx, "tuigy: switching to "+branch); err != nil {
					return err
				}
				return repo.Checkout(ctx, branch)
			})
		},
	}
	return m, nil
}

func (m Model) askDeleteBranch() (tea.Model, tea.Cmd) {
	b, ok := m.selectedBranch()
	if !ok {
		return m, nil
	}
	switch {
	case b.Remote:
		m.err = errors.New("tuigy does not delete remote branches")
		return m, nil
	case b.Current:
		m.err = errors.New("cannot delete the branch you are on")
		return m, nil
	}

	repo, name := m.repo, b.Name
	m.modal = modalConfirm
	m.confirm = confirmation{
		title: "Delete branch",
		detail: name + "\n\nOnly the local branch is removed. git refuses if it still\n" +
			"holds commits that are not merged anywhere else.",
		run: func() tea.Cmd {
			return runOp("deleting "+name, "deleted "+name, func(ctx context.Context) error {
				return repo.DeleteBranch(ctx, name, false)
			})
		},
	}
	return m, nil
}

// ---------------------------------------------------------------- merge

func (m Model) openMerge() (tea.Model, tea.Cmd) {
	b, ok := m.selectedBranch()
	if !ok {
		return m, nil
	}

	targets := m.localBranchNames(b.Name)
	if len(targets) == 0 {
		m.err = errors.New("there is no other local branch to merge into")
		return m, nil
	}

	m.err = nil
	m.mergeSource = b.Name
	m.targets = targets
	m.targetIndex = 0
	m.mergePlan = nil

	// Merging into the branch you are on is by far the common case.
	if m.status != nil {
		for i, name := range targets {
			if name == m.status.Branch {
				m.targetIndex = i
				break
			}
		}
	}

	m.modal = modalMerge
	return m, m.planMerge()
}

// planMerge works out what the merge would do, so the dialog can say it.
func (m Model) planMerge() tea.Cmd {
	repo := m.repo
	source, target := m.mergeSource, m.currentTarget()

	var current string
	if m.status != nil {
		current = m.status.Branch
	}

	return func() tea.Msg {
		plan := mergePlan{source: source, target: target, intoCurrent: target == current}
		if !plan.intoCurrent {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			ff, err := repo.CanFastForward(ctx, source, target)
			if err != nil {
				return errMsg{err}
			}
			plan.fastForward = ff
		}
		return mergePlanMsg{plan: plan}
	}
}

func (m Model) handleMergeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.closeMerge()
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.targetIndex == 0 {
			return m, nil
		}
		m.targetIndex--
		m.mergePlan = nil
		return m, m.planMerge()

	case key.Matches(msg, m.keys.Down):
		if m.targetIndex >= len(m.targets)-1 {
			return m, nil
		}
		m.targetIndex++
		m.mergePlan = nil
		return m, m.planMerge()

	case key.Matches(msg, m.keys.Confirm):
		repo, source, target := m.repo, m.mergeSource, m.currentTarget()

		var current string
		if m.status != nil {
			current = m.status.Branch
		}

		m.closeMerge()
		m.err = nil

		return m, runOp(fmt.Sprintf("merging %s into %s", source, target), fmt.Sprintf("merged %s into %s", source, target), func(ctx context.Context) error {
			return repo.MergeInto(ctx, source, target, current)
		})
	}
	return m, nil
}

func (m *Model) closeMerge() {
	m.modal = modalNone
	m.mergeSource = ""
	m.targets = nil
	m.targetIndex = 0
	m.mergePlan = nil
}

// ---------------------------------------------------------------- in-progress operation

func (m Model) openOperation() (tea.Model, tea.Cmd) {
	if m.opState == git.OpNone {
		return m, nil
	}
	m.err = nil
	m.modal = modalOperation
	return m, nil
}

func (m Model) handleOperationKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.modal = modalNone
		return m, nil

	case key.Matches(msg, m.keys.Continue):
		state := m.opState
		m.modal = modalNone
		return m, runOp("finishing the "+string(state), string(state)+" completed", m.repo.ContinueOperation)

	case key.Matches(msg, m.keys.Abort):
		state := m.opState
		m.modal = modalNone
		return m, runOp("aborting the "+string(state), string(state)+" aborted", m.repo.AbortOperation)
	}
	return m, nil
}

func (m Model) openNewBranch() (tea.Model, tea.Cmd) {
	from := m.selectedBranchName()
	if from == "" && m.status != nil {
		from = m.status.Branch
	}

	m.err = nil
	m.branchFrom = from
	m.nameInput.SetValue("")
	m.modal = modalNewBranch
	return m, m.nameInput.Focus()
}

func (m Model) handleNewBranchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.closeNewBranch()
		return m, nil

	case key.Matches(msg, m.keys.Confirm, m.keys.Submit):
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			m.err = errors.New("branch name cannot be empty")
			return m, nil
		}

		repo, from := m.repo, m.branchFrom
		m.closeNewBranch()
		m.err = nil

		return m, runOp("creating "+name, "created "+name, func(ctx context.Context) error {
			return repo.CreateBranch(ctx, name, from)
		})
	}

	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

func (m *Model) closeNewBranch() {
	m.closeTextInput()
	m.branchFrom = ""
}

func (m *Model) closeTextInput() {
	m.modal = modalNone
	m.nameInput.Blur()
	m.nameInput.SetValue("")
}

// ---------------------------------------------------------------- remote

func (m Model) push() (tea.Model, tea.Cmd) {
	if m.status == nil {
		return m, nil
	}
	if m.status.Branch == "" {
		m.err = errors.New("cannot push while HEAD is detached")
		return m, nil
	}

	repo, branch, upstream := m.repo, m.status.Branch, m.status.Upstream
	done := "pushed " + branch
	if upstream == "" {
		done = "pushed " + branch + " and set its upstream"
	}

	return m, runOp("pushing "+branch, done, func(ctx context.Context) error {
		return repo.Push(ctx, branch, upstream)
	})
}

// ---------------------------------------------------------------- commit

func (m Model) openCommit() (tea.Model, tea.Cmd) {
	if m.status == nil || len(m.status.Staged()) == 0 {
		m.err = git.ErrNothingStaged
		return m, nil
	}
	if len(m.status.Conflicted()) > 0 {
		m.err = errors.New("resolve the conflicts first: some files are still unmerged")
		return m, nil
	}

	m.err = nil
	m.amending = false
	m.modal = modalCommit
	m.commit.SetValue("")
	return m, m.commit.Focus()
}

// loadAmendMessage reads the last commit message and opens the amend view.
// The read happens in the background; the UI never waits on a git call.
func (m Model) loadAmendMessage() tea.Cmd {
	repo := m.repo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		msg, err := repo.LastCommitMessage(ctx)
		if err != nil {
			return errMsg{err}
		}
		return amendPrefillMsg{message: msg}
	}
}

func (m Model) handleCommitKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.closeCommit()
		return m, nil

	case key.Matches(msg, m.keys.Generate):
		return m.generateMessage()

	case key.Matches(msg, m.keys.Submit):
		text := strings.TrimSpace(m.commit.Value())
		if text == "" {
			m.err = errors.New("commit message cannot be empty")
			return m, nil
		}

		repo, amend := m.repo, m.amending
		m.closeCommit()
		m.err = nil

		if amend {
			return m, runOp("amending the last commit", "amended the last commit", func(ctx context.Context) error {
				return repo.Amend(ctx, text)
			})
		}
		return m, runOp("committing", "created commit", func(ctx context.Context) error {
			return repo.Commit(ctx, text)
		})
	}

	var cmd tea.Cmd
	m.commit, cmd = m.commit.Update(msg)
	return m, cmd
}

// generateMessage asks the configured agent to describe the staged changes.
//
// The result lands in the editable field rather than being committed: the
// message is a draft the user still has to read and accept.
func (m Model) generateMessage() (Model, tea.Cmd) {
	if m.ai == nil {
		m.err = errors.New("no coding agent found; set " + ai.CommandEnv + " to a command that writes commit messages")
		return m, nil
	}
	if m.generating {
		return m, nil
	}

	m.err = nil
	m.generating = true
	m.busy = "asking " + m.ai.Name() + " for a message"

	repo, generator := m.repo, m.ai
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		diff, err := repo.StagedDiff(ctx)
		if err != nil {
			return aiMessageMsg{err: err}
		}

		message, err := generator.CommitMessage(ctx, diff)
		return aiMessageMsg{message: message, err: err}
	})
}

func (m *Model) closeCommit() {
	m.modal = modalNone
	m.amending = false
	m.generating = false
	m.commit.Blur()
	m.commit.SetValue("")
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Confirm):
		run := m.confirm.run
		m.modal = modalNone
		m.confirm = confirmation{}
		if run == nil {
			return m, nil
		}
		return m, run()

	case key.Matches(msg, m.keys.Cancel, m.keys.Quit):
		m.modal = modalNone
		m.confirm = confirmation{}
		return m, nil
	}
	return m, nil
}

// paths are the paths handed to git. A renamed file needs both the new and the
// old path, otherwise unstaging only half-applies.
func (r row) paths() []string {
	if r.file.OrigPath != "" {
		return []string{r.file.Path, r.file.OrigPath}
	}
	return []string{r.file.Path}
}
