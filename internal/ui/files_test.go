package ui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newTreeModel is a test model whose repository has directories in it, which is
// the only kind a tree has anything to say about.
func newTreeModel(t *testing.T, width, height int) (Model, string) {
	t.Helper()

	m, dir := newTestModel(t, width, height)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	write("internal/git/repo.go", "package git\n")
	write("internal/ui/view.go", "package ui\n")
	write("cmd/tuigy/main.go", "package main\n")
	write(".gitignore", "ignored/\n")
	write("ignored/junk.txt", "junk\n")

	// Committed, so that the tree holds both changed and unchanged files: one
	// directory that opens itself and one that does not.
	run("add", "internal", "cmd", ".gitignore")
	run("commit", "-m", "a tree to look at")
	write("internal/git/repo.go", "package git // changed\n")

	paths, err := m.repo.Files(context.Background())
	if err != nil {
		t.Fatalf("Files: %v", err)
	}

	m = m.refresh(t)
	m, _ = m.press(t, "1")
	next, _ := m.Update(filesMsg{paths: paths})
	return next.(Model), dir
}

// ---------------------------------------------------------------- the tree

func TestTreeGroupsPathsByDirectory(t *testing.T) {
	root := buildTree([]string{"cmd/tuigy/main.go", "go.mod", "internal/git/repo.go"})

	// Directories come before files, each alphabetically.
	var top []string
	for _, c := range root.children {
		top = append(top, c.name)
	}
	if want := []string{"cmd", "internal", "go.mod"}; !equalStrings(top, want) {
		t.Fatalf("top level = %v, want %v", top, want)
	}

	cmdDir := root.children[0]
	if !cmdDir.dir || cmdDir.path != "cmd" {
		t.Fatalf("cmd = %+v, want a directory at path cmd", cmdDir)
	}
	if len(cmdDir.children) != 1 || cmdDir.children[0].name != "tuigy" {
		t.Fatalf("cmd holds %+v, want one directory tuigy", cmdDir.children)
	}
	if leaf := cmdDir.children[0].children[0]; leaf.dir || leaf.path != "cmd/tuigy/main.go" {
		t.Errorf("the leaf is %+v, want the file cmd/tuigy/main.go", leaf)
	}
}

// A closed directory shows nothing of what is inside it, which is what makes a
// large repository openable at all.
func TestClosedDirectoriesHideTheirContents(t *testing.T) {
	root := buildTree([]string{"internal/git/repo.go", "go.mod"})

	rows := flattenTree(root, nil)
	if len(rows) != 2 {
		t.Fatalf("got %d rows with everything closed, want 2", len(rows))
	}

	rows = flattenTree(root, map[string]bool{"internal": true})
	if len(rows) != 3 {
		t.Fatalf("got %d rows with internal open, want 3", len(rows))
	}

	rows = flattenTree(root, map[string]bool{"internal": true, "internal/git": true})
	if len(rows) != 4 {
		t.Fatalf("got %d rows with both open, want 4", len(rows))
	}
}

func TestExpandToOpensEveryDirectoryAbove(t *testing.T) {
	expanded := map[string]bool{}
	expandTo(expanded, "internal/ui/view.go")

	for _, want := range []string{"internal", "internal/ui"} {
		if !expanded[want] {
			t.Errorf("%q was not opened", want)
		}
	}
	if expanded["internal/ui/view.go"] {
		t.Error("the file itself was opened as if it were a directory")
	}
}

// A filter unfolds the tree to what it matched: a result you still have to go
// looking for is not a result.
func TestFilteringOpensTheWayToEachMatch(t *testing.T) {
	paths := []string{"cmd/tuigy/main.go", "internal/git/repo.go", "internal/ui/view.go"}

	rows := buildFileRows(paths, "repo", nil)

	var shown []string
	for _, r := range rows {
		shown = append(shown, r.node.path)
	}
	want := []string{"internal", "internal/git", "internal/git/repo.go"}
	if !equalStrings(shown, want) {
		t.Errorf("filtered rows = %v, want %v", shown, want)
	}
}

func TestDirectoriesCarryAChangeMark(t *testing.T) {
	m, _ := newTreeModel(t, 120, 32)

	changed := changedPaths(m.status)
	if !dirHasChanges(changed, "internal") {
		t.Error("internal holds changed files but is not marked")
	}
	if dirHasChanges(changed, "nowhere") {
		t.Error("a directory with nothing in it was marked as changed")
	}
}

// ---------------------------------------------------------------- the tab

func TestFilesTabOpensOnTheChangedDirectories(t *testing.T) {
	m, _ := newTreeModel(t, 120, 32)

	// internal/git/repo.go has changed, so the tree opens the way down to it
	// rather than making the user unfold three levels to find it.
	list := plain(m.View())
	for _, want := range []string{"internal/", "repo.go"} {
		if !strings.Contains(list, want) {
			t.Errorf("the tree does not show %q:\n%s", want, list)
		}
	}
}

