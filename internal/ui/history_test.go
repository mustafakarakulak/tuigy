package ui

import (
	"context"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mustafakarakulak/tuigy/internal/git"
)

// newHistoryModel builds a model over a repository where feature carries three
// commits that main does not have, plus a branch that conflicts with them.
func newHistoryModel(t *testing.T, width, height int) (Model, string) {
	t.Helper()

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	run(t, dir, "init", "-b", "main")
	run(t, dir, "config", "user.email", "t@example.com")
	run(t, dir, "config", "user.name", "t")

	write(t, dir, "shared.txt", "base\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-m", "initial")

	run(t, dir, "checkout", "-qb", "feature")
	for _, c := range []struct{ file, msg string }{
		{"one.txt", "feat: first"},
		{"two.txt", "feat: second"},
		{"three.txt", "feat: third"},
	} {
		write(t, dir, c.file, c.file+"\n")
		run(t, dir, "add", ".")
		run(t, dir, "commit", "-m", c.msg)
	}

	run(t, dir, "checkout", "-q", "main")

	repo, err := git.Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}

	m := New(repo)
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return next.(Model).reloadAll(t), dir
}

// step runs a command and feeds the result back, the way the bubbletea runtime
// would. It returns any follow-up command.
func (m Model) step(t *testing.T, cmd tea.Cmd) (Model, tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return m, nil
	}

	msg := cmd()
	if msg == nil {
		return m, nil
	}

	if grouped, ok := groupedCommands(msg); ok {
		var follow tea.Cmd
		for _, c := range grouped {
			m, follow = m.step(t, c)
		}
		return m, follow
	}

	next, follow := m.Update(msg)
	return next.(Model), follow
}

// groupedCommands unwraps the messages bubbletea uses to carry several commands
// at once. tea.Batch's message type is exported and tea.Sequence's is not, so
// both are recognised by their shape instead: a slice of commands.
func groupedCommands(msg tea.Msg) ([]tea.Cmd, bool) {
	value := reflect.ValueOf(msg)
	if value.Kind() != reflect.Slice {
		return nil, false
	}

	cmds := make([]tea.Cmd, 0, value.Len())
	for i := range value.Len() {
		cmd, ok := value.Index(i).Interface().(tea.Cmd)
		if !ok {
			return nil, false
		}
		cmds = append(cmds, cmd)
	}
	return cmds, true
}

// opResult drives a command to the operation result it eventually produces.
func opResult(t *testing.T, cmd tea.Cmd) opDoneMsg {
	t.Helper()

	var found *opDoneMsg
	var drive func(tea.Cmd)
	drive = func(c tea.Cmd) {
		if c == nil || found != nil {
			return
		}
		msg := c()
		if grouped, ok := groupedCommands(msg); ok {
			for _, inner := range grouped {
				drive(inner)
			}
			return
		}
		if done, ok := msg.(opDoneMsg); ok {
			found = &done
		}
	}

	drive(cmd)
	if found == nil {
		t.Fatal("the command produced no operation result")
	}
	return *found
}

// openHistory switches to the history tab and settles the loads it starts.
func (m Model) openHistory(t *testing.T, keys ...string) Model {
	t.Helper()

	next, cmd := m.press(t, "3")
	m = next
	for range 3 { // commits, then the detail they trigger
		m, cmd = m.step(t, cmd)
	}

	for _, k := range keys {
		next, cmd = m.press(t, k)
		m = next
		for range 3 {
			m, cmd = m.step(t, cmd)
		}
	}
	return m
}

func commitSubjects(commits []git.Commit) []string {
	out := make([]string, len(commits))
	for i, c := range commits {
		out[i] = c.Subject
	}
	return out
}

func TestHistoryLoadsCurrentBranch(t *testing.T) {
	m, _ := newHistoryModel(t, 120, 32)
	m = m.openHistory(t)

	if m.tab != tabHistory {
		t.Fatal("pressing 3 should show the history tab")
	}
	if m.historyRef != "main" {
		t.Errorf("historyRef = %q, want the current branch main", m.historyRef)
	}
	if got := commitSubjects(m.commits); !slices.Equal(got, []string{"initial"}) {
		t.Fatalf("commits = %v, want main's history only", got)
	}
	// Only one page exists, so there is nothing more to fetch.
	if !m.historyDone {
		t.Error("historyDone = false, want true for a history that fits in one page")
	}

	view := m.View()
	for _, want := range []string{"initial", "main"} {
		if !strings.Contains(view, want) {
			t.Errorf("the history view is missing %q", want)
		}
	}
}

