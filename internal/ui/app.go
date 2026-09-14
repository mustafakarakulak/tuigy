package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mustafakarakulak/tuigy/internal/ai"
	"github.com/mustafakarakulak/tuigy/internal/config"
	"github.com/mustafakarakulak/tuigy/internal/git"
	"github.com/mustafakarakulak/tuigy/internal/keys"
)

// pollInterval is how often the status refreshes on its own.
//
// tuigy's core use case is watching the changes an AI coding agent produces in
// another terminal pane, so the list must stay current without a keypress.
// The read takes no index lock, so it never blocks the agent.
const pollInterval = time.Second

// opTimeout bounds a single git write. It is generous enough to fit the network
// operations, which are the slowest thing tuigy does.
const opTimeout = 2 * time.Minute

// historyPage is how many commits are read at a time. The history loads more
// as the cursor approaches the end, so paging is never something the user has
// to think about.
const historyPage = 150

// historyPrefetch is how close to the end the cursor gets before the next page
// is requested.
const historyPrefetch = 20

// horizontalStep is how far one keypress scrolls a diff sideways: far enough to
// make progress through an indented line, short enough not to lose your place.
const horizontalStep = 8

const (
	headerHeight = 3 // repository line, tab row, rule
	footerHeight = 1
)

type tab int

const (
	tabChanges tab = iota
	tabBranches
	tabHistory
	tabStashes
)

var allTabs = []tab{tabChanges, tabBranches, tabHistory, tabStashes}

func (t tab) title() string {
	switch t {
	case tabChanges:
		return "Changes"
	case tabBranches:
		return "Branches"
	case tabHistory:
		return "History"
	default:
		return "Stashes"
	}
}

type pane int

const (
	paneList pane = iota
	paneDetail
)

type modal int

const (
	modalNone modal = iota
	modalCommit
	modalNewBranch
	modalMerge
	modalCherryPick
	modalStash
	modalOperation
	modalConfirm
	modalSettings
	modalHelp
)

// confirmation is the content and action of a confirmation dialog.
type confirmation struct {
	title  string
	detail string
	run    func() tea.Cmd
}

type Model struct {
	repo *git.Repo
	keys keys.Map
	// ai is nil when no coding agent was found, and the feature stays hidden.
	ai *ai.Generator
	// editor is what git itself would open; empty falls back to the environment.
	editor string

	// reviewed records which files have been looked at, and what each looked
	// like at the time. tuigy is meant to be read alongside a coding agent, and
	// an agent that rewrites a file after it was reviewed has to undo that
	// review — which is what the stamp is for.
	reviewed map[string]fileStamp

	// busy names the git operation being waited on, and is empty when idle.
	busy    string
	spinner spinner.Model

	// theme is the name of the palette in use, and themeCursor is where the
	// settings view is pointing while themes are being tried out.
	theme       string
	themeCursor int

	width, height int
	ready         bool

	status   *git.Status
	statusFP string
	opState  git.OpState

	tab tab

	// Changes tab.
	rows    []row
	cursor  int
	listOff int
	diff    viewport.Model
	diffKey string
	// diffRaw is the diff as git wrote it, kept so that a single hunk can be
	// turned back into a patch.
	diffRaw string
	// diffHunks holds the line offsets of the hunk headers in the diff pane,
	// and hunkCursor which of them is selected.
	diffHunks  []int
	hunkCursor int
	focus      pane

	// The help screen scrolls: on a short terminal it would otherwise be
	// impossible to reach the shortcuts near the bottom.
	help viewport.Model

	// Branches tab. allBranches is the unfiltered list the rows are built from.
	allBranches []git.Branch
	branchRows  []branchRow
	branchCur   int
	branchOff   int

	// filter narrows the active list. It is cleared when the tab changes, so
	// a forgotten filter can never hide things on a screen you did not set it on.
	filter    string
	filtering bool

	// History tab.
	historyRef   string
	commits      []git.Commit
	commitCursor int
	commitOff    int
	// picked holds the hashes selected for a cherry-pick, by hash rather than
	// by index so a selection survives paging and reloads.
	picked      map[string]bool
	historyDone bool
	historyBusy bool

	detail    viewport.Model
	detailKey string

	// Stashes tab. allStashes is the unfiltered list.
	allStashes  []git.Stash
	stashes     []git.Stash
	stashCursor int
	stashOff    int
	stashView   viewport.Model
	stashKey    string

	listW, listH int
	diffW, diffH int

	modal modal

	commit     textarea.Model
	amending   bool
	generating bool

	// nameInput serves every dialog that asks for a single line of text.
	nameInput  textinput.Model
	branchFrom string

	// Branch picker, shared by the merge and cherry-pick dialogs.
	targets     []string
	targetIndex int

	// Merge dialog.
	mergeSource string
	// mergePlan is nil while the fast-forward check is still running.
	mergePlan *mergePlan

	// Cherry-pick dialog: the commits it will apply, newest first.
	pickCommits []git.Commit

	confirm confirmation

	err     error
	flash   string
	flashID int
}

