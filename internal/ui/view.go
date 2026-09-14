package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/mustafakarakulak/tuigy/internal/config"
	"github.com/mustafakarakulak/tuigy/internal/editor"
	"github.com/mustafakarakulak/tuigy/internal/git"
)

func (m Model) View() string {
	if !m.ready {
		return "loading tuigy…"
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.headerView(),
		m.bodyView(),
		m.footerView(),
	)
}

// ---------------------------------------------------------------- header

func (m Model) headerView() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		m.repoLine(),
		m.tabsView(),
		m.rule(),
	)
}

func (m Model) repoLine() string {
	if m.status == nil {
		return styleDim.Render("loading…")
	}

	parts := []string{styleRepoName.Render(m.repo.Name())}

	if m.status.Detached {
		parts = append(parts, styleBranch.Render("detached @ "+m.status.Head))
	} else {
		parts = append(parts, styleBranch.Render(m.status.Branch))
	}

	switch {
	case m.status.Detached:
		// upstream is meaningless while HEAD is detached
	case m.status.Upstream == "":
		parts = append(parts, styleDim.Render("no upstream"))
	default:
		if m.status.Ahead > 0 {
			parts = append(parts, styleAhead.Render(fmt.Sprintf("↑%d", m.status.Ahead)))
		}
		if m.status.Behind > 0 {
			parts = append(parts, styleBehind.Render(fmt.Sprintf("↓%d", m.status.Behind)))
		}
		if m.status.Ahead == 0 && m.status.Behind == 0 {
			parts = append(parts, styleDim.Render("up to date"))
		}
	}

	if n := len(m.status.Files); n > 0 {
		parts = append(parts, styleDirty.Render(fmt.Sprintf("● %d changed", n)))
	} else {
		parts = append(parts, styleDim.Render("clean"))
	}

	if m.opState != git.OpNone {
		parts = append(parts, styleOpState.Render(strings.ToUpper(string(m.opState))))
	}

	return clipStyled(strings.Join(parts, "  "), m.width)
}

func (m Model) tabsView() string {
	line := m.renderTabs(true)
	// Four labelled tabs do not fit a narrow terminal; fall back to numbers
	// with only the active tab named.
	if lipgloss.Width(line) > m.width {
		line = m.renderTabs(false)
	}

	if suffix := m.tabSuffix(); suffix != "" &&
		lipgloss.Width(line)+lipgloss.Width(suffix)+2 <= m.width {
		line += "  " + suffix
	}

	return clipStyled(line, m.width)
}

func (m Model) renderTabs(labelled bool) string {
	parts := make([]string, 0, len(allTabs))
	for _, t := range allTabs {
		label := fmt.Sprintf("%d", int(t)+1)
		if labelled || t == m.tab {
			label += " " + t.title()
		}

		style := styleTab
		if t == m.tab {
			style = styleTabActive
		}
		parts = append(parts, style.Render(label))
	}
	return strings.Join(parts, " ")
}

// tabSuffix carries what the active tab needs shown at all times: the filter,
// which ref the history is following, and any cherry-pick selection.
func (m Model) tabSuffix() string {
	var parts []string

	if m.tab == tabChanges {
		if done, total := m.reviewProgress(); done > 0 {
			parts = append(parts, styleAdded.Render(fmt.Sprintf("✓ %d/%d reviewed", done, total)))
		}
	}

	if m.tab == tabHistory {
		if m.historyRef != "" {
			parts = append(parts, styleDim.Render(m.historyRef))
		}
		if n := len(m.picked); n > 0 {
			parts = append(parts, styleFlash.Render(plural(n, "commit")+" selected"))
		}
	}

	if m.filtering || m.filter != "" {
		query := "/" + m.filter
		if m.filtering {
			query += "▏"
		}
		shown, total := m.filteredCount()
		if !m.filtering && total > shown {
			query += fmt.Sprintf("  %d of %d", shown, total)
		}
		parts = append(parts, styleKey.Render(query))
	}

	return strings.Join(parts, "  ")
}

func (m Model) rule() string {
	return styleDim.Render(strings.Repeat("─", max(m.width, 0)))
}

// ---------------------------------------------------------------- body

