package ui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mustafakarakulak/tuigy/internal/git"
)

// newTestModel builds a model over a sample repository with its status loaded.
func newTestModel(t *testing.T, width, height int, opts ...Option) (Model, string) {
	t.Helper()

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

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
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	run("init", "-b", "main")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")
	write("README.md", "ilk\n")
	write("app.go", "package main\n")
	write("main.go", "package main\n")
	run("add", ".")
	run("commit", "-m", "initial")

	// STAGED: README.md · UNSTAGED: app.go, main.go · UNTRACKED: yeni.go
	write("README.md", "degisti\n")
	run("add", "README.md")
	write("app.go", "package main // degisti\n")
	write("main.go", "package main // degisti\n")
	write("yeni.go", "package main\n")

	repo, err := git.Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}

	m := New(repo, opts...)
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m = next.(Model)

	st, err := repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	next, _ = m.Update(statusMsg{status: st, state: repo.State()})
	return next.(Model), dir
}

// The view must fit the terminal exactly: one extra line pushes the footer off
// screen, and one over-long line stretches every pane to match it.
func TestViewFitsTerminal(t *testing.T) {
	// Each step names a screen and the keys that reach it from the previous one.
	steps := []struct {
		name string
		keys []string
	}{
		{"changes", nil},
		{"diff focused", []string{"tab"}},
		{"commit", []string{"esc", "c"}},
		{"discard confirm", []string{"esc", "j", "d"}},
		{"help", []string{"esc", "?"}},
	}

	for _, size := range []struct{ w, h int }{{120, 32}, {80, 24}, {200, 60}, {60, 12}, {40, 10}} {
		m, _ := newTestModel(t, size.w, size.h)

		for _, step := range steps {
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

func TestFooterShowsShortcuts(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	footer := m.footerView()
	for _, want := range []string{"space", "commit", "help"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer is missing %q:\n%s", want, footer)
		}
	}
}

// keyMsg builds a bubbletea message from a key name.
func keyMsg(s string) tea.KeyMsg {
	switch s {
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+g":
		return tea.KeyMsg{Type: tea.KeyCtrlG}
	case "ctrl+k":
		return tea.KeyMsg{Type: tea.KeyCtrlK}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+o":
		return tea.KeyMsg{Type: tea.KeyCtrlO}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func (m Model) press(t *testing.T, key string) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(keyMsg(key))
	return next.(Model), cmd
}

// refresh reads the current status from git and applies it to the model.
func (m Model) refresh(t *testing.T) Model {
	t.Helper()
	st, err := m.repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	next, _ := m.Update(statusMsg{status: st, state: m.repo.State()})
	return next.(Model)
}

func (m Model) selectedPath(t *testing.T) string {
	t.Helper()
	r, ok := m.selected()
	if !ok {
		t.Fatal("no file selected")
	}
	return r.file.Path
}

func TestCursorSkipsSectionHeaders(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	// Kurulum: README.md staged, main.go unstaged, yeni.go untracked.
	seen := []string{m.selectedPath(t)}
	for range 3 {
		m, _ = m.press(t, "j")
		seen = append(seen, m.selectedPath(t))
	}

	want := []string{"README.md", "app.go", "main.go"}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("cursor order = %v, want %v", seen, want)
		}
	}

	// At the end of the list the cursor stays put, never landing on a header or past it.
	m, _ = m.press(t, "j")
	if got := m.selectedPath(t); got != "yeni.go" {
		t.Errorf("selection after j on the last row = %q, want %q", got, "yeni.go")
	}
}

// tuigy's core scenario: while an agent in another pane produces files, the
// user's cursor and selection must stay exactly where they were.
func TestSelectionSurvivesBackgroundChanges(t *testing.T) {
	m, dir := newTestModel(t, 120, 32)

	m, _ = m.press(t, "j")
	if got := m.selectedPath(t); got != "app.go" {
		t.Fatalf("setup: selection = %q", got)
	}

	// The agent writes new files underneath us.
	for _, name := range []string{"aaa.go", "bbb.go", "ccc.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package main\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	m = m.refresh(t)

	if got := m.selectedPath(t); got != "app.go" {
		t.Errorf("selection after a background change = %q, want app.go", got)
	}
}

// An unchanged status must leave the model untouched; otherwise the once-a-second
// poll would reset the user's scroll position.
func TestUnchangedStatusIsIgnored(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, "j")
	before := m.cursor

	m = m.refresh(t)
	if m.cursor != before {
		t.Errorf("cursor moved %d → %d, want unchanged", before, m.cursor)
	}
}

func TestToggleStagesAndUnstages(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, "j") // app.go, unstaged
	_, cmd := m.press(t, " ")
	if msg := opResult(t, cmd); msg.err != nil {
		t.Fatalf("stage failed: %+v", msg)
	}

	st := mustUIStatus(t, m)
	if !containsPath(st.Staged(), "app.go") {
		t.Fatal("app.go should have been staged")
	}

	// The same key must unstage a file sitting in the staged section.
	m = m.refresh(t)
	m.cursor = findRow(t, m, secStaged, "app.go")
	_, cmd = m.press(t, " ")
	if msg := opResult(t, cmd); msg.err != nil {
		t.Fatalf("unstage failed: %+v", msg)
	}

	st = mustUIStatus(t, m)
	if containsPath(st.Staged(), "app.go") {
		t.Error("app.go should have been unstaged")
	}
}