// Option adjusts a model at construction. Everything has a working default, so
// a caller that has read no configuration can simply call New(repo).
type Option func(*Model)

// WithKeys replaces the key map, which is how a config file rebinds actions.
func WithKeys(k keys.Map) Option {
	return func(m *Model) { m.keys = k }
}

// WithAI sets the commit message generator, or disables it when nil.
func WithAI(g *ai.Generator) Option {
	return func(m *Model) { m.ai = g }
}

// WithEditor sets the command used to open a file, which is asked of git so
// that core.editor is honoured like anywhere else.
func WithEditor(command string) Option {
	return func(m *Model) { m.editor = command }
}

// WithTheme records which theme is in use, so the settings view opens on it.
func WithTheme(name string) Option {
	return func(m *Model) {
		if name != "" {
			m.theme = name
		}
	}
}

func New(repo *git.Repo, opts ...Option) Model {
	ta := textarea.New()
	ta.Placeholder = "feat: a short, descriptive subject"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0

	ti := textinput.New()
	ti.Placeholder = "feature/payment-retry"
	ti.Prompt = ""

	m := Model{
		repo:      repo,
		theme:     "default",
		keys:      keys.Default(),
		ai:        ai.Detect(""),
		commit:    ta,
		nameInput: ti,
		diff:      viewport.New(0, 0),
		detail:    viewport.New(0, 0),
		stashView: viewport.New(0, 0),
		help:      viewport.New(0, 0),
		spinner:   newSpinner(),
	}

	for _, opt := range opts {
		opt(&m)
	}
	return m
}

// newSpinner is deliberately understated: it says work is happening without
// pulling the eye away from what the user was reading.
func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return s
}

func (m Model) Init() tea.Cmd {
	// Branches are loaded up front, not on first visit: the cherry-pick dialog
	// needs the list of local branches whichever tab it is opened from.
	return tea.Batch(m.loadStatus(), m.loadBranches(), scheduleTick())
}

// ---------------------------------------------------------------- messages

type (
	tickMsg   struct{}
	statusMsg struct {
		status *git.Status
		state  git.OpState
	}
	branchesMsg struct{ branches []git.Branch }
	diffMsg     struct{ key, text string }
	// opStartedMsg marks the view busy; opDoneMsg reports how it went.
	opStartedMsg struct{ label string }
	opDoneMsg    struct {
		label string
		err   error
	}
	errMsg        struct{ err error }
	clearFlashMsg struct{ id int }

	// checkoutFailedMsg lets a failed switch offer to stash first, which needs
	// to know which branch was being switched to.
	checkoutFailedMsg struct {
		branch string
		err    error
	}

	mergePlanMsg  struct{ plan mergePlan }
	editorDoneMsg struct{ err error }

	themeSavedMsg struct {
		path string
		err  error
	}

	aiMessageMsg struct {
		message string
		err     error
	}

	stashesMsg   struct{ stashes []git.Stash }
	stashDiffMsg struct{ key, text string }

	commitsMsg struct {
		ref     string
		filter  string
		skip    int
		commits []git.Commit
	}
	detailMsg struct {
		key    string
		detail *git.CommitDetail
	}
)

// mergePlan is what a merge would actually do, worked out before it is run so
// that the confirmation can say it rather than leave the user guessing.
type mergePlan struct {
	source, target string
	// intoCurrent means the merge lands on the branch already checked out.
	intoCurrent bool
	// fastForward means target only has to be moved forward, so neither the
	// working tree nor the current branch is touched.
	fastForward bool
}

