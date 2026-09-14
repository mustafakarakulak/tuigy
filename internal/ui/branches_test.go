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

// newBranchModel builds a model on the branches tab of a repository holding
// main (current), develop and a remote-tracking origin/main.
func newBranchModel(t *testing.T, width, height int) (Model, string) {
	t.Helper()

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	remote, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	run := func(wd string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = wd
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run(remote, "init", "--bare", "-b", "main")
	run(dir, "init", "-b", "main")
	run(dir, "config", "user.email", "t@example.com")
	run(dir, "config", "user.name", "t")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("first\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run(dir, "add", ".")
	run(dir, "commit", "-m", "initial")
	run(dir, "remote", "add", "origin", remote)
	run(dir, "push", "-u", "origin", "main")

	// develop carries a file main does not have, so switching to it while that
	// file is modified locally is genuinely blocked.
	run(dir, "checkout", "-qb", "develop")
	if err := os.WriteFile(filepath.Join(dir, "only-on-develop.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run(dir, "add", ".")
	run(dir, "commit", "-m", "develop work")
	run(dir, "checkout", "-q", "main")

	repo, err := git.Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}

	m := New(repo)
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m = next.(Model)
	m, _ = m.press(t, "2")
	return m.reloadAll(t), dir
}

// reloadAll applies a fresh status and branch list to the model.
func (m Model) reloadAll(t *testing.T) Model {
	t.Helper()
	ctx := context.Background()

	st, err := m.repo.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	next, _ := m.Update(statusMsg{status: st, state: m.repo.State()})
	m = next.(Model)

	branches, err := m.repo.Branches(ctx)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	next, _ = m.Update(branchesMsg{branches: branches})
	return next.(Model)
}

func (m Model) selectedBranchName_(t *testing.T) string {
	t.Helper()
	b, ok := m.selectedBranch()
	if !ok {
		t.Fatal("no branch selected")
	}
	return b.Name
}

func TestTabSwitchingShowsBranches(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)

	if m.tab != tabBranches {
		t.Fatal("pressing 2 should show the branches tab")
	}
	view := m.View()
	for _, want := range []string{"LOCAL", "develop", "origin/main"} {
		if !strings.Contains(view, want) {
			t.Errorf("branches view is missing %q", want)
		}
	}

	m, _ = m.press(t, "1")
	if m.tab != tabChanges {
		t.Error("pressing 1 should return to the changes tab")
	}
}

// The list is ordered by recency, so opening the tab must not leave the cursor
// on whichever branch happens to be newest.
func TestBranchCursorStartsOnCurrentBranch(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)

	if got := m.selectedBranchName_(t); got != "main" {
		t.Errorf("selected branch = %q, want the current branch main", got)
	}
}

func TestBranchCursorSkipsSectionHeaders(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)

	// The list is ordered by recency, so start at the top and walk the whole way.
	m, _ = m.press(t, "g")

	seen := map[string]bool{}
	for range len(m.branchRows) {
		seen[m.selectedBranchName_(t)] = true
		m, _ = m.press(t, "j")
	}

	for _, want := range []string{"main", "develop", "origin/main"} {
		if !seen[want] {
			t.Errorf("cursor never reached %q; visited %v", want, seen)
		}
	}
}

func TestCheckoutSwitchesBranch(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)

	m = m.selectBranch(t, "develop")
	_, cmd := m.press(t, "enter")
	if cmd == nil {
		t.Fatal("enter should start a checkout")
	}
	if msg := opResult(t, cmd); msg.err != nil {
		t.Fatalf("checkout failed: %+v", msg)
	}

	m = m.reloadAll(t)
	if got := m.status.Branch; got != "develop" {
		t.Errorf("branch = %q, want develop", got)
	}
}