func (m Model) bodyView() string {
	bodyH := max(m.height-headerHeight-footerHeight, 3)

	switch m.modal {
	case modalCommit:
		return m.center(m.commitBox(), bodyH)
	case modalNewBranch:
		return m.center(m.newBranchBox(), bodyH)
	case modalMerge:
		return m.center(m.mergeBox(), bodyH)
	case modalCherryPick:
		return m.center(m.cherryPickBox(), bodyH)
	case modalStash:
		return m.center(m.stashBox(), bodyH)
	case modalSettings:
		return m.center(m.settingsBox(), bodyH)
	case modalOperation:
		return m.center(m.operationBox(), bodyH)
	case modalConfirm:
		return m.center(m.confirmBox(), bodyH)
	case modalHelp:
		return m.center(m.helpBox(), bodyH)
	}

	switch m.tab {
	case tabBranches:
		return m.branchesBody()
	case tabHistory:
		return m.historyBody()
	case tabStashes:
		return m.stashesBody()
	default:
		return m.changesBody()
	}
}

func (m Model) stashesBody() string {
	list := paneStyle(m.focus == paneList).
		Width(m.listW).Height(m.listH).
		Render(fitPane(
			renderStashList(m.stashes, m.stashCursor, m.stashOff, m.listW, m.listH),
			m.listW, m.listH))

	detail := paneStyle(m.focus == paneDetail).
		Width(m.diffW).Height(m.diffH).
		Render(fitPane(m.stashView.View(), m.diffW, m.diffH))

	return lipgloss.JoinHorizontal(lipgloss.Top, list, detail)
}

func (m Model) historyBody() string {
	list := paneStyle(m.focus == paneList).
		Width(m.listW).Height(m.listH).
		Render(fitPane(
			renderCommitList(m.commits, m.commitCursor, m.commitOff, m.picked, m.listW, m.listH),
			m.listW, m.listH))

	detail := paneStyle(m.focus == paneDetail).
		Width(m.diffW).Height(m.diffH).
		Render(fitPane(m.detail.View(), m.diffW, m.diffH))

	return lipgloss.JoinHorizontal(lipgloss.Top, list, detail)
}

func (m Model) changesBody() string {
	list := paneStyle(m.focus == paneList).
		Width(m.listW).Height(m.listH).
		Render(fitPane(renderList(m.rows, m.cursor, m.listOff, m.listW, m.listH, m.reviewed), m.listW, m.listH))

	diff := paneStyle(m.focus == paneDetail).
		Width(m.diffW).Height(m.diffH).
		Render(fitPane(m.diff.View(), m.diffW, m.diffH))

	return lipgloss.JoinHorizontal(lipgloss.Top, list, diff)
}

func (m Model) branchesBody() string {
	list := paneStyle(true).
		Width(m.listW).Height(m.listH).
		Render(fitPane(renderBranchList(m.branchRows, m.branchCur, m.branchOff, m.listW, m.listH), m.listW, m.listH))

	detail := styleDim.Render("no branch selected")
	if b, ok := m.selectedBranch(); ok {
		detail = renderBranchDetail(b, m.diffW)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		list,
		paneStyle(false).Width(m.diffW).Height(m.diffH).
			Render(fitPane(detail, m.diffW, m.diffH)),
	)
}

// center places a dialog in the body area, trimmed to what actually fits.
// styleModal costs 2 columns and 2 rows of border plus 4 columns and 2 rows of padding.
func (m Model) center(content string, height int) string {
	box := styleModal.Render(fitPane(content, max(m.width-6, 8), max(height-4, 1)))
	return lipgloss.Place(m.width, height, lipgloss.Center, lipgloss.Center, box)
}

// ---------------------------------------------------------------- modals