func (p mergePlan) summary() string {
	switch {
	case p.intoCurrent:
		return "merged into the branch you are on"
	case p.fastForward:
		return "fast-forward: your working tree and current branch are untouched"
	default:
		return p.target + " will be checked out first, and you will be left on it"
	}
}

func scheduleTick() tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) loadStatus() tea.Cmd {
	repo := m.repo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		st, err := repo.Status(ctx)
		if err != nil {
			return errMsg{err}
		}
		return statusMsg{status: st, state: repo.State()}
	}
}

func (m Model) loadBranches() tea.Cmd {
	repo := m.repo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		branches, err := repo.Branches(ctx)
		if err != nil {
			return errMsg{err}
		}
		return branchesMsg{branches: branches}
	}
}

func (m Model) loadHistory(ref string, skip int) tea.Cmd {
	repo, filter := m.repo, m.filter
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		commits, err := repo.Log(ctx, ref, filter, skip, historyPage)
		if err != nil {
			return errMsg{err}
		}
		return commitsMsg{ref: ref, filter: filter, skip: skip, commits: commits}
	}
}

func (m Model) loadStashes() tea.Cmd {
	repo := m.repo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		stashes, err := repo.Stashes(ctx)
		if err != nil {
			return errMsg{err}
		}
		return stashesMsg{stashes: stashes}
	}
}

func (m Model) loadStashDiff(s git.Stash) tea.Cmd {
	repo, ref, key := m.repo, s.Ref, stashKey(s)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		diff, err := repo.StashDiff(ctx, ref)
		if err != nil {
			return errMsg{err}
		}
		return stashDiffMsg{key: key, text: diff}
	}
}

func (m Model) loadDetail(c git.Commit) tea.Cmd {
	repo, hash := m.repo, c.Hash
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		detail, err := repo.CommitDetail(ctx, hash)
		if err != nil {
			return errMsg{err}
		}
		return detailMsg{key: hash, detail: detail}
	}
}

func (m Model) loadDiff(r row) tea.Cmd {
	repo, k := m.repo, diffKey(r)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		text, err := repo.FileDiff(ctx, r.file, r.staged())
		if err != nil {
			return errMsg{err}
		}
		return diffMsg{key: k, text: text}
	}
}

// runOp performs a git write in the background; the UI never blocks on it.
//
// doing is what to show while it runs and done is what to report afterwards,
// because "pushing" and "pushed" are not the same message. The two commands are
// sequenced rather than batched so the view is marked busy before the work
// starts instead of racing with it.
func runOp(doing, done string, fn func(context.Context) error) tea.Cmd {
	return tea.Sequence(
		func() tea.Msg { return opStartedMsg{label: doing} },
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
			defer cancel()
			return opDoneMsg{label: done, err: fn(ctx)}
		},
	)
}

func diffKey(r row) string {
	return fmt.Sprintf("%d\x00%s", r.sec, r.file.Path)
}

// fingerprint summarises the meaningful content of a status. If the once-a-second
// poll comes back unchanged we touch neither the list nor the diff, so the
// cursor and scroll position never move under the user.
func fingerprint(st *git.Status, state git.OpState) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s|%d|%d|%v|%s\n", st.Branch, st.Head, st.Ahead, st.Behind, st.Detached, state)
	for _, f := range st.Files {
		fmt.Fprintf(&b, "%s\x00%s\x00%c%c%v\n", f.Path, f.OrigPath, f.Index, f.Worktree, f.Unmerged)
	}
	return b.String()
}

