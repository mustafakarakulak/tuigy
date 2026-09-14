package ui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mustafakarakulak/tuigy/internal/git"
)

// newHunkModel builds a model over a file with three separate changes, so each
// one lands in its own hunk.
func newHunkModel(t *testing.T, width, height int) (Model, string) {
	t.Helper()

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	run(t, dir, "init", "-b", "main")
	run(t, dir, "config", "user.email", "t@example.com")
	run(t, dir, "config", "user.name", "t")

	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line " + string(rune('a'+i%26))
	}
	write(t, dir, "file.txt", strings.Join(lines, "\n")+"\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-m", "base")

	lines[2], lines[14], lines[26] = "FIRST", "SECOND", "THIRD"
	write(t, dir, "file.txt", strings.Join(lines, "\n")+"\n")

	repo, err := git.Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}

	m := New(repo)
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m = next.(Model).refresh(t)

	// Load the diff the way the runtime would.
	r, ok := m.selected()
	if !ok {
		t.Fatal("no file selected")
	}
	return m.applyCmd(t, m.loadDiff(r)), dir
}

func TestHunkCursorMovesAndIsMarked(t *testing.T) {
	// A short pane, so the diff genuinely has to scroll.
	m, _ := newHunkModel(t, 120, 16)
	m, _ = m.press(t, "enter")

	if len(m.diffHunks) != 3 {
		t.Fatalf("%d hunks, want 3", len(m.diffHunks))
	}
	if m.hunkCursor != 0 {
		t.Errorf("hunkCursor = %d, want it to start at the first", m.hunkCursor)
	}

	// The hunk under the cursor is marked, and only that one.
	if got := strings.Count(plain(m.diff.View()), "▸"); got != 1 {
		t.Errorf("%d hunks are marked, want exactly one", got)
	}

	m, _ = m.press(t, "]")
	if m.hunkCursor != 1 {
		t.Errorf("hunkCursor = %d after ], want 1", m.hunkCursor)
	}
	// Moving to a hunk brings it into view; how far the pane scrolls depends on
	// how much diff is left below it.
	if !hunkIsVisible(m, 1) {
		t.Errorf("hunk 2 at line %d is outside the view (%d..%d)",
			m.diffHunks[1], m.diff.YOffset, m.diff.YOffset+m.diff.Height)
	}

	m, _ = m.press(t, "[")
	if m.hunkCursor != 0 {
		t.Errorf("hunkCursor = %d after [, want 0", m.hunkCursor)
	}

	// It stops at the ends rather than wrapping.
	m, _ = m.press(t, "[")
	if m.hunkCursor != 0 {
		t.Errorf("hunkCursor = %d past the first hunk, want it to stay", m.hunkCursor)
	}
	for range 5 {
		m, _ = m.press(t, "]")
	}
	if m.hunkCursor != 2 {
		t.Errorf("hunkCursor = %d past the last hunk, want 2", m.hunkCursor)
	}
}

// The point of the feature: one change goes in, the others stay behind.
// hunkIsVisible reports whether a hunk header is inside the pane's window.
func hunkIsVisible(m Model, hunk int) bool {
	at := m.diffHunks[hunk]
	return at >= m.diff.YOffset && at < m.diff.YOffset+m.diff.Height
}

func TestStagingOneHunkLeavesTheRest(t *testing.T) {
	m, dir := newHunkModel(t, 120, 40)
	m, _ = m.press(t, "enter")
	m, _ = m.press(t, "]") // the second change

	next, cmd := m.press(t, " ")
	done := opResult(t, cmd)
	if done.err != nil {
		t.Fatalf("staging the hunk failed: %v", done.err)
	}
	if !strings.Contains(done.label, "hunk 2 of 3") {
		t.Errorf("reported %q, want it to say which hunk", done.label)
	}
	m = next.refresh(t)

	staged := run(t, dir, "diff", "--cached")
	if !strings.Contains(staged, "SECOND") {
		t.Errorf("the chosen change was not staged:\n%s", staged)
	}
	for _, other := range []string{"FIRST", "THIRD"} {
		if strings.Contains(staged, other) {
			t.Errorf("%s was staged as well:\n%s", other, staged)
		}
	}

	// The file is now on both sides, which is what partial staging means.
	st := mustUIStatus(t, m)
	if !containsPath(st.Staged(), "file.txt") || !containsPath(st.Unstaged(), "file.txt") {
		t.Error("the file should be both staged and unstaged")
	}
}

func TestUnstagingOneHunk(t *testing.T) {
	m, dir := newHunkModel(t, 120, 40)

	run(t, dir, "add", "file.txt")
	m = m.refresh(t)
	m.cursor = findRow(t, m, secStaged, "file.txt")
	m = m.applyCmd(t, m.syncDiff())

	m, _ = m.press(t, "enter")
	if !strings.Contains(plain(m.footerView()), "unstage this hunk") {
		t.Errorf("the footer does not say which way the hunk would move:\n%s", plain(m.footerView()))
	}

	next, cmd := m.press(t, " ")
	if done := opResult(t, cmd); done.err != nil {
		t.Fatalf("unstaging the hunk failed: %v", done.err)
	}
	m = next.refresh(t)

	staged := run(t, dir, "diff", "--cached")
	if strings.Contains(staged, "FIRST") {
		t.Errorf("the first hunk is still staged:\n%s", staged)
	}
	for _, kept := range []string{"SECOND", "THIRD"} {
		if !strings.Contains(staged, kept) {
			t.Errorf("%s was taken out too:\n%s", kept, staged)
		}
	}
}

// An untracked file has no hunks to move, so it says so rather than failing
// somewhere inside git apply.
func TestHunkStagingOnAnUntrackedFile(t *testing.T) {
	m, dir := newHunkModel(t, 120, 40)

	write(t, dir, "brand-new.txt", "hello\n")
	m = m.refresh(t)
	m.cursor = findRow(t, m, secUntracked, "brand-new.txt")
	m = m.applyCmd(t, m.syncDiff())

	m, _ = m.press(t, "enter")
	after, cmd := m.press(t, " ")

	if cmd != nil {
		t.Error("nothing should have been run for an untracked file")
	}
	if after.err == nil {
		t.Fatal("the user should be told why this does not work")
	}
	if !strings.Contains(after.err.Error(), "whole file") {
		t.Errorf("error = %q, want it to say what to do instead", after.err)
	}
}

// Selecting another file starts its diff from the first hunk.
func TestHunkCursorResetsWithTheDiff(t *testing.T) {
	m, dir := newHunkModel(t, 120, 40)
	m, _ = m.press(t, "enter")
	m, _ = m.press(t, "]")
	if m.hunkCursor == 0 {
		t.Fatal("setup: the cursor did not move")
	}

	write(t, dir, "other.txt", "another file\n")
	m = m.refresh(t)
	m.cursor = findRow(t, m, secUntracked, "other.txt")
	m = m.applyCmd(t, m.syncDiff())

	if m.hunkCursor != 0 {
		t.Errorf("hunkCursor = %d after changing file, want 0", m.hunkCursor)
	}
}