// Checking out a remote-tracking branch creates a local branch that tracks it.
func TestCheckoutRemoteBranchCreatesLocalTrackingBranch(t *testing.T) {
	m, dir := newBranchModel(t, 120, 32)

	run(t, dir, "push", "-u", "origin", "develop")
	run(t, dir, "branch", "-D", "develop")
	m = m.reloadAll(t)

	m = m.selectBranch(t, "origin/develop")
	_, cmd := m.press(t, "enter")
	if msg := opResult(t, cmd); msg.err != nil {
		t.Fatalf("checkout failed: %+v", msg)
	}

	m = m.reloadAll(t)
	if m.status.Branch != "develop" {
		t.Errorf("branch = %q, want develop", m.status.Branch)
	}
	if m.status.Upstream != "origin/develop" {
		t.Errorf("upstream = %q, want origin/develop", m.status.Upstream)
	}
}

func TestCheckoutOnCurrentBranchDoesNothing(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)

	if _, cmd := m.press(t, "enter"); cmd != nil {
		t.Error("checking out the branch you are already on should be a no-op")
	}
}

// A blocked switch must ask before stashing; work never disappears silently.
func TestBlockedCheckoutAsksBeforeStashing(t *testing.T) {
	m, dir := newBranchModel(t, 120, 32)

	// Modify a file develop does not have, which blocks the switch.
	write(t, dir, "only-on-main.txt", "x\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-m", "main work")
	write(t, dir, "only-on-main.txt", "modified\n")
	m = m.reloadAll(t)

	m = m.selectBranch(t, "develop")
	_, cmd := m.press(t, "enter")

	msg, ok := cmd().(checkoutFailedMsg)
	if !ok {
		t.Fatalf("expected the checkout to fail, got %T", cmd())
	}

	next, _ := m.Update(msg)
	m = next.(Model)
	if m.modal != modalConfirm {
		t.Fatal("a blocked switch should open a confirmation")
	}
	if !strings.Contains(m.confirm.detail, "Stash") {
		t.Errorf("the dialog does not offer to stash: %q", m.confirm.detail)
	}

	// Nothing may have happened yet.
	if out := run(t, dir, "stash", "list"); strings.TrimSpace(out) != "" {
		t.Errorf("a stash was created without asking:\n%s", out)
	}
	if got := strings.TrimSpace(run(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); got != "main" {
		t.Errorf("branch = %q, the switch should not have happened", got)
	}

	// Confirming stashes and switches.
	m, cmd = m.press(t, "enter")
	if done := opResult(t, cmd); done.err != nil {
		t.Fatalf("stash and switch failed: %+v", done)
	}
	if got := strings.TrimSpace(run(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); got != "develop" {
		t.Errorf("branch = %q, want develop", got)
	}
	if out := run(t, dir, "stash", "list"); !strings.Contains(out, "tuigy") {
		t.Errorf("no stash was created:\n%s", out)
	}
}

func TestDeleteRefusesCurrentAndRemoteBranches(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)

	m, _ = m.press(t, "D") // cursor is on the current branch
	if m.modal == modalConfirm {
		t.Error("no confirmation should open for the current branch")
	}
	if m.err == nil {
		t.Fatal("deleting the current branch should be refused with an error")
	}

	m = m.selectBranch(t, "origin/main")
	m, _ = m.press(t, "D")
	if m.modal == modalConfirm {
		t.Error("no confirmation should open for a remote branch")
	}
	if m.err == nil {
		t.Fatal("deleting a remote branch should be refused with an error")
	}
}

func TestDeleteBranchAsksThenDeletes(t *testing.T) {
	m, dir := newBranchModel(t, 120, 32)

	m = m.selectBranch(t, "develop")
	m, _ = m.press(t, "D")
	if m.modal != modalConfirm {
		t.Fatal("a confirmation should open")
	}

	m, cmd := m.press(t, "enter")
	// develop holds unmerged commits, so git refuses without force. The point
	// is that the refusal surfaces instead of the branch quietly disappearing.
	done := opResult(t, cmd)
	if done.err == nil {
		t.Fatal("deleting an unmerged branch should have been refused")
	}
	if out := run(t, dir, "branch", "--list", "develop"); !strings.Contains(out, "develop") {
		t.Error("develop should still exist")
	}
}

func TestNewBranchUsesSelectedBranchAsSource(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)

	m = m.selectBranch(t, "develop")
	m, _ = m.press(t, "n")

	if m.modal != modalNewBranch {
		t.Fatal("n should open the new-branch view")
	}
	if m.branchFrom != "develop" {
		t.Errorf("source = %q, want the selected branch develop", m.branchFrom)
	}
	if !strings.Contains(plain(m.View()), "develop") {
		t.Error("the view does not show which branch the new one comes from")
	}
}