// ---------------------------------------------------------------- update

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.layout()
		// Diff lines are clipped to the pane width, so they need re-rendering.
		return m, m.syncDiff()

	case tickMsg:
		return m, tea.Batch(m.loadStatus(), scheduleTick())

	case statusMsg:
		return m.applyStatus(msg)

	case branchesMsg:
		m.applyBranches(msg.branches)
		return m, nil

	case stashesMsg:
		return m, m.applyStashes(msg.stashes)

	case stashDiffMsg:
		if msg.key == m.stashKey {
			if s, ok := m.selectedStash(); ok {
				m.stashView.SetContent(renderStashDetail(s, msg.text, m.diffW))
				m.stashView.GotoTop()
			}
		}
		return m, nil

	case commitsMsg:
		return m, m.applyCommits(msg)

	case detailMsg:
		// Stale requests that lost the race are dropped.
		if msg.key == m.detailKey {
			m.detail.SetContent(renderCommitDetail(msg.detail, m.diffW))
			m.detail.GotoTop()
		}
		return m, nil

	case diffMsg:
		// Stale requests that lost the race are dropped.
		if msg.key == m.diffKey {
			m.diffRaw = msg.text
			m.hunkCursor = 0
			m.renderDiffPane()
			m.diff.GotoTop()
			m.diff.SetXOffset(0)
		}
		return m, nil

	case opStartedMsg:
		m.busy = msg.label
		return m, m.spinner.Tick

	case spinner.TickMsg:
		// Ticking stops on its own once there is nothing to wait for.
		if m.busy == "" {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case opDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.err = msg.err
			return m, m.reload()
		}
		m.err = nil
		return m, tea.Batch(m.reload(), m.setFlash(msg.label))

	case checkoutFailedMsg:
		return m.offerStashAndSwitch(msg)

	case mergePlanMsg:
		// Drop a plan the user has already navigated away from.
		if m.modal == modalMerge && msg.plan.source == m.mergeSource && msg.plan.target == m.currentTarget() {
			plan := msg.plan
			m.mergePlan = &plan
		}
		return m, nil

	case editorDoneMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		// The editor almost certainly changed something on disk.
		m.statusFP = ""
		return m, m.reload()

	case themeSavedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		return m, m.setFlash("theme saved to " + config.ShortPath(msg.path))

	case aiMessageMsg:
		m.generating = false
		m.busy = ""
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.commit.SetValue(msg.message)
		return m, nil

	case amendPrefillMsg:
		m.err = nil
		m.amending = true
		m.modal = modalCommit
		m.commit.SetValue(msg.message)
		return m, m.commit.Focus()

	case errMsg:
		m.err = msg.err
		return m, nil

	case clearFlashMsg:
		if msg.id == m.flashID {
			m.flash = ""
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// reload refreshes everything the active view depends on.
func (m Model) reload() tea.Cmd {
	switch m.tab {
	case tabBranches:
		return tea.Batch(m.loadStatus(), m.loadBranches())
	case tabHistory:
		return tea.Batch(m.loadStatus(), m.loadHistory(m.historyRef, 0))
	case tabStashes:
		return tea.Batch(m.loadStatus(), m.loadStashes())
	default:
		return m.loadStatus()
	}
}

// fileStamp is enough to notice that a file has been rewritten. Hashing the
// contents would be exact, but this runs against every reviewed file on every
// poll, and a changed file always changes one of the two.
type fileStamp struct {
	modTime time.Time
	size    int64
}

func stampFile(path string) (fileStamp, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, err
	}
	return fileStamp{modTime: info.ModTime(), size: info.Size()}, nil
}

// dropStaleReviews forgets files that have changed since they were reviewed.
//
// It runs on every poll rather than only when the status changes: an agent
// rewriting a file it had already modified leaves the status identical while
// making the review worthless.
func (m *Model) dropStaleReviews() {
	if len(m.reviewed) == 0 {
		return
	}

	kept := make(map[string]fileStamp, len(m.reviewed))
	for path, stamp := range m.reviewed {
		if current, err := stampFile(filepath.Join(m.repo.Root, path)); err == nil && current == stamp {
			kept[path] = stamp
		}
	}
	m.reviewed = kept
}

// reviewProgress counts how many of the changed files have been reviewed.
func (m Model) reviewProgress() (done, total int) {
	if m.status == nil {
		return 0, 0
	}
	for _, f := range m.status.Files {
		total++
		if _, ok := m.reviewed[f.Path]; ok {
			done++
		}
	}
	return done, total
}

// unreviewedStaged counts staged files that have not been looked at.
func (m Model) unreviewedStaged() int {
	if m.status == nil {
		return 0
	}

	n := 0
	for _, f := range m.status.Staged() {
		if _, ok := m.reviewed[f.Path]; !ok {
			n++
		}
	}
	return n
}

