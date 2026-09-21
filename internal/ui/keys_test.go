package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestChangesNavigation(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	// g and G reach the ends of the list whatever section they are in.
	m, _ = m.press(t, "G")
	if got := m.selectedPath(t); got != "yeni.go" {
		t.Errorf("G selected %q, want the last file", got)
	}

	m, _ = m.press(t, "g")
	if got := m.selectedPath(t); got != "README.md" {
		t.Errorf("g selected %q, want the first file", got)
	}

	// k moves back up, and stops rather than wrapping.
	m, _ = m.press(t, "j")
	m, _ = m.press(t, "k")
	if got := m.selectedPath(t); got != "README.md" {
		t.Errorf("k selected %q, want README.md", got)
	}
	m, _ = m.press(t, "k")
	if got := m.selectedPath(t); got != "README.md" {
		t.Errorf("k past the top selected %q, want it to stay put", got)
	}
}

func TestDiffPaneScrollingAndReturn(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, "enter")
	if m.focus != paneDetail {
		t.Fatal("enter should focus the diff")
	}

	// Scrolling keys go to the diff rather than moving the file cursor.
	before := m.cursor
	m, _ = m.press(t, "ctrl+d")
	m, _ = m.press(t, "ctrl+u")
	if m.cursor != before {
		t.Error("scrolling the diff moved the file cursor")
	}

	m, _ = m.press(t, "esc")
	if m.focus != paneList {
		t.Error("esc should return to the file list")
	}
}

func TestExplicitStageAndUnstageKeys(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, "j") // app.go, unstaged
	_, cmd := m.press(t, "s")
	if done := opResult(t, cmd); done.err != nil {
		t.Fatalf("s failed to stage: %+v", done)
	}

	m = m.refresh(t)
	m.cursor = findRow(t, m, secStaged, "app.go")
	_, cmd = m.press(t, "u")
	if done := opResult(t, cmd); done.err != nil {
		t.Fatalf("u failed to unstage: %+v", done)
	}

	if containsPath(mustUIStatus(t, m).Staged(), "app.go") {
		t.Error("app.go should have been unstaged")
	}
}

func TestStageAllKey(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	_, cmd := m.press(t, "a")
	if done := opResult(t, cmd); done.err != nil {
		t.Fatalf("a failed: %+v", done)
	}

	st := mustUIStatus(t, m)
	if len(st.Unstaged()) != 0 || len(st.Untracked()) != 0 {
		t.Errorf("a left %d unstaged and %d untracked files",
			len(st.Unstaged()), len(st.Untracked()))
	}
}

func TestAmendLoadsTheLastMessage(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	next, cmd := m.press(t, "C")
	m = next.applyCmd(t, cmd)

	if m.modal != modalCommit {
		t.Fatal("C should open the commit view")
	}
	if !m.amending {
		t.Error("the view should be in amend mode")
	}
	if got := m.commit.Value(); got != "initial" {
		t.Errorf("the field holds %q, want the last commit's message", got)
	}
	if !strings.Contains(plain(m.View()), "Amend") {
		t.Error("the view does not say it is amending")
	}

	next, cmd = m.press(t, "ctrl+s")
	if done := opResult(t, cmd); done.err != nil {
		t.Fatalf("amend failed: %+v", done)
	}
	if next.amending {
		t.Error("amend mode should be over")
	}
}

func TestOpenEditorKey(t *testing.T) {
	m, dir := newTestModel(t, 120, 32)

	// A normal file hands the terminal to the editor.
	if _, cmd := m.press(t, "e"); cmd == nil {
		t.Error("e should start the editor")
	}

	// A file that is gone cannot be opened, and says so rather than failing
	// somewhere inside the editor.
	if err := os.Remove(filepath.Join(dir, "app.go")); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	m = m.refresh(t)
	m.cursor = findRow(t, m, secUnstaged, "app.go")

	after, cmd := m.press(t, "e")
	if cmd != nil {
		t.Error("a deleted file should not be opened")
	}
	if after.err == nil {
		t.Fatal("the user should be told the file is gone")
	}
}