func TestHistoryOfAnotherBranchFromTheBranchList(t *testing.T) {
	m, _ := newHistoryModel(t, 120, 32)

	m, _ = m.press(t, "2")
	m = m.selectBranch(t, "feature")

	next, cmd := m.press(t, "l")
	m = next
	for range 3 {
		m, cmd = m.step(t, cmd)
	}

	if m.tab != tabHistory {
		t.Fatal("l should open the history tab")
	}
	if m.historyRef != "feature" {
		t.Errorf("historyRef = %q, want feature", m.historyRef)
	}
	want := []string{"feat: third", "feat: second", "feat: first", "initial"}
	if got := commitSubjects(m.commits); !slices.Equal(got, want) {
		t.Errorf("commits = %v, want %v", got, want)
	}
	if !strings.Contains(plain(m.View()), "feature") {
		t.Error("the view does not say which ref it is showing")
	}
}

func TestHistoryCursorMovesAndDetailFollows(t *testing.T) {
	m, _ := newHistoryModel(t, 120, 32)

	m, _ = m.press(t, "2")
	m = m.selectBranch(t, "feature")
	next, cmd := m.press(t, "l")
	m = next
	for range 3 {
		m, cmd = m.step(t, cmd)
	}

	if got := m.commits[m.commitCursor].Subject; got != "feat: third" {
		t.Fatalf("cursor starts on %q, want the newest commit", got)
	}

	next, cmd = m.press(t, "j")
	m = next
	for range 2 {
		m, cmd = m.step(t, cmd)
	}

	if got := m.commits[m.commitCursor].Subject; got != "feat: second" {
		t.Errorf("after j the cursor is on %q, want feat: second", got)
	}
	if !strings.Contains(plain(m.View()), "two.txt") {
		t.Error("the detail pane does not show the selected commit's file")
	}
}

// Space marks a commit and steps down, so a run of commits can be selected
// without alternating between two keys.
func TestSelectingCommitsAdvancesTheCursor(t *testing.T) {
	m, _ := newHistoryModel(t, 120, 32)

	m, _ = m.press(t, "2")
	m = m.selectBranch(t, "feature")
	m = m.openHistoryOfSelected(t)

	for range 2 {
		next, cmd := m.press(t, " ")
		m = next
		for range 2 {
			m, cmd = m.step(t, cmd)
		}
	}

	if len(m.picked) != 2 {
		t.Fatalf("%d commits selected, want 2", len(m.picked))
	}
	if got := commitSubjects(m.pickedCommits()); !slices.Equal(got, []string{"feat: third", "feat: second"}) {
		t.Errorf("selection = %v, want the top two commits", got)
	}
	if !strings.Contains(plain(m.View()), "2 commits selected") {
		t.Error("the view does not show how many commits are selected")
	}

	// Selecting is a toggle.
	m = m.selectCommit(t, "feat: third")
	next, cmd := m.press(t, " ")
	m = next
	for range 2 {
		m, cmd = m.step(t, cmd)
	}
	if len(m.picked) != 1 {
		t.Errorf("%d commits selected after toggling one off, want 1", len(m.picked))
	}
}

// With nothing explicitly selected, the commit under the cursor is what the
// user means.
func TestCherryPickDefaultsToTheCommitUnderTheCursor(t *testing.T) {
	m, _ := newHistoryModel(t, 120, 32)

	m, _ = m.press(t, "2")
	m = m.selectBranch(t, "feature")
	m = m.openHistoryOfSelected(t)

	m, _ = m.press(t, "y")
	if m.modal != modalCherryPick {
		t.Fatal("y should open the cherry-pick dialog")
	}
	if got := commitSubjects(m.pickCommits); !slices.Equal(got, []string{"feat: third"}) {
		t.Errorf("dialog holds %v, want the commit under the cursor", got)
	}
}

func TestCherryPickAppliesInChronologicalOrder(t *testing.T) {
	m, dir := newHistoryModel(t, 120, 32)

	m, _ = m.press(t, "2")
	m = m.selectBranch(t, "feature")
	m = m.openHistoryOfSelected(t)

	// Select all three feature commits.
	for range 3 {
		next, cmd := m.press(t, " ")
		m = next
		for range 2 {
			m, cmd = m.step(t, cmd)
		}
	}

	m, _ = m.press(t, "y")
	if m.modal != modalCherryPick {
		t.Fatal("y should open the cherry-pick dialog")
	}

	// The dialog must list them in the order they will be replayed.
	view := m.View()
	first := strings.Index(view, "feat: first")
	third := strings.Index(view, "feat: third")
	if first < 0 || third < 0 || first > third {
		t.Errorf("the dialog does not list commits oldest first:\n%s", view)
	}

	m = m.selectTarget(t, "main")
	_, cmd := m.press(t, "enter")
	if done := opResult(t, cmd); done.err != nil {
		t.Fatalf("cherry-pick failed: %+v", done)
	}

	got := strings.Fields(strings.ReplaceAll(
		strings.TrimSpace(run(t, dir, "log", "--format=%s", "-3", "main")), " ", "_"))
	want := []string{"feat:_third", "feat:_second", "feat:_first"}
	if !slices.Equal(got, want) {
		t.Errorf("main history = %v, want %v", got, want)
	}
}