func (m Model) commitBox() string {
	title, hint := "Commit", "a new commit from the staged changes"
	if m.amending {
		title, hint = "Amend last commit", "the HEAD commit will be rewritten"
	}

	var summary string
	if m.status != nil {
		staged := m.status.Staged()
		names := make([]string, 0, 3)
		for _, f := range staged[:min(len(staged), 3)] {
			names = append(names, f.Path)
		}
		summary = fmt.Sprintf("%d file(s) staged", len(staged))
		if len(names) > 0 {
			summary += ": " + strings.Join(names, ", ")
		}
		if len(staged) > len(names) {
			summary += fmt.Sprintf(" and %d more", len(staged)-len(names))
		}
	}

	lines := []string{
		styleTitle.Render(title),
		styleDim.Render(hint),
		"",
		styleDim.Render(summary),
		"",
		m.commit.View(),
	}

	// Staging something you have not read is exactly what this tool exists to
	// prevent, so it is said here rather than left to be noticed.
	if n := m.unreviewedStaged(); n > 0 {
		lines = append(lines, styleDirty.Render(fmt.Sprintf("%s staged but not reviewed (v marks one)", plural(n, "file"))))
	}

	switch {
	case m.generating:
		lines = append(lines, "",
			m.spinner.View()+styleDim.Render("asking "+m.ai.Name()+" to describe the change…"))
	case m.ai != nil:
		lines = append(lines, "",
			styleKey.Render("ctrl+g")+styleDim.Render(" writes the message with "+m.ai.Name()))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) newBranchBox() string {
	from := m.branchFrom
	if from == "" {
		from = "current HEAD"
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		styleTitle.Render("New branch"),
		styleDim.Render("created from "+from+" and checked out"),
		"",
		styleDim.Render("name"),
		m.nameInput.View(),
	)
}

func (m Model) mergeBox() string {
	lines := []string{
		styleTitle.Render("Merge"),
		"",
		styleDim.Render(padRight("source", 9)) + styleBranch.Render(m.mergeSource),
		"",
		styleDim.Render("into"),
	}

	lines = append(lines, m.targetPicker(13)...)

	lines = append(lines, "", styleTitle.Render(m.mergeSource+"  →  "+m.currentTarget()))
	if m.mergePlan == nil {
		lines = append(lines, styleDim.Render("working out what this will do…"))
	} else {
		lines = append(lines, styleDim.Render(m.mergePlan.summary()))
	}

	return strings.Join(lines, "\n")
}

func (m Model) cherryPickBox() string {
	count := plural(len(m.pickCommits), "commit")

	lines := []string{
		styleTitle.Render("Cherry-pick"),
		"",
		styleDim.Render(count + ", in the order they will be applied:"),
	}

	// Listed oldest first, which is how they are replayed.
	const shown = 5
	for i := len(m.pickCommits) - 1; i >= max(len(m.pickCommits)-shown, 0); i-- {
		c := m.pickCommits[i]
		lines = append(lines, "  "+styleHunk.Render(c.Short)+" "+
			styleBase.Render(truncateRight(c.Subject, max(m.width/2, 20))))
	}
	if len(m.pickCommits) > shown {
		lines = append(lines, styleDim.Render(fmt.Sprintf("  and %d more", len(m.pickCommits)-shown)))
	}

	lines = append(lines, "", styleDim.Render("onto"))
	lines = append(lines, m.targetPicker(20)...)

	lines = append(lines, "", styleTitle.Render(count+"  →  "+m.currentTarget()))
	if m.status != nil && m.currentTarget() == m.status.Branch {
		lines = append(lines, styleDim.Render("applied to the branch you are on"))
	} else {
		lines = append(lines, styleDim.Render(m.currentTarget()+" will be checked out first, and you will be left on it"))
	}

	return strings.Join(lines, "\n")
}

// targetPicker renders the branch chooser the merge and cherry-pick dialogs
// share, windowed so a long branch list cannot push the summary underneath it
// off the screen. reserve is how many rows the rest of the dialog needs.
func (m Model) targetPicker(reserve int) []string {
	if len(m.targets) == 0 {
		return []string{styleDim.Render("   no branches")}
	}

	bodyH := max(m.height-headerHeight-footerHeight, 3)
	visible := clamp(bodyH-reserve, 3, len(m.targets))
	start := clamp(m.targetIndex-visible/2, 0, max(len(m.targets)-visible, 0))

	var lines []string
	if start > 0 {
		lines = append(lines, styleDim.Render("   ↑ more"))
	}
	for i := start; i < min(start+visible, len(m.targets)); i++ {
		if i == m.targetIndex {
			lines = append(lines, styleSelected.Render(" ▸ "+m.targets[i]+" "))
			continue
		}
		lines = append(lines, "   "+styleBase.Render(m.targets[i]))
	}
	if start+visible < len(m.targets) {
		lines = append(lines, styleDim.Render("   ↓ more"))
	}
	return lines
}

// operationBox explains a merge or cherry-pick that stopped partway, and is
// itself the confirmation for aborting: it spells out what abort throws away.
func (m Model) operationBox() string {
	lines := []string{
		styleTitle.Render(strings.ToUpper(string(m.opState)) + " in progress"),
		"",
	}

	var conflicted []git.FileStatus
	if m.status != nil {
		conflicted = m.status.Conflicted()
	}

	if len(conflicted) == 0 {
		lines = append(lines, styleAdded.Render("Every conflict is resolved and staged."))
	} else {
		lines = append(lines, styleDirty.Render(fmt.Sprintf("%d file(s) still conflicted:", len(conflicted))))
		for _, f := range conflicted[:min(len(conflicted), 6)] {
			lines = append(lines, "  "+styleRemoved.Render(f.Path))
		}
		if len(conflicted) > 6 {
			lines = append(lines, styleDim.Render(fmt.Sprintf("  and %d more", len(conflicted)-6)))
		}
		where := "your editor"
		if name := editor.Name(m.editor); name != "" {
			where = name
		}
		lines = append(lines, "",
			styleDim.Render("Resolve each one in "+where+" (e), then stage it (space)."))
	}

	return strings.Join(append(lines,
		"",
		styleKey.Render(padRight("enter", 7))+styleDesc.Render("continue, committing the result"),
		styleKey.Render(padRight("a", 7))+styleDesc.Render("abort, discarding the "+string(m.opState)+" and any resolutions"),
	), "\n")
}

func (m Model) stashBox() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		styleTitle.Render("Stash changes"),
		styleDim.Render("your working tree is saved away and left clean"),
		"",
		styleDim.Render("untracked files stay where they are"),
		"",
		styleDim.Render("message (optional)"),
		m.nameInput.View(),
	)
}