func TestRemoteKeys(t *testing.T) {
	m, dir := newBranchModel(t, 120, 32)

	for _, c := range []struct {
		key, label string
	}{
		{"f", "fetched"},
		{"F", "fetched all remotes"},
		{"p", "pulled"},
	} {
		_, cmd := m.press(t, c.key)
		if cmd == nil {
			t.Fatalf("%s produced no command", c.key)
		}
		done := opResult(t, cmd)
		if done.err != nil {
			t.Errorf("%s: %v", c.key, done.err)
		}
		if !strings.Contains(done.label, c.label) {
			t.Errorf("%s reported %q, want it to mention %q", c.key, done.label, c.label)
		}
	}

	// Pushing a branch with no upstream sets one.
	run(t, dir, "checkout", "-qb", "feature/fresh")
	write(t, dir, "fresh.txt", "x\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-m", "fresh work")
	m = m.reloadAll(t)

	_, cmd := m.press(t, "P")
	done := opResult(t, cmd)
	if done.err != nil {
		t.Fatalf("push failed: %+v", done)
	}
	if !strings.Contains(done.label, "upstream") {
		t.Errorf("push reported %q, want it to mention setting the upstream", done.label)
	}
	if got := mustUIStatus(t, m).Upstream; got != "origin/feature/fresh" {
		t.Errorf("upstream = %q, want origin/feature/fresh", got)
	}
}

func TestPushWhileDetached(t *testing.T) {
	m, dir := newBranchModel(t, 120, 32)

	head := strings.TrimSpace(run(t, dir, "rev-parse", "HEAD"))
	run(t, dir, "checkout", "-q", head)
	m = m.reloadAll(t)

	after, cmd := m.press(t, "P")
	if cmd != nil {
		t.Error("pushing a detached HEAD should not run anything")
	}
	if after.err == nil {
		t.Fatal("the user should be told why it cannot push")
	}
	if !strings.Contains(after.err.Error(), "detached") {
		t.Errorf("error = %q", after.err)
	}
}

func TestRefreshAndQuitKeys(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	after, cmd := m.press(t, "r")
	if cmd == nil {
		t.Error("r should reload")
	}
	if after.statusFP != "" {
		t.Error("r should clear the fingerprint so the next result is applied")
	}

	_, cmd = m.press(t, "q")
	if cmd == nil {
		t.Fatal("q should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q produced %T, want a quit", cmd())
	}
}

func TestBranchNavigation(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)

	m, _ = m.press(t, "G")
	last := m.selectedBranchName_(t)
	m, _ = m.press(t, "g")
	first := m.selectedBranchName_(t)

	if first == last {
		t.Errorf("g and G both selected %q", first)
	}

	m, _ = m.press(t, "j")
	m, _ = m.press(t, "k")
	if got := m.selectedBranchName_(t); got != first {
		t.Errorf("k returned to %q, want %q", got, first)
	}
}

func TestHistoryNavigationKeys(t *testing.T) {
	m, _ := newHistoryModel(t, 120, 32)
	m, _ = m.press(t, "3")
	m = m.selectBranch(t, "feature").openHistoryOfSelected(t)

	m, _ = m.press(t, "G")
	if got := m.commits[m.commitCursor].Subject; got != "initial" {
		t.Errorf("G selected %q, want the oldest commit", got)
	}

	m, _ = m.press(t, "g")
	if got := m.commits[m.commitCursor].Subject; got != "feat: third" {
		t.Errorf("g selected %q, want the newest commit", got)
	}

	m, _ = m.press(t, "ctrl+d")
	deep := m.commitCursor
	m, _ = m.press(t, "ctrl+u")
	if m.commitCursor >= deep && deep != 0 {
		t.Error("ctrl+u did not scroll back up")
	}

	m, _ = m.press(t, "j")
	m, _ = m.press(t, "k")
	if m.commitCursor != 0 {
		t.Errorf("cursor = %d, want it back at the top", m.commitCursor)
	}
}

func TestStashNavigationKeys(t *testing.T) {
	m, dir := newStashModel(t, 120, 32)

	for _, msg := range []string{"oldest", "middle", "newest"} {
		write(t, dir, "service.go", "package payment // "+msg+"\n")
		run(t, dir, "stash", "push", "-m", msg)
	}
	m = m.reloadAll(t).openStashes(t)

	m, _ = m.press(t, "G")
	if got := m.stashes[m.stashCursor].Message; got != "oldest" {
		t.Errorf("G selected %q, want the oldest stash", got)
	}

	m, _ = m.press(t, "g")
	if got := m.stashes[m.stashCursor].Message; got != "newest" {
		t.Errorf("g selected %q, want the newest stash", got)
	}

	m, _ = m.press(t, "j")
	if got := m.stashes[m.stashCursor].Message; got != "middle" {
		t.Errorf("j selected %q, want the middle stash", got)
	}
	m, _ = m.press(t, "k")
	if got := m.stashes[m.stashCursor].Message; got != "newest" {
		t.Errorf("k selected %q, want the newest stash", got)
	}

	// The detail pane scrolls rather than moving the list cursor.
	m, _ = m.press(t, "tab")
	if m.focus != paneDetail {
		t.Fatal("tab should focus the stash contents")
	}
	before := m.stashCursor
	m, _ = m.press(t, "ctrl+d")
	if m.stashCursor != before {
		t.Error("scrolling the pane moved the stash cursor")
	}
	m, _ = m.press(t, "esc")
	if m.focus != paneList {
		t.Error("esc should return to the list")
	}
}