func (m Model) applyStatus(msg statusMsg) (tea.Model, tea.Cmd) {
	m.dropStaleReviews()

	fp := fingerprint(msg.status, msg.state)
	if fp == m.statusFP {
		return m, nil
	}

	prev := m.currentSelection()

	m.status, m.opState, m.statusFP = msg.status, msg.state, fp
	m.rows = buildRows(msg.status, m.filter)
	m.err = nil

	m.cursor = reselect(m.rows, prev)
	m.ensureVisible()

	cmds := []tea.Cmd{m.syncDiff()}
	// A changed status usually means refs moved too, so whichever ref-derived
	// view the user is looking at is now stale.
	switch m.tab {
	case tabBranches:
		cmds = append(cmds, m.loadBranches())
	case tabHistory:
		cmds = append(cmds, m.loadHistory(m.historyRef, 0))
	case tabStashes:
		cmds = append(cmds, m.loadStashes())
	}
	return m, tea.Batch(cmds...)
}

// stashKey identifies a stash by what it holds rather than by its ref, because
// dropping one renumbers every stash below it.
func stashKey(s git.Stash) string {
	return fmt.Sprintf("%s\x00%s\x00%d", s.Ref, s.Message, s.Date.Unix())
}

func (m *Model) applyStashes(stashes []git.Stash) tea.Cmd {
	prev := ""
	if s, ok := m.selectedStash(); ok {
		prev = stashKey(s)
	}

	m.allStashes = stashes
	m.stashes = nil
	for _, stash := range stashes {
		if matchesFilter(m.filter, stash.Message) {
			m.stashes = append(m.stashes, stash)
		}
	}
	stashes = m.stashes
	m.stashCursor = 0
	if prev != "" {
		for i, s := range stashes {
			if stashKey(s) == prev {
				m.stashCursor = i
				break
			}
		}
	}
	m.stashCursor = clamp(m.stashCursor, 0, max(len(stashes)-1, 0))
	m.stashOff = scrollTo(m.stashCursor, m.stashOff, m.listH)
	return m.syncStashDiff()
}

func (m *Model) syncStashDiff() tea.Cmd {
	s, ok := m.selectedStash()
	if !ok {
		m.stashKey = ""
		m.stashView.SetContent(styleDim.Render("no stashes"))
		return nil
	}
	if key := stashKey(s); key != m.stashKey {
		m.stashKey = key
		return m.loadStashDiff(s)
	}
	return nil
}

func (m Model) selectedStash() (git.Stash, bool) {
	if m.stashCursor < 0 || m.stashCursor >= len(m.stashes) {
		return git.Stash{}, false
	}
	return m.stashes[m.stashCursor], true
}

func (m *Model) moveStashCursor(delta int) tea.Cmd {
	if len(m.stashes) == 0 {
		return nil
	}
	m.stashCursor = clamp(m.stashCursor+delta, 0, len(m.stashes)-1)
	m.stashOff = scrollTo(m.stashCursor, m.stashOff, m.listH)
	return m.syncStashDiff()
}

// applyCommits folds a freshly loaded page into the history.
func (m *Model) applyCommits(msg commitsMsg) tea.Cmd {
	// The user may have changed the ref or the filter while this was in flight.
	if msg.ref != m.historyRef || msg.filter != m.filter {
		return nil
	}
	m.historyBusy = false

	if msg.skip == 0 {
		// A reload must not move the cursor off the commit being looked at.
		selected := ""
		if c, ok := m.selectedCommit(); ok {
			selected = c.Hash
		}

		m.commits = msg.commits
		m.commitCursor = 0
		for i, c := range m.commits {
			if c.Hash == selected {
				m.commitCursor = i
				break
			}
		}
	} else {
		// Ignore a page that does not continue where the list currently ends,
		// which happens when a reload lands between request and response.
		if msg.skip != len(m.commits) {
			return nil
		}
		m.commits = append(m.commits, msg.commits...)
	}

	m.historyDone = len(msg.commits) < historyPage
	m.commitOff = scrollTo(m.commitCursor, m.commitOff, m.listH)
	return m.syncDetail()
}