// settingsBox is the answer to "where do I change how this looks". The theme is
// picked here because it is the one setting worth seeing before choosing; the
// rest is named so that the file stops being something you have to know about.
func (m Model) settingsBox() string {
	lines := []string{
		styleTitle.Render("Settings"),
		"",
		styleDim.Render("theme"),
	}

	for i, name := range ThemeNames() {
		if i == m.themeCursor {
			lines = append(lines, styleSelected.Render(" ▸ "+name+" "))
			continue
		}
		lines = append(lines, "   "+styleBase.Render(name))
	}

	path := "~/.config/tuigy/config.yml"
	if p, err := config.Path(); err == nil {
		path = config.ShortPath(p)
	}

	// The path can be long, so it is shortened from the front: the file name is
	// the part that has to stay readable.
	width := max(m.width-10, 20)

	return strings.Join(append(lines,
		"",
		styleDim.Render("Everything is repainted as you move, so you can see each one."),
		"",
		styleSection.Render("EVERYTHING ELSE"),
		styleDim.Render("Individual colours, key bindings, the editor and the commit"),
		styleDim.Render("message agent live in this file:"),
		"  "+styleBase.Render(truncateLeft(path, width)),
		styleDim.Render("Run tuigy --init-config to write a documented one."),
	), "\n")
}

func (m Model) confirmBox() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		styleTitle.Render(m.confirm.title),
		"",
		styleBase.Render(m.confirm.detail),
	)
}

func (m Model) helpBox() string { return m.help.View() }

// helpContent lists every shortcut, grouped, generated from the keymap so that
// adding a binding cannot leave the help screen out of date.
func (m Model) helpContent() string {
	type group struct {
		title string
		binds []key.Binding
	}

	left := []group{
		{"Navigate", []key.Binding{m.keys.Up, m.keys.Down, m.keys.Top, m.keys.Bottom, m.keys.NextPane}},
		{"Tabs", []key.Binding{m.keys.TabChanges, m.keys.TabBranches, m.keys.TabHistory, m.keys.TabStashes}},
		{"Changes", []key.Binding{m.keys.Toggle, m.keys.Stage, m.keys.Unstage, m.keys.StageAll, m.keys.UnstageAll, m.keys.Review, m.keys.Discard, m.editorBinding()}},
		{"Any list", []key.Binding{m.keys.Filter, m.keys.Copy}},
	}
	right := []group{
		{"Branches", []key.Binding{m.keys.Checkout, m.keys.NewBranch, m.keys.DeleteRef, m.keys.Merge, m.keys.History}},
		{"History", []key.Binding{m.keys.Pick, m.keys.CherryPick}},
		{"Stashes", []key.Binding{m.keys.StashPush, m.keys.StashPop, m.keys.StashApply, m.dropStashBinding()}},
		{"Remote", []key.Binding{m.keys.Fetch, m.keys.FetchAll, m.keys.Pull, m.keys.Push}},
		{"Conflicts", []key.Binding{m.keys.Merge, m.keys.Continue, m.keys.Abort}},
		{"Commit and general", []key.Binding{m.keys.Commit, m.keys.Amend, m.generateBinding(), m.keys.Submit, m.keys.Settings, m.keys.Refresh, m.keys.Help, m.keys.Quit}},
	}

	render := func(groups []group) string {
		var lines []string
		for i, g := range groups {
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, styleSection.Render(g.title))
			for _, b := range g.binds {
				h := b.Help()
				lines = append(lines, "  "+styleKey.Render(padRight(h.Key, 10))+styleDesc.Render(h.Desc))
			}
		}
		return strings.Join(lines, "\n")
	}

	columns := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().MarginRight(4).Render(render(left)),
		render(right),
	)

	lines := []string{styleTitle.Render("Shortcuts"), "", columns}

	lines = append(lines, "",
		styleDim.Render("press ")+styleKey.Render(",")+
			styleDim.Render(" to change the theme, or tuigy --init-config for everything else"))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// ---------------------------------------------------------------- footer

