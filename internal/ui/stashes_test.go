package ui

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mustafakarakulak/tuigy/internal/git"
)

// newStashModel builds a model over a repository with a dirty working tree and
// no stashes yet.
func newStashModel(t *testing.T, width, height int) (Model, string) {
	t.Helper()

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	run(t, dir, "init", "-b", "main")
	run(t, dir, "config", "user.email", "t@example.com")
	run(t, dir, "config", "user.name", "t")

	write(t, dir, "service.go", "package payment\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-m", "initial")

	write(t, dir, "service.go", "package payment // work in progress\n")
	write(t, dir, "scratch.txt", "untracked\n")

	repo, err := git.Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}

	m := New(repo)
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return next.(Model).reloadAll(t), dir
}

// openStashes switches to the stash tab and settles the loads it starts.
func (m Model) openStashes(t *testing.T) Model {
	t.Helper()
	next, cmd := m.press(t, "4")
	m = next
	for range 3 {
		m, cmd = m.step(t, cmd)
	}
	return m
}

func stashMessages(stashes []git.Stash) []string {
	out := make([]string, len(stashes))
	for i, s := range stashes {
		out[i] = s.Message
	}
	return out
}

func TestStashPushFromTheChangesTab(t *testing.T) {
	m, dir := newStashModel(t, 120, 32)

	m, _ = m.press(t, "S")
	if m.modal != modalStash {
		t.Fatal("S should open the stash dialog")
	}
	if !strings.Contains(m.View(), "untracked files stay where they are") {
		t.Error("the dialog does not say untracked files are left alone")
	}

	for _, r := range "half done" {
		m, _ = m.press(t, string(r))
	}
	next, cmd := m.press(t, "enter")
	m = next.applyCmd(t, cmd)

	if m.modal != modalNone {
		t.Error("the dialog should close once the stash is made")
	}

	out := run(t, dir, "stash", "list", "--format=%gs")
	if !strings.Contains(out, "half done") {
		t.Errorf("the stash was not created with the message given:\n%s", out)
	}
	if got := strings.TrimSpace(run(t, dir, "status", "--porcelain", "service.go")); got != "" {
		t.Errorf("service.go is still modified after stashing: %q", got)
	}
	// Untracked files are deliberately left behind.
	if _, err := os.Stat(filepath.Join(dir, "scratch.txt")); err != nil {
		t.Error("the untracked file should still be on disk")
	}
}

func TestStashPushWithNothingToStash(t *testing.T) {
	m, dir := newStashModel(t, 120, 32)

	run(t, dir, "checkout", "--", ".")
	m = m.reloadAll(t)

	m, _ = m.press(t, "S")
	if m.modal == modalStash {
		t.Error("the dialog should not open when there is nothing to stash")
	}
	if m.err == nil {
		t.Fatal("the user should be told there is nothing to stash")
	}
}

func TestStashesTabListsStashes(t *testing.T) {
	m, dir := newStashModel(t, 120, 32)

	run(t, dir, "stash", "push", "-m", "first")
	write(t, dir, "service.go", "package payment // more\n")
	run(t, dir, "stash", "push", "-m", "second")
	m = m.reloadAll(t).openStashes(t)

	if m.tab != tabStashes {
		t.Fatal("pressing 4 should show the stashes tab")
	}
	if got := stashMessages(m.stashes); !slices.Equal(got, []string{"second", "first"}) {
		t.Fatalf("stashes = %v, want the newest first", got)
	}

	view := m.View()
	for _, want := range []string{"second", "first", "stash@{0}"} {
		if !strings.Contains(view, want) {
			t.Errorf("the stash view is missing %q", want)
		}
	}
	if !strings.Contains(view, "package payment // more") {
		t.Error("the detail pane does not show the selected stash's diff")
	}
}