func TestDialogsCancel(t *testing.T) {
	m, _ := newMergeModel(t, 120, 32)

	// Merge dialog.
	m = m.openMergeFrom(t, "feature")
	m, _ = m.press(t, "esc")
	if m.modal != modalNone || m.mergeSource != "" {
		t.Error("esc should close and clear the merge dialog")
	}

	// New branch dialog.
	m, _ = m.press(t, "n")
	m, _ = m.press(t, "esc")
	if m.modal != modalNone || m.branchFrom != "" {
		t.Error("esc should close and clear the new branch dialog")
	}

	// Confirmation dialogs also close on q, which reads as "no".
	m = m.selectBranch(t, "develop")
	m, _ = m.press(t, "D")
	if m.modal != modalConfirm {
		t.Fatal("D should ask first")
	}
	m, _ = m.press(t, "q")
	if m.modal != modalNone {
		t.Error("q should dismiss a confirmation")
	}
}

func TestCherryPickDialogNavigation(t *testing.T) {
	m, _ := newHistoryModel(t, 120, 32)
	m, _ = m.press(t, "3")
	m = m.selectBranch(t, "feature").openHistoryOfSelected(t)

	m, _ = m.press(t, "y")
	if m.modal != modalCherryPick {
		t.Fatal("y should open the cherry-pick dialog")
	}

	if len(m.targets) < 2 {
		t.Fatalf("the picker holds %v, which is too few to navigate", m.targets)
	}

	// The picker stops at the ends rather than wrapping.
	for range len(m.targets) + 2 {
		m, _ = m.press(t, "k")
	}
	if m.targetIndex != 0 {
		t.Fatalf("targetIndex = %d, want it clamped to the first target", m.targetIndex)
	}

	m, _ = m.press(t, "j")
	if m.targetIndex != 1 {
		t.Error("j did not move the target picker down")
	}
	m, _ = m.press(t, "k")
	if m.targetIndex != 0 {
		t.Error("k did not move the target picker back up")
	}
	for range len(m.targets) + 2 {
		m, _ = m.press(t, "j")
	}
	if want := len(m.targets) - 1; m.targetIndex != want {
		t.Errorf("targetIndex = %d, want it clamped to %d", m.targetIndex, want)
	}

	m, _ = m.press(t, "esc")
	if m.modal != modalNone || len(m.pickCommits) != 0 {
		t.Error("esc should close and clear the cherry-pick dialog")
	}
}

func TestCherryPickWithoutASelection(t *testing.T) {
	m, _ := newHistoryModel(t, 120, 32)
	m, _ = m.press(t, "4")

	// An empty history has nothing to pick.
	m.commits = nil
	after, _ := m.press(t, "y")
	if after.modal == modalCherryPick {
		t.Error("the dialog should not open with nothing to pick")
	}
	if after.err == nil {
		t.Error("the user should be told to select a commit")
	}
}

// Which editor "e" opens is answered where someone would look for the key.
func TestHelpNamesTheEditor(t *testing.T) {
	m, _ := newTestModel(t, 120, 32, WithEditor("/opt/homebrew/bin/micro"))

	m, _ = m.press(t, "?")
	if got := plain(m.helpContent()); !strings.Contains(got, "open in micro") {
		t.Errorf("the help screen does not name the editor:\n%s", got)
	}
	if got := plain(m.helpContent()); !strings.Contains(got, "--init-config") {
		t.Error("the help screen does not say how to change it")
	}
}

// The configured editor reaches the model, and "e" runs something.
func TestConfiguredEditorIsUsed(t *testing.T) {
	m, _ := newTestModel(t, 120, 32, WithEditor("my-editor --wait"))

	if m.editor != "my-editor --wait" {
		t.Fatalf("editor = %q, want the configured command", m.editor)
	}
	if _, cmd := m.press(t, "e"); cmd == nil {
		t.Error("e should start the editor")
	}
}