func (m Model) footerView() string {
	if m.err != nil {
		return styleError.Render("✗ " + clipLine(firstLine(m.err.Error()), max(m.width-4, 8)))
	}

	// A running operation occupies the same place its result will, so the
	// spinner becomes the confirmation without anything moving.
	status := m.statusNote()
	if status == "" {
		return renderHints(m.hints(), m.width)
	}

	hints := renderHints(m.hints(), m.width-lipgloss.Width(status)-2)
	gap := m.width - lipgloss.Width(hints) - lipgloss.Width(status)
	if gap < 1 {
		return hints
	}
	return hints + strings.Repeat(" ", gap) + status
}

// statusNote is what is happening, or what just happened.
func (m Model) statusNote() string {
	width := max(m.width/2, 8)

	if m.busy != "" {
		return m.spinner.View() + styleDim.Render(clipLine(m.busy, width)+"…")
	}
	if m.flash != "" {
		return styleFlash.Render("✓ " + clipLine(m.flash, width))
	}
	return ""
}

// hintSet is what the footer offers. The split matters: when the terminal is
// too narrow for everything, actions are dropped and always is kept, so the way
// out of a screen never disappears.
type hintSet struct {
	actions []key.Binding
	always  []key.Binding
}

// hints picks the shortcuts to advertise for wherever the user currently is.
// Nobody should have to memorise shortcuts; the relevant ones stay on screen,
// and "?" reaches the rest.
func (m Model) hints() hintSet {
	switch m.modal {
	case modalCommit:
		actions := []key.Binding{m.keys.Submit}
		if m.ai != nil {
			actions = append(actions, m.generateBinding())
		}
		return hintSet{actions, []key.Binding{m.keys.Cancel}}

	case modalMerge, modalCherryPick:
		return hintSet{
			[]key.Binding{m.keys.Down, m.keys.Up, m.keys.Confirm},
			[]key.Binding{m.keys.Cancel},
		}

	case modalNewBranch, modalStash, modalConfirm:
		return hintSet{[]key.Binding{m.keys.Confirm}, []key.Binding{m.keys.Cancel}}

	case modalSettings:
		return hintSet{
			[]key.Binding{m.keys.Down, m.keys.Up, m.keys.Confirm},
			[]key.Binding{m.keys.Cancel},
		}

	case modalOperation:
		return hintSet{
			[]key.Binding{m.keys.Continue, m.keys.Abort},
			[]key.Binding{m.keys.Cancel},
		}

	case modalHelp:
		return hintSet{
			[]key.Binding{m.keys.Down, m.keys.Up},
			[]key.Binding{m.keys.Cancel},
		}
	}

	if m.filtering {
		return hintSet{[]key.Binding{m.keys.Confirm}, []key.Binding{m.keys.Cancel}}
	}

	// An unfinished merge or cherry-pick outranks everything else on offer.
	set := m.viewHints()
	if m.opState != git.OpNone {
		set.actions = append([]key.Binding{m.resolveBinding()}, set.actions...)
	}
	return set
}

// resolveBinding labels the merge key for what it currently does: while an
// operation is unfinished, "m" opens that rather than starting a new merge.
func (m Model) resolveBinding() key.Binding {
	return key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "resolve "+string(m.opState)),
	)
}

// generateBinding names the agent in the hint, so the key says who is going to
// write the message rather than leaving "AI" to stand for something unnamed.
func (m Model) generateBinding() key.Binding {
	if m.ai == nil {
		return m.keys.Generate
	}
	return key.NewBinding(
		key.WithKeys(m.keys.Generate.Keys()...),
		key.WithHelp(m.keys.Generate.Help().Key, "write with "+m.ai.Name()),
	)
}