// Staging several files in a row must flow: after staging one, the cursor stays
// in the same section on the next file so space can simply be pressed again.
func TestCursorStaysInSectionAfterStaging(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, "j") // app.go
	_, cmd := m.press(t, " ")
	opResult(t, cmd)
	m = m.refresh(t)

	r, ok := m.selected()
	if !ok {
		t.Fatal("no file selected")
	}
	if r.sec != secUnstaged || r.file.Path != "main.go" {
		t.Errorf("selection after staging = %s/%s, want UNSTAGED/main.go", r.sec.title(), r.file.Path)
	}
}

// When a section empties out entirely, follow the file to its new section.
func TestCursorFollowsFileWhenSectionEmpties(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	m.cursor = findRow(t, m, secUntracked, "yeni.go")
	_, cmd := m.press(t, " ")
	opResult(t, cmd)
	m = m.refresh(t)

	r, ok := m.selected()
	if !ok {
		t.Fatal("no file selected")
	}
	if r.file.Path != "yeni.go" || r.sec != secStaged {
		t.Errorf("selection = %s/%s, want STAGED/yeni.go", r.sec.title(), r.file.Path)
	}
}

func TestCommitWithoutStagedChangesShowsError(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	// Unstage the only staged file, then try to commit.
	_, cmd := m.press(t, "A")
	opResult(t, cmd)
	m = m.refresh(t)

	m, _ = m.press(t, "c")
	if m.modal == modalCommit {
		t.Error("the commit view must not open with nothing staged")
	}
	if m.err == nil {
		t.Fatal("the user should have been shown an error")
	}
	if !strings.Contains(plain(m.footerView()), "staged") {
		t.Errorf("the footer does not explain the situation: %s", m.footerView())
	}
}

func TestDiscardOnStagedFileIsRefused(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	// The cursor sits on the staged README.md.
	m, _ = m.press(t, "d")
	if m.modal == modalConfirm {
		t.Error("no confirmation should open for a staged file")
	}
	if m.err == nil {
		t.Fatal("the user should have been told to unstage first")
	}
}

func TestDiscardAsksBeforeDeleting(t *testing.T) {
	m, dir := newTestModel(t, 120, 32)

	m, _ = m.press(t, "j") // main.go, unstaged
	m, _ = m.press(t, "d")

	if m.modal != modalConfirm {
		t.Fatal("a confirmation should have opened")
	}

	// Esc must change nothing on disk.
	before, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	m, _ = m.press(t, "esc")
	if m.modal != modalNone {
		t.Fatal("esc should have closed the confirmation")
	}
	after, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(before) != string(after) {
		t.Error("a cancelled discard still modified the file")
	}
}

func TestTabSwitchesPaneAndEscReturns(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, "tab")
	if m.focus != paneDetail {
		t.Fatal("tab should have moved focus to the diff")
	}
	if !strings.Contains(plain(m.footerView()), "page") {
		t.Errorf("the footer does not show the diff shortcuts: %s", m.footerView())
	}

	m, _ = m.press(t, "esc")
	if m.focus != paneList {
		t.Error("esc should have returned to the file pane")
	}
}

func mustUIStatus(t *testing.T, m Model) *git.Status {
	t.Helper()
	st, err := m.repo.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	return st
}

func containsPath(files []git.FileStatus, path string) bool {
	for _, f := range files {
		if f.Path == path {
			return true
		}
	}
	return false
}

func findRow(t *testing.T, m Model, sec section, path string) int {
	t.Helper()
	for i, r := range m.rows {
		if !r.header && r.sec == sec && r.file.Path == path {
			return i
		}
	}
	t.Fatalf("no row for %s in section %s", path, sec.title())
	return 0
}