func TestOpeningAndClosingADirectory(t *testing.T) {
	m, _ := newTreeModel(t, 120, 32)

	m = m.selectTreePath(t, "cmd")
	if m.expanded["cmd"] {
		t.Fatal("cmd started open")
	}

	m, _ = m.press(t, "right")
	if !m.expanded["cmd"] {
		t.Fatal("right did not open the directory")
	}
	m, _ = m.press(t, "left")
	if m.expanded["cmd"] {
		t.Fatal("left did not close the directory")
	}

	// enter is the other way to do the same thing.
	m, _ = m.press(t, "enter")
	if !m.expanded["cmd"] {
		t.Error("enter did not open the directory")
	}
}

// Closing with the cursor on something that is not an open directory steps out
// to the directory holding it, which is how you walk back up.
func TestClosingFromInsideStepsOut(t *testing.T) {
	m, _ := newTreeModel(t, 120, 32)

	m = m.selectTreePath(t, "internal/git/repo.go")
	m, _ = m.press(t, "left")

	if got := m.selectedTreePath(); got != "internal/git" {
		t.Errorf("left from a file landed on %q, want internal/git", got)
	}
}

func TestSelectingAFileLoadsItsContents(t *testing.T) {
	m, _ := newTreeModel(t, 120, 32)

	m = m.selectTreePath(t, "internal/git/repo.go")
	if m.previewKey != "internal/git/repo.go" {
		t.Fatalf("the preview is showing %q, want internal/git/repo.go", m.previewKey)
	}

	msg, ok := m.loadPreview("internal/git/repo.go")().(previewMsg)
	if !ok {
		t.Fatal("loadPreview did not produce a previewMsg")
	}
	if !strings.Contains(msg.text, "package git") {
		t.Errorf("the preview is missing the file's contents: %q", msg.text)
	}

	next, _ := m.Update(msg)
	if body := plain(next.(Model).preview.View()); !strings.Contains(body, "package git") {
		t.Errorf("the pane does not show the file:\n%s", body)
	}
}

// A directory has no contents to show, so the pane says what is in it instead.
func TestSelectingADirectorySummarisesIt(t *testing.T) {
	m, _ := newTreeModel(t, 120, 32)

	m = m.selectTreePath(t, "internal")
	body := plain(m.preview.View())

	if !strings.Contains(body, "internal/") {
		t.Errorf("the summary does not name the directory:\n%s", body)
	}
	if !strings.Contains(body, "file") {
		t.Errorf("the summary does not count what is inside:\n%s", body)
	}
}

// What git ignores is not part of the repository, and a tree full of build
// output would be useless.
func TestIgnoredFilesAreNotInTheTree(t *testing.T) {
	m, _ := newTreeModel(t, 120, 32)

	for _, path := range m.allFiles {
		if strings.HasPrefix(path, "ignored/") {
			t.Errorf("%q is ignored by git but is in the tree", path)
		}
	}
}

func TestCopyingFromTheTreeTakesThePath(t *testing.T) {
	m, _ := newTreeModel(t, 120, 32)
	m = m.selectTreePath(t, "internal/git/repo.go")

	label, value := m.selectionForClipboard()
	if label != "path" || value != "internal/git/repo.go" {
		t.Errorf("clipboard = (%q, %q), want (path, internal/git/repo.go)", label, value)
	}
}

func TestFilteringTheTreeCountsWhatIsShown(t *testing.T) {
	m, _ := newTreeModel(t, 120, 32)

	m, _ = m.press(t, "/")
	for _, r := range "repo" {
		m, _ = m.press(t, string(r))
	}

	shown, total := m.filteredCount()
	if shown != 1 {
		t.Errorf("the filter shows %d files, want 1", shown)
	}
	if total != len(m.allFiles) {
		t.Errorf("the total is %d, want %d", total, len(m.allFiles))
	}
}

// ---------------------------------------------------------------- preview

func TestPreviewDeclinesWhatItCannotShow(t *testing.T) {
	if note := previewNote([]byte("hello"), 5); note != "" {
		t.Errorf("a small text file was declined: %q", note)
	}
	if note := previewNote([]byte("bin\x00ary"), 8); !strings.Contains(note, "binary") {
		t.Errorf("a binary file gave %q, want it named as binary", note)
	}
	if note := previewNote([]byte("x"), maxPreviewBytes+1); !strings.Contains(note, "too large") {
		t.Errorf("an oversized file gave %q, want it named as too large", note)
	}
}

func TestPreviewNumbersItsLines(t *testing.T) {
	body := plain(renderPreview("first\nsecond\n", 40))

	if !strings.Contains(body, "1 first") || !strings.Contains(body, "2 second") {
		t.Errorf("the preview has no line numbers:\n%s", body)
	}
}