// editorBinding names the editor in the hint, so the key says what it will
// actually do rather than leaving the user to find out by pressing it.
func (m Model) editorBinding() key.Binding {
	name := editor.Name(m.editor)
	if name == "" {
		return m.keys.OpenEditor
	}
	return key.NewBinding(
		key.WithKeys(m.keys.OpenEditor.Keys()...),
		key.WithHelp(m.keys.OpenEditor.Help().Key, "open in "+name),
	)
}

// dropStashBinding relabels the delete key for the stash list, where "D" drops
// a stash rather than deleting a branch.
func (m Model) dropStashBinding() key.Binding {
	return key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "drop stash"))
}

func (m Model) viewHints() hintSet {
	wayOut := []key.Binding{m.keys.Help, m.keys.Quit}
	scrolling := hintSet{
		[]key.Binding{m.keys.PageDown, m.keys.PageUp},
		[]key.Binding{m.keys.Cancel, m.keys.Help},
	}
	// The diff pane scrolls sideways and jumps between hunks; the others do not.
	diffScrolling := hintSet{
		[]key.Binding{m.keys.PageDown, m.keys.PageUp, m.keys.NextHunk, m.keys.PrevHunk, m.keys.Right, m.keys.Left},
		[]key.Binding{m.keys.Cancel, m.keys.Help},
	}

	switch m.tab {
	case tabBranches:
		var actions []key.Binding
		if b, ok := m.selectedBranch(); ok && !b.Current {
			actions = append(actions, m.keys.Checkout)
		}
		if m.opState == git.OpNone {
			actions = append(actions, m.keys.Merge)
		}
		actions = append(actions,
			m.keys.NewBranch, m.keys.Filter, m.keys.History,
			m.keys.Fetch, m.keys.Pull, m.keys.Push, m.keys.DeleteRef, m.keys.Copy)
		return hintSet{actions, wayOut}

	case tabHistory:
		if m.focus == paneDetail {
			return scrolling
		}
		return hintSet{
			[]key.Binding{m.keys.Pick, m.keys.CherryPick, m.keys.Filter, m.keys.Copy, m.keys.NextPane},
			wayOut,
		}

	case tabStashes:
		if m.focus == paneDetail {
			return scrolling
		}
		var actions []key.Binding
		if _, ok := m.selectedStash(); ok {
			actions = append(actions, m.keys.StashPop, m.keys.StashApply, m.dropStashBinding())
		}
		actions = append(actions, m.keys.StashPush, m.keys.Filter, m.keys.NextPane)
		return hintSet{actions, wayOut}

	default:
		if m.focus == paneDetail {
			return diffScrolling
		}
		// Ordered by what someone reaching for the footer most often wants:
		// stage, then finish the job, then read through, then the rest.
		actions := []key.Binding{m.keys.Toggle, m.keys.Commit}
		if m.ai != nil {
			actions = append(actions, m.generateBinding())
		}
		actions = append(actions, m.keys.Review)
		if r, ok := m.selected(); ok && !r.staged() {
			actions = append(actions, m.keys.Discard)
		}
		actions = append(actions,
			m.keys.StageAll, m.keys.Filter,
			m.keys.StashPush, m.editorBinding(), m.keys.Copy, m.keys.NextPane)
		return hintSet{actions, wayOut}
	}
}

// renderHints lays out the footer, fitting as many actions as the width allows
// around the shortcuts that are always kept.
func renderHints(set hintSet, width int) string {
	sep := styleDim.Render("  ·  ")
	sepW := lipgloss.Width(sep)

	render := func(b key.Binding) string {
		h := b.Help()
		return styleKey.Render(h.Key) + " " + styleDesc.Render(h.Desc)
	}

	// The always-shown shortcuts claim their space first.
	var kept []string
	keptW := 0
	for _, b := range set.always {
		part := render(b)
		need := lipgloss.Width(part)
		if len(kept) > 0 || keptW > 0 {
			need += sepW
		}
		if keptW+need > width {
			break
		}
		kept = append(kept, part)
		keptW += need
	}

	var shown []string
	used := keptW
	for _, b := range set.actions {
		part := render(b)
		need := lipgloss.Width(part) + sepW
		if used+need > width {
			break
		}
		shown = append(shown, part)
		used += need
	}

	return strings.Join(append(shown, kept...), sep)
}

func padRight(s string, width int) string {
	if n := width - len([]rune(s)); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s + " "
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