// showHistory points the history tab at a ref, loading it if need be.
func (m *Model) showHistory(ref string) tea.Cmd {
	if ref == "" {
		ref = m.historyRef
	}
	if ref == "" {
		ref = "HEAD"
		if m.status != nil && m.status.Branch != "" {
			ref = m.status.Branch
		}
	}

	if ref != m.historyRef {
		m.historyRef = ref
		m.commits = nil
		m.commitCursor, m.commitOff = 0, 0
		m.historyDone = false
		m.picked = nil
	}

	if len(m.commits) > 0 {
		return m.syncDetail()
	}
	m.historyBusy = true
	return m.loadHistory(m.historyRef, 0)
}

// syncDetail reloads the commit detail when the selected commit has changed.
func (m *Model) syncDetail() tea.Cmd {
	c, ok := m.selectedCommit()
	if !ok {
		m.detailKey = ""
		m.detail.SetContent(styleDim.Render("no commit selected"))
		return nil
	}
	if c.Hash == m.detailKey {
		return nil
	}
	m.detailKey = c.Hash
	return m.loadDetail(c)
}

func (m Model) selectedCommit() (git.Commit, bool) {
	if m.commitCursor < 0 || m.commitCursor >= len(m.commits) {
		return git.Commit{}, false
	}
	return m.commits[m.commitCursor], true
}

// pickedCommits are the selected commits in display order, newest first. With
// nothing explicitly selected, the commit under the cursor is what is meant.
func (m Model) pickedCommits() []git.Commit {
	var out []git.Commit
	for _, c := range m.commits {
		if m.picked[c.Hash] {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		if c, ok := m.selectedCommit(); ok {
			return []git.Commit{c}
		}
	}
	return out
}

func (m *Model) moveCommitCursor(delta int) tea.Cmd {
	if len(m.commits) == 0 {
		return nil
	}
	m.commitCursor = clamp(m.commitCursor+delta, 0, len(m.commits)-1)
	m.commitOff = scrollTo(m.commitCursor, m.commitOff, m.listH)
	return tea.Batch(m.syncDetail(), m.maybeLoadMore())
}

// maybeLoadMore fetches the next page as the cursor nears the end of what is
// loaded, so scrolling through a long history never stalls.
func (m *Model) maybeLoadMore() tea.Cmd {
	if m.historyDone || m.historyBusy || m.historyRef == "" {
		return nil
	}
	if m.commitCursor < len(m.commits)-historyPrefetch {
		return nil
	}
	m.historyBusy = true
	return m.loadHistory(m.historyRef, len(m.commits))
}

func (m *Model) applyBranches(branches []git.Branch) {
	prev := m.selectedBranchName()

	m.allBranches = branches
	m.branchRows = buildBranchRows(branches, m.filter)
	// The list is ordered by recency, so the current branch is rarely first.
	// Opening the tab on the branch you are actually on is the useful default.
	m.branchCur = currentBranchRow(m.branchRows)

	if prev != "" {
		for i, r := range m.branchRows {
			if !r.header && r.branch.Name == prev {
				m.branchCur = i
				break
			}
		}
	}
	m.ensureBranchVisible()
}

// renderDiffPane redraws the diff for the hunk currently under the cursor.
func (m *Model) renderDiffPane() {
	rendered := renderDiff(m.diffRaw, m.hunkCursor)
	m.diff.SetContent(rendered.content)
	m.diffHunks = rendered.hunks
	m.hunkCursor = clamp(m.hunkCursor, 0, max(len(m.diffHunks)-1, 0))
}

// moveHunkCursor steps between hunks and brings the chosen one into view.
func (m *Model) moveHunkCursor(delta int) {
	if len(m.diffHunks) == 0 {
		return
	}
	m.hunkCursor = clamp(m.hunkCursor+delta, 0, len(m.diffHunks)-1)
	m.renderDiffPane()
	m.diff.SetYOffset(m.diffHunks[m.hunkCursor])
}

// syncDiff reloads the diff when the selected row has changed.
func (m *Model) syncDiff() tea.Cmd {
	r, ok := m.selected()
	if !ok {
		m.diffKey = ""
		m.diffRaw = ""
		m.diffHunks = nil
		m.diff.SetContent(styleDim.Render("no file selected"))
		return nil
	}
	m.diffKey = diffKey(r)
	return m.loadDiff(r)
}

func (m *Model) setFlash(text string) tea.Cmd {
	m.flash = text
	m.flashID++
	id := m.flashID
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{id: id} })
}