func TestStashPopRestoresAndRemoves(t *testing.T) {
	m, dir := newStashModel(t, 120, 32)

	run(t, dir, "stash", "push", "-m", "pop me")
	m = m.reloadAll(t).openStashes(t)

	next, cmd := m.press(t, "enter")
	m = next.applyCmd(t, cmd).reloadAll(t).openStashes(t)

	if len(m.stashes) != 0 {
		t.Errorf("%d stashes after pop, want none", len(m.stashes))
	}
	body, err := os.ReadFile(filepath.Join(dir, "service.go"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(body), "work in progress") {
		t.Errorf("service.go = %q, the stashed change was not restored", body)
	}
}

func TestStashApplyKeepsTheStash(t *testing.T) {
	m, dir := newStashModel(t, 120, 32)

	run(t, dir, "stash", "push", "-m", "keep me")
	m = m.reloadAll(t).openStashes(t)

	next, cmd := m.press(t, "a")
	m = next.applyCmd(t, cmd).reloadAll(t).openStashes(t)

	if got := stashMessages(m.stashes); !slices.Equal(got, []string{"keep me"}) {
		t.Errorf("stashes = %v, want the stash kept", got)
	}
	if got := strings.TrimSpace(run(t, dir, "status", "--porcelain", "service.go")); got == "" {
		t.Error("the change was not applied to the working tree")
	}
}

// Dropping discards work, so it has to be confirmed and has to say so.
func TestStashDropAsksFirst(t *testing.T) {
	m, dir := newStashModel(t, 120, 32)

	run(t, dir, "stash", "push", "-m", "drop me")
	m = m.reloadAll(t).openStashes(t)

	m, _ = m.press(t, "D")
	if m.modal != modalConfirm {
		t.Fatal("dropping a stash should ask first")
	}
	if !strings.Contains(m.confirm.detail, "discarded") {
		t.Errorf("the dialog does not say the changes are discarded: %q", m.confirm.detail)
	}

	// Cancelling must leave it alone.
	m, _ = m.press(t, "esc")
	if out := run(t, dir, "stash", "list"); !strings.Contains(out, "drop me") {
		t.Error("a cancelled drop removed the stash anyway")
	}

	m, _ = m.press(t, "D")
	next, cmd := m.press(t, "enter")
	m = next.applyCmd(t, cmd).reloadAll(t).openStashes(t)

	if len(m.stashes) != 0 {
		t.Errorf("%d stashes after dropping, want none", len(m.stashes))
	}
}

// "D" deletes a branch on one tab and drops a stash on another, so the footer
// has to say which.
func TestStashFooterRelabelsTheDeleteKey(t *testing.T) {
	m, dir := newStashModel(t, 120, 32)

	run(t, dir, "stash", "push", "-m", "something")
	m = m.reloadAll(t).openStashes(t)

	footer := m.footerView()
	if !strings.Contains(footer, "drop stash") {
		t.Errorf("footer does not label D as dropping a stash:\n%s", footer)
	}
	if strings.Contains(footer, "delete branch") {
		t.Errorf("footer still labels D as deleting a branch:\n%s", footer)
	}
	for _, want := range []string{"pop", "apply"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer is missing %q:\n%s", want, footer)
		}
	}
}

// Four labelled tabs do not fit a narrow terminal, so the bar has to shrink
// rather than push the layout wider.
func TestTabBarShrinksOnNarrowTerminals(t *testing.T) {
	wide, _ := newStashModel(t, 120, 32)
	if got := wide.tabsView(); !strings.Contains(got, "Branches") || !strings.Contains(got, "Stashes") {
		t.Errorf("a wide terminal should label every tab: %q", got)
	}

	narrow, _ := newStashModel(t, 40, 12)
	got := narrow.tabsView()
	if lipglossWidth(got) > 40 {
		t.Errorf("tab bar is %d columns on a 40 column terminal: %q", lipglossWidth(got), got)
	}
	if !strings.Contains(got, "Changes") {
		t.Errorf("the active tab should still be named: %q", got)
	}
	if strings.Contains(got, "Stashes") {
		t.Errorf("inactive tabs should shrink to their number: %q", got)
	}
}

func TestStashViewsFitTerminal(t *testing.T) {
	for _, size := range []struct{ w, h int }{{120, 32}, {80, 24}, {60, 12}, {40, 10}} {
		m, dir := newStashModel(t, size.w, size.h)
		run(t, dir, "stash", "push", "-m", "a stash with a fairly long message attached")
		m = m.reloadAll(t).openStashes(t)

		for _, step := range []struct {
			name string
			keys []string
		}{
			{"stashes", nil},
			{"detail focused", []string{"tab"}},
			{"drop confirm", []string{"esc", "D"}},
			{"stash dialog", []string{"esc", "S"}},
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