func TestNewBranchCreatesFromSource(t *testing.T) {
	m, dir := newBranchModel(t, 120, 32)

	m = m.selectBranch(t, "develop")
	m, _ = m.press(t, "n")
	for _, r := range "feature/x" {
		m, _ = m.press(t, string(r))
	}

	m, cmd := m.press(t, "enter")
	if done := opResult(t, cmd); done.err != nil {
		t.Fatalf("branch creation failed: %+v", done)
	}
	if m.modal != modalNone {
		t.Error("the view should close after creating the branch")
	}

	if got := strings.TrimSpace(run(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); got != "feature/x" {
		t.Fatalf("branch = %q, want feature/x", got)
	}
	// Branched from develop, so develop's file must be present.
	if _, err := os.Stat(filepath.Join(dir, "only-on-develop.txt")); err != nil {
		t.Error("the branch was not created from develop")
	}
}

func TestNewBranchRejectsEmptyName(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)

	m, _ = m.press(t, "n")
	m, cmd := m.press(t, "enter")

	if cmd != nil {
		t.Error("an empty name should not start a git command")
	}
	if m.modal != modalNewBranch {
		t.Error("the view should stay open")
	}
	if m.err == nil {
		t.Fatal("the user should be told the name is required")
	}
}

func TestBranchesFooterShowsRelevantShortcuts(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)

	footer := m.footerView()
	for _, want := range []string{"new branch", "fetch", "push"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer is missing %q:\n%s", want, footer)
		}
	}
	// The cursor starts on the current branch, so switching is not on offer.
	if strings.Contains(footer, "switch to branch") {
		t.Errorf("footer offers to switch to the branch already checked out:\n%s", footer)
	}

	m = m.selectBranch(t, "develop")
	if !strings.Contains(plain(m.footerView()), "switch to branch") {
		t.Errorf("footer does not offer to switch:\n%s", m.footerView())
	}
}

// selectBranch moves the cursor onto the named branch.
func (m Model) selectBranch(t *testing.T, name string) Model {
	t.Helper()
	for i, r := range m.branchRows {
		if !r.header && r.branch.Name == name {
			m.branchCur = i
			return m
		}
	}
	t.Fatalf("no branch named %q in the list", name)
	return m
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// The branches tab must respect the terminal just as strictly as the changes tab.
func TestBranchesViewFitsTerminal(t *testing.T) {
	for _, size := range []struct{ w, h int }{{120, 32}, {80, 24}, {60, 12}, {40, 10}} {
		m, _ := newBranchModel(t, size.w, size.h)

		for _, view := range []string{"list", "help", "new branch"} {
			switch view {
			case "help":
				m, _ = m.press(t, "?")
			case "new branch":
				m, _ = m.press(t, "esc")
				m, _ = m.press(t, "n")
			}

			lines := strings.Split(m.View(), "\n")
			if len(lines) != size.h {
				t.Errorf("%dx%d %s: view is %d lines, want %d", size.w, size.h, view, len(lines), size.h)
			}
			for i, line := range lines {
				if w := lipglossWidth(line); w > size.w {
					t.Errorf("%dx%d %s: line %d is %d columns, want at most %d",
						size.w, size.h, view, i+1, w, size.w)
				}
			}
		}
	}
}