func TestCherryPickOntoAnotherBranchChecksItOut(t *testing.T) {
	m, dir := newHistoryModel(t, 120, 32)

	run(t, dir, "branch", "target")
	m = m.reloadAll(t)

	m, _ = m.press(t, "2")
	m = m.selectBranch(t, "feature")
	m = m.openHistoryOfSelected(t)

	m, _ = m.press(t, "y")
	m = m.selectTarget(t, "target")

	if !strings.Contains(plain(m.View()), "checked out first") {
		t.Error("the dialog does not warn that the target will be checked out")
	}

	_, cmd := m.press(t, "enter")
	if done := opResult(t, cmd); done.err != nil {
		t.Fatalf("cherry-pick failed: %+v", done)
	}

	if got := strings.TrimSpace(run(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); got != "target" {
		t.Errorf("branch = %q, want target", got)
	}
}

// A cherry-pick conflict must land in the same resolve/abort flow as a merge.
func TestCherryPickConflictUsesTheOperationDialog(t *testing.T) {
	m, dir := newHistoryModel(t, 120, 32)

	// Make main conflict with feature's first commit.
	write(t, dir, "one.txt", "main's own version\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-m", "main touches one.txt")
	m = m.reloadAll(t)

	m, _ = m.press(t, "2")
	m = m.selectBranch(t, "feature")
	m = m.openHistoryOfSelected(t)
	m = m.selectCommit(t, "feat: first")

	m, _ = m.press(t, "y")
	m = m.selectTarget(t, "main")
	next, cmd := m.press(t, "enter")
	m = next.applyCmd(t, cmd).reloadAll(t)

	if m.opState != git.OpCherryPick {
		t.Fatalf("opState = %q, want %q", m.opState, git.OpCherryPick)
	}
	if !strings.Contains(plain(m.footerView()), "resolve cherry-pick") {
		t.Errorf("footer does not offer to resolve:\n%s", m.footerView())
	}

	m, _ = m.press(t, "m")
	if m.modal != modalOperation {
		t.Fatal("m should open the operation dialog")
	}

	next, cmd = m.press(t, "a")
	m = next.applyCmd(t, cmd).reloadAll(t)
	if m.opState != git.OpNone {
		t.Errorf("opState = %q after abort, want none", m.opState)
	}
}

func TestHistoryViewsFitTerminal(t *testing.T) {
	for _, size := range []struct{ w, h int }{{120, 32}, {80, 24}, {60, 12}, {40, 10}} {
		m, _ := newHistoryModel(t, size.w, size.h)
		m, _ = m.press(t, "2")
		m = m.selectBranch(t, "feature")
		m = m.openHistoryOfSelected(t)

		for _, step := range []struct {
			name string
			keys []string
		}{
			{"history", nil},
			{"detail focused", []string{"tab"}},
			{"cherry-pick", []string{"esc", " ", "y"}},
		} {
			for _, k := range step.keys {
				m, _ = m.press(t, k)
			}

			lines := strings.Split(m.View(), "\n")
			if len(lines) != size.h {
				t.Errorf("%dx%d %s: view is %d lines, want %d",
					size.w, size.h, step.name, len(lines), size.h)
			}
			for i, line := range lines {
				if w := lipglossWidth(line); w > size.w {
					t.Errorf("%dx%d %s: line %d is %d columns, want at most %d",
						size.w, size.h, step.name, i+1, w, size.w)
				}
			}
		}
	}
}

// openHistoryOfSelected presses l on the highlighted branch and settles loads.
func (m Model) openHistoryOfSelected(t *testing.T) Model {
	t.Helper()
	next, cmd := m.press(t, "l")
	m = next
	for range 3 {
		m, cmd = m.step(t, cmd)
	}
	return m
}

// selectCommit moves the history cursor onto the commit with that subject.
func (m Model) selectCommit(t *testing.T, subject string) Model {
	t.Helper()
	for i, c := range m.commits {
		if c.Subject == subject {
			m.commitCursor = i
			return m
		}
	}
	t.Fatalf("no commit titled %q in %v", subject, commitSubjects(m.commits))
	return m
}

// selectTarget moves a dialog's branch picker onto the named branch.
func (m Model) selectTarget(t *testing.T, target string) Model {
	t.Helper()
	i := slices.Index(m.targets, target)
	if i < 0 {
		t.Fatalf("target %q is not offered; picker holds %v", target, m.targets)
	}
	m.targetIndex = i
	return m
}