func TestHumanSizeReadsAsPeopleWriteIt(t *testing.T) {
	for _, tc := range []struct {
		n    int64
		want string
	}{
		{512, "512 bytes"},
		{2048, "2.0 kB"},
		{3 << 20, "3.0 MB"},
	} {
		if got := humanSize(tc.n); got != tc.want {
			t.Errorf("humanSize(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------- helpers

// selectTreePath moves the cursor onto a path, opening whatever is above it.
func (m Model) selectTreePath(t *testing.T, path string) Model {
	t.Helper()

	expandTo(m.expanded, path)
	m.rebuildFileRows()

	for i, r := range m.fileRows {
		if r.node.path == path {
			m.fileCursor = i
			m.ensureFileVisible()
			if cmd := m.syncPreview(); cmd != nil {
				next, _ := m.Update(cmd())
				return next.(Model)
			}
			return m
		}
	}

	t.Fatalf("%q is not in the tree", path)
	return m
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------- folding

// L opens everything, H puts it all away again.
func TestFoldingTheWholeTree(t *testing.T) {
	m, _ := newTreeModel(t, 120, 40)

	m, _ = m.press(t, "L")
	for _, want := range []string{"cmd", "cmd/tuigy", "internal", "internal/git", "internal/ui"} {
		if !m.expanded[want] {
			t.Errorf("L left %q closed", want)
		}
	}
	if !m.showsPath("cmd/tuigy/main.go") {
		t.Error("L did not bring a file three levels down onto the screen")
	}

	m, _ = m.press(t, "H")
	if len(m.expanded) != 0 {
		t.Errorf("H left %d directories open, want none", len(m.expanded))
	}

	// What is left is the top level, which is what the tab would have opened on.
	for _, r := range m.fileRows {
		if strings.Contains(r.node.path, "/") {
			t.Errorf("H left %q on screen, which is not top level", r.node.path)
		}
	}
}

// Closing the tree must not strand the cursor on a row that is no longer drawn.
func TestFoldingTheTreeTakesTheCursorWithIt(t *testing.T) {
	m, _ := newTreeModel(t, 120, 40)

	m = m.selectTreePath(t, "internal/git/repo.go")
	m, _ = m.press(t, "H")

	if got := m.selectedTreePath(); got != "internal" {
		t.Errorf("after H the cursor is on %q, want the top-level internal", got)
	}
}

// + and - act on the folder under the cursor and everything beneath it.
func TestFoldingOneSubtree(t *testing.T) {
	m, _ := newTreeModel(t, 120, 40)

	m = m.selectTreePath(t, "cmd")
	m, _ = m.press(t, "+")

	if !m.expanded["cmd"] || !m.expanded["cmd/tuigy"] {
		t.Fatalf("+ did not open the whole subtree: %v", m.expanded)
	}
	if !m.showsPath("cmd/tuigy/main.go") {
		t.Error("+ did not bring the files inside onto the screen")
	}
	// The rest of the tree is left exactly as it was.
	if !m.expanded["internal"] {
		t.Error("+ on cmd disturbed what was open elsewhere")
	}

	m, _ = m.press(t, "-")
	if m.expanded["cmd"] || m.expanded["cmd/tuigy"] {
		t.Errorf("- did not close the whole subtree: %v", m.expanded)
	}
	if m.showsPath("cmd/tuigy/main.go") {
		t.Error("- left a file inside the closed folder on the screen")
	}
	if got := m.selectedTreePath(); got != "cmd" {
		t.Errorf("after - the cursor is on %q, want the folder that closed", got)
	}
}

// Opening a folder again shows one level, not however many were unfolded
// inside it before it was closed.
func TestClosingASubtreeForgetsWhatWasOpenInside(t *testing.T) {
	m, _ := newTreeModel(t, 120, 40)

	m = m.selectTreePath(t, "cmd")
	m, _ = m.press(t, "+")
	m, _ = m.press(t, "-")
	m, _ = m.press(t, "right")

	if !m.expanded["cmd"] {
		t.Fatal("the folder did not reopen")
	}
	if m.expanded["cmd/tuigy"] {
		t.Error("reopening the folder also reopened what was inside it")
	}
}

// With the cursor on a file, the subtree keys act on the folder holding it:
// that is the thing there is to fold.
func TestFoldingASubtreeFromAFileInside(t *testing.T) {
	m, _ := newTreeModel(t, 120, 40)

	m = m.selectTreePath(t, "internal/git/repo.go")
	m, _ = m.press(t, "-")

	if m.expanded["internal/git"] {
		t.Error("- from a file did not close the folder holding it")
	}
	if got := m.selectedTreePath(); got != "internal/git" {
		t.Errorf("after - the cursor is on %q, want internal/git", got)
	}
}

// showsPath reports whether a path is currently drawn in the tree.
func (m Model) showsPath(path string) bool {
	for _, r := range m.fileRows {
		if r.node.path == path {
			return true
		}
	}
	return false
}