// ---------------------------------------------------------------- layout

func (m *Model) layout() {
	bodyH := max(m.height-headerHeight-footerHeight, 3)

	listW := clamp(m.width*2/5, 26, 52)
	if listW > m.width-20 {
		listW = max(m.width/2, 12)
	}

	// The border costs each pane two columns and two rows.
	m.listW, m.listH = max(listW-2, 1), max(bodyH-2, 1)
	m.diffW, m.diffH = max(m.width-listW-2, 1), max(bodyH-2, 1)

	m.diff.Width, m.diff.Height = m.diffW, m.diffH
	m.detail.Width, m.detail.Height = m.diffW, m.diffH
	m.stashView.Width, m.stashView.Height = m.diffW, m.diffH

	m.commit.SetWidth(max(m.width/2, 20))
	m.commit.SetHeight(max(min(bodyH-10, 8), 3))
	m.nameInput.Width = max(m.width/3, 16)

	// The help viewport fills the dialog interior: border and padding cost it
	// 6 columns and 4 rows.
	m.help.Width, m.help.Height = max(m.width-6, 8), max(bodyH-4, 1)

	m.ensureVisible()
	m.ensureBranchVisible()
}

func (m *Model) ensureVisible() {
	m.listOff = scrollTo(m.cursor, m.listOff, m.listH)
}

func (m *Model) ensureBranchVisible() {
	m.branchOff = scrollTo(m.branchCur, m.branchOff, m.listH)
}

// scrollTo returns the offset that keeps cursor inside a window of height rows.
func scrollTo(cursor, offset, height int) int {
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+height {
		offset = cursor - height + 1
	}
	return max(offset, 0)
}

// ---------------------------------------------------------------- selection

func (m Model) selected() (row, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) || m.rows[m.cursor].header {
		return row{}, false
	}
	return m.rows[m.cursor], true
}

// matchesFilter reports whether text satisfies the current filter, which is a
// case-insensitive substring. An empty filter matches everything.
func matchesFilter(filter, text string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(filter))
}

// applyFilter rebuilds the active list for the current filter.
//
// Each tab filters what it already has, except the history, which asks git: a
// commit worth finding is usually older than the page that happens to be loaded.
func (m *Model) applyFilter() tea.Cmd {
	switch m.tab {
	case tabBranches:
		m.applyBranches(m.allBranches)
		return nil

	case tabStashes:
		return m.applyStashes(m.allStashes)

	case tabHistory:
		m.commits = nil
		m.commitCursor, m.commitOff = 0, 0
		m.historyDone = false
		m.historyBusy = true
		return m.loadHistory(m.historyRef, 0)

	default:
		m.rows = buildRows(m.status, m.filter)
		m.cursor = firstFileRow(m.rows)
		m.ensureVisible()
		return m.syncDiff()
	}
}

// clearFilter drops the filter and rebuilds, which is what leaving a tab does.
func (m *Model) clearFilter() {
	m.filter, m.filtering = "", false
}

// filteredCount is how many rows the filter is hiding, for the header.
func (m Model) filteredCount() (shown, total int) {
	switch m.tab {
	case tabBranches:
		return len(m.branchRows) - countHeaders(m.branchRows), len(m.allBranches)
	case tabStashes:
		return len(m.stashes), len(m.allStashes)
	case tabHistory:
		return len(m.commits), len(m.commits)
	default:
		if m.status == nil {
			return 0, 0
		}
		return countFiles(m.rows), len(m.status.Files)
	}
}

func countHeaders(rows []branchRow) int {
	n := 0
	for _, r := range rows {
		if r.header {
			n++
		}
	}
	return n
}

func countFiles(rows []row) int {
	n := 0
	for _, r := range rows {
		if !r.header {
			n++
		}
	}
	return n
}

