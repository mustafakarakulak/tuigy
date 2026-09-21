package ui

import (
	"strings"
	"testing"
)

// typeFilter opens the filter and types a query, settling the loads it starts.
func (m Model) typeFilter(t *testing.T, query string) Model {
	t.Helper()

	next, cmd := m.press(t, "/")
	m = next
	for range 2 {
		m, cmd = m.step(t, cmd)
	}

	for _, r := range query {
		next, cmd = m.press(t, string(r))
		m = next
		for range 2 {
			m, cmd = m.step(t, cmd)
		}
	}
	return m
}

func TestFilterNarrowsTheBranchList(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)
	m, _ = m.press(t, "3")

	before := len(m.branchRows)
	m = m.typeFilter(t, "develop")

	if !m.filtering {
		t.Error("typing should still be in progress")
	}
	if len(m.branchRows) >= before {
		t.Errorf("the list still holds %d rows, want fewer than %d", len(m.branchRows), before)
	}
	if got := m.selectedBranchName_(t); got != "develop" {
		t.Errorf("selected %q, want the match", got)
	}
	if !strings.Contains(plain(m.View()), "/develop") {
		t.Error("the header does not show the query")
	}

	// Accepting keeps the filter and hands the keys back to the actions.
	m, _ = m.press(t, "enter")
	if m.filtering {
		t.Error("enter should stop editing the filter")
	}
	if m.filter != "develop" {
		t.Errorf("filter = %q, want it kept", m.filter)
	}
	if !strings.Contains(plain(m.View()), "of") {
		t.Error("the header does not say how much is hidden")
	}

	// Esc clears it.
	m, _ = m.press(t, "esc")
	if m.filter != "" {
		t.Errorf("filter = %q, want esc to clear it", m.filter)
	}
	if len(m.branchRows) != before {
		t.Errorf("the list holds %d rows, want the original %d back", len(m.branchRows), before)
	}
}

// Abandoning a half-typed filter leaves the list as it was.
func TestFilterCanBeAbandoned(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)
	m, _ = m.press(t, "3")

	before := len(m.branchRows)
	m = m.typeFilter(t, "dev")
	m, _ = m.press(t, "esc")

	if m.filtering || m.filter != "" {
		t.Error("esc should abandon the filter")
	}
	if len(m.branchRows) != before {
		t.Errorf("the list holds %d rows, want %d", len(m.branchRows), before)
	}
}

func TestFilterBackspace(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)
	m, _ = m.press(t, "3")

	m = m.typeFilter(t, "devx")
	if n := countHeaders(m.branchRows); n != 0 {
		t.Fatalf("a query matching nothing still shows %d sections", n)
	}

	next, cmd := m.Update(keyMsg("backspace"))
	m = next.(Model).applyCmd(t, cmd)

	if m.filter != "dev" {
		t.Errorf("filter = %q, want a character removed", m.filter)
	}
	if got := m.selectedBranchName_(t); got != "develop" {
		t.Errorf("selected %q, want develop back", got)
	}
}

func TestFilterNarrowsTheChangesList(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	m = m.typeFilter(t, "app")
	if got := countFiles(m.rows); got != 1 {
		t.Fatalf("%d files shown, want only app.go", got)
	}
	if got := m.selectedPath(t); got != "app.go" {
		t.Errorf("selected %q, want app.go", got)
	}

	// The filter is case-insensitive, since nobody wants to match casing.
	m, _ = m.press(t, "esc")
	m = m.typeFilter(t, "APP")
	if got := countFiles(m.rows); got != 1 {
		t.Errorf("%d files shown for an upper-case query, want 1", got)
	}
}

// The history is paginated, so its filter is handed to git rather than applied
// to whichever page happens to be loaded.
func TestFilterSearchesTheWholeHistory(t *testing.T) {
	m, _ := newHistoryModel(t, 120, 32)
	m, _ = m.press(t, "3")
	m = m.selectBranch(t, "feature").openHistoryOfSelected(t)

	if len(m.commits) != 4 {
		t.Fatalf("%d commits loaded, want the fixture's four", len(m.commits))
	}

	m = m.typeFilter(t, "second")
	if len(m.commits) != 1 {
		t.Fatalf("%d commits match, want 1: %v", len(m.commits), commitSubjects(m.commits))
	}
	if got := m.commits[0].Subject; got != "feat: second" {
		t.Errorf("matched %q", got)
	}

	m, _ = m.press(t, "esc")
	m = m.applyCmd(t, m.applyFilter())
	if len(m.commits) != 4 {
		t.Errorf("%d commits after clearing, want 4", len(m.commits))
	}
}