func (m Model) selectedBranch() (git.Branch, bool) {
	if m.branchCur < 0 || m.branchCur >= len(m.branchRows) || m.branchRows[m.branchCur].header {
		return git.Branch{}, false
	}
	return m.branchRows[m.branchCur].branch, true
}

func (m Model) selectedBranchName() string {
	if b, ok := m.selectedBranch(); ok {
		return b.Name
	}
	return ""
}

func (m Model) firstFileRow() int { return firstFileRow(m.rows) }

func firstFileRow(rows []row) int {
	for i, r := range rows {
		if !r.header {
			return i
		}
	}
	return 0
}

func (m Model) lastFileRow() int {
	for i := len(m.rows) - 1; i >= 0; i-- {
		if !m.rows[i].header {
			return i
		}
	}
	return 0
}

// localBranchNames lists local branches, optionally leaving one out.
func (m Model) localBranchNames(exclude string) []string {
	var names []string
	for _, r := range m.branchRows {
		if r.header || r.branch.Remote || r.branch.Name == exclude {
			continue
		}
		names = append(names, r.branch.Name)
	}
	return names
}

// currentTarget is the branch highlighted in the picker.
func (m Model) currentTarget() string {
	if m.targetIndex < 0 || m.targetIndex >= len(m.targets) {
		return ""
	}
	return m.targets[m.targetIndex]
}

func firstBranchRow(rows []branchRow) int {
	for i, r := range rows {
		if !r.header {
			return i
		}
	}
	return 0
}

// currentBranchRow is the row of the checked-out branch, falling back to the
// first branch when HEAD is detached.
func currentBranchRow(rows []branchRow) int {
	for i, r := range rows {
		if !r.header && r.branch.Current {
			return i
		}
	}
	return firstBranchRow(rows)
}

// selection describes the cursor position independently of the row list.
type selection struct {
	valid bool
	sec   section
	path  string
	idx   int // the file's position within its own section
}

func (m Model) currentSelection() selection {
	cur, ok := m.selected()
	if !ok {
		return selection{}
	}

	idx := 0
	for i, r := range m.rows {
		if r.header || r.sec != cur.sec {
			continue
		}
		if i == m.cursor {
			break
		}
		idx++
	}
	return selection{valid: true, sec: cur.sec, path: cur.file.Path, idx: idx}
}

// reselect picks the row the cursor lands on after the list is rebuilt.
//
// The order is deliberate and separates two different situations:
//
//  1. If the file has not moved, stay on it. An agent adding files in the
//     background must not shift the user's selection.
//  2. If the file changed section, the user staged it: move to the slot it
//     vacated in the old section, so holding down space stages several files
//     in a row without navigating.
//  3. If the old section emptied out, follow the file to its new section.
func reselect(rows []row, prev selection) int {
	if !prev.valid {
		return firstFileRow(rows)
	}

	samePath, sameSecNth, sameSecLast, n := -1, -1, -1, 0

	for i, r := range rows {
		if r.header {
			continue
		}
		if r.sec == prev.sec {
			if n == prev.idx {
				sameSecNth = i
			}
			sameSecLast = i
			n++
		}
		if r.file.Path == prev.path {
			if r.sec == prev.sec {
				return i
			}
			if samePath < 0 {
				samePath = i
			}
		}
	}

	switch {
	case sameSecNth >= 0:
		return sameSecNth
	case sameSecLast >= 0:
		return sameSecLast
	case samePath >= 0:
		return samePath
	default:
		return firstFileRow(rows)
	}
}

func (m *Model) moveCursor(delta int) tea.Cmd {
	i := m.cursor
	for {
		i += delta
		if i < 0 || i >= len(m.rows) {
			return nil // at the end of the list; leave the cursor where it is
		}
		if !m.rows[i].header {
			m.cursor = i
			m.ensureVisible()
			return m.syncDiff()
		}
	}
}

func (m *Model) moveBranchCursor(delta int) {
	i := m.branchCur
	for {
		i += delta
		if i < 0 || i >= len(m.branchRows) {
			return
		}
		if !m.branchRows[i].header {
			m.branchCur = i
			m.ensureBranchVisible()
			return
		}
	}
}

func clamp(v, lo, hi int) int { return min(max(v, lo), hi) }

var _ tea.Model = Model{}