func TestFilterNarrowsTheStashList(t *testing.T) {
	m, dir := newStashModel(t, 120, 32)

	for _, msg := range []string{"payment retry", "invoice export"} {
		write(t, dir, "service.go", "package payment // "+msg+"\n")
		run(t, dir, "stash", "push", "-m", msg)
	}
	m = m.reloadAll(t).openStashes(t)

	m = m.typeFilter(t, "invoice")
	if len(m.stashes) != 1 {
		t.Fatalf("%d stashes match, want 1", len(m.stashes))
	}
	if m.stashes[0].Message != "invoice export" {
		t.Errorf("matched %q", m.stashes[0].Message)
	}
}

// A filter left on one tab must not quietly hide things on another.
func TestFilterIsClearedWhenTheTabChanges(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	all := countFiles(m.rows)
	m = m.typeFilter(t, "app")
	m, _ = m.press(t, "enter")

	if countFiles(m.rows) >= all {
		t.Fatal("setup: the filter did not narrow anything")
	}

	// Leave for another tab and come back.
	next, cmd := m.press(t, "3")
	m = next.applyCmd(t, cmd)
	if m.filter != "" {
		t.Errorf("filter = %q, want it cleared by the tab change", m.filter)
	}

	next, cmd = m.press(t, "2")
	m = next.applyCmd(t, cmd)
	if got := countFiles(m.rows); got != all {
		t.Errorf("%d files shown, want the original %d back", got, all)
	}
}

// While typing, letters narrow the list instead of triggering actions.
func TestFilterSwallowsActionKeys(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, "/")
	m, _ = m.press(t, "c") // would normally open the commit view
	if m.modal != modalNone {
		t.Error("a letter typed into the filter triggered an action")
	}
	if m.filter != "c" {
		t.Errorf("filter = %q, want the letter", m.filter)
	}
}

func TestCopySelectionPerTab(t *testing.T) {
	// The clipboard write itself is left alone: a test has no business
	// replacing what the person running it had copied.
	changes, _ := newTestModel(t, 120, 32)
	if label, value := changes.selectionForClipboard(); label != "path" || value != "README.md" {
		t.Errorf("changes tab offers (%q, %q), want a file path", label, value)
	}

	m, _ := newHistoryModel(t, 120, 32)
	m, _ = m.press(t, "3")
	m = m.selectBranch(t, "feature")
	if label, value := m.selectionForClipboard(); label != "branch" || value != "feature" {
		t.Errorf("branches tab offers (%q, %q), want the branch name", label, value)
	}

	m = m.openHistoryOfSelected(t)
	label, value := m.selectionForClipboard()
	if label != "commit" || len(value) != 40 {
		t.Errorf("history tab offers (%q, %q), want a full commit hash", label, value)
	}
}

func TestCopyStashRef(t *testing.T) {
	m, dir := newStashModel(t, 120, 32)
	run(t, dir, "stash", "push", "-m", "something")
	m = m.reloadAll(t).openStashes(t)

	if label, value := m.selectionForClipboard(); label != "stash" || value != "stash@{0}" {
		t.Errorf("stashes tab offers (%q, %q), want the stash ref", label, value)
	}
}

// The way out of a screen must survive a narrow terminal.
func TestFooterAlwaysKeepsHelpAndQuit(t *testing.T) {
	for _, width := range []int{120, 80, 60, 44} {
		m, _ := newTestModel(t, width, 24)

		footer := plain(m.footerView())
		if lipglossWidth(m.footerView()) > width {
			t.Errorf("%d columns: footer is %d wide", width, lipglossWidth(footer))
		}
		for _, want := range []string{"? help", "q quit"} {
			if !strings.Contains(footer, want) {
				t.Errorf("%d columns: footer dropped %q:\n%s", width, want, footer)
			}
		}
	}
}
