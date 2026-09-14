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

// newMergeModel builds a model on the branches tab of a repository shaped to
// exercise all three merge outcomes:
//
//	main       current, conflicts with feature over shared.txt
//	feature    one commit past the root, conflicting
//	develop    sits at the root, so it can be fast-forwarded
//	sidetrack  has its own commit, so merging into it needs a checkout
func newMergeModel(t *testing.T, width, height int) (Model, string) {
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

	run(t, dir, "branch", "develop")

	run(t, dir, "checkout", "-qb", "sidetrack")
	write(t, dir, "sidetrack.txt", "x\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-m", "sidetrack work")

	run(t, dir, "checkout", "-q", "main")
	run(t, dir, "checkout", "-qb", "feature")
	write(t, dir, "shared.txt", "feature side\n")
	run(t, dir, "commit", "-am", "feature side")

	run(t, dir, "checkout", "-q", "main")
	write(t, dir, "shared.txt", "main side\n")
	run(t, dir, "commit", "-am", "main side")

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

// applyCmd runs a command and feeds everything it produces back to the model.
func (m Model) applyCmd(t *testing.T, cmd tea.Cmd) Model {
	t.Helper()
	m, _ = m.step(t, cmd)
	return m
}

// openMergeFrom opens the merge dialog with the named branch as the source.
func (m Model) openMergeFrom(t *testing.T, source string) Model {
	t.Helper()
	m = m.selectBranch(t, source)
	m, cmd := m.press(t, "m")
	if m.modal != modalMerge {
		t.Fatal("m should open the merge dialog")
	}
	return m.applyCmd(t, cmd)
}

// selectMergeTarget moves the dialog's picker onto the named branch, using the
// same keys a user would.
func (m Model) selectMergeTarget(t *testing.T, target string) Model {
	t.Helper()

	want := slices.Index(m.targets, target)
	if want < 0 {
		t.Fatalf("target %q is not offered; picker holds %v", target, m.targets)
	}

	stroke := "j"
	if want < m.targetIndex {
		stroke = "k"
	}

	for range len(m.targets) {
		if m.targetIndex == want {
			return m
		}
		next, cmd := m.press(t, stroke)
		m = next.applyCmd(t, cmd)
	}

	t.Fatalf("could not reach target %q; picker holds %v", target, m.targets)
	return m
}

func TestMergeDialogDefaultsToCurrentBranch(t *testing.T) {
	m, _ := newMergeModel(t, 120, 32)

	m = m.openMergeFrom(t, "feature")

	if m.mergeSource != "feature" {
		t.Errorf("source = %q, want the selected branch feature", m.mergeSource)
	}
	if got := m.currentTarget(); got != "main" {
		t.Errorf("target = %q, want the current branch main", got)
	}
	// A branch cannot be merged into itself, so it must not be offered.
	for _, name := range m.targets {
		if name == "feature" {
			t.Error("the source branch must not appear among the targets")
		}
	}

	view := m.View()
	if !strings.Contains(view, "feature") || !strings.Contains(view, "main") {
		t.Error("the dialog does not show source and target")
	}
}

// The dialog must say which of the three things a merge will actually do,
// because two of them move the branch you are on.
func TestMergeDialogExplainsWhatWillHappen(t *testing.T) {
	m, _ := newMergeModel(t, 120, 32)
	m = m.openMergeFrom(t, "feature")

	cases := []struct {
		target string
		want   string
	}{
		{"main", "branch you are on"},
		{"develop", "fast-forward"},
		{"sidetrack", "checked out first"},
	}

	for _, c := range cases {
		m = m.selectMergeTarget(t, c.target)
		if m.mergePlan == nil {
			t.Fatalf("target %s: no plan was worked out", c.target)
		}
		if got := m.mergePlan.summary(); !strings.Contains(got, c.want) {
			t.Errorf("target %s: summary = %q, want it to mention %q", c.target, got, c.want)
		}
		if !strings.Contains(plain(m.View()), c.want) {
			t.Errorf("target %s: the dialog does not show the consequence", c.target)
		}
	}
}

// Fast-forwarding into a branch you are not on must not move you or touch the
// working tree, which is the whole reason for preferring it to a checkout.
func TestMergeFastForwardLeavesYouWhereYouAre(t *testing.T) {
	m, dir := newMergeModel(t, 120, 32)

	m = m.openMergeFrom(t, "feature")
	m = m.selectMergeTarget(t, "develop")

	_, cmd := m.press(t, "enter")
	if done := opResult(t, cmd); done.err != nil {
		t.Fatalf("merge failed: %+v", done)
	}

	if got := strings.TrimSpace(run(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); got != "main" {
		t.Errorf("branch = %q, a fast-forward must not check anything out", got)
	}
	develop := strings.TrimSpace(run(t, dir, "rev-parse", "develop"))
	feature := strings.TrimSpace(run(t, dir, "rev-parse", "feature"))
	if develop != feature {
		t.Errorf("develop = %s, want it advanced to feature at %s", develop, feature)
	}
}

func TestMergeConflictOffersResolveAndAbort(t *testing.T) {
	m, dir := newMergeModel(t, 120, 32)

	m = m.openMergeFrom(t, "feature")
	m = m.selectMergeTarget(t, "main")

	next, cmd := m.press(t, "enter")
	m = next.applyCmd(t, cmd) // the merge conflicts, so this reports an error
	m = m.reloadAll(t)

	if m.opState != git.OpMerge {
		t.Fatalf("opState = %q, want %q", m.opState, git.OpMerge)
	}
	if got := m.status.Conflicted(); len(got) != 1 || got[0].Path != "shared.txt" {
		t.Fatalf("conflicted files = %v, want [shared.txt]", got)
	}

	// The footer must lead with resolving, and must not still offer to start a
	// new merge on the same key.
	footer := m.footerView()
	if !strings.Contains(footer, "resolve merge") {
		t.Errorf("footer does not offer to resolve the merge:\n%s", footer)
	}
	if strings.Contains(footer, "m merge") {
		t.Errorf("footer still offers to start a merge on the same key:\n%s", footer)
	}

	// The dialog must name the conflicted files and spell out what abort costs.
	m, _ = m.press(t, "m")
	if m.modal != modalOperation {
		t.Fatal("m should open the in-progress operation dialog")
	}
	view := m.View()
	for _, want := range []string{"shared.txt", "abort", "continue"} {
		if !strings.Contains(view, want) {
			t.Errorf("the dialog does not mention %q", want)
		}
	}

	next, cmd = m.press(t, "a")
	m = next.applyCmd(t, cmd)
	m = m.reloadAll(t)

	if m.opState != git.OpNone {
		t.Errorf("opState = %q after abort, want none", m.opState)
	}
	if !m.status.IsClean() {
		t.Error("abort should have restored a clean working tree")
	}
	if out := run(t, dir, "log", "--oneline", "--merges"); strings.TrimSpace(out) != "" {
		t.Errorf("abort should not have left a merge commit:\n%s", out)
	}
}

func TestMergeConflictContinueAfterResolving(t *testing.T) {
	m, dir := newMergeModel(t, 120, 32)

	m = m.openMergeFrom(t, "feature")
	m = m.selectMergeTarget(t, "main")
	next, cmd := m.press(t, "enter")
	m = next.applyCmd(t, cmd).reloadAll(t)

	if m.opState != git.OpMerge {
		t.Fatalf("setup: opState = %q", m.opState)
	}

	// Resolve the way a user would: edit the file, then stage it from the list.
	write(t, dir, "shared.txt", "resolved by hand\n")
	m, _ = m.press(t, "1")
	m = m.reloadAll(t)

	next, cmd = m.press(t, " ")
	m = next.applyCmd(t, cmd).reloadAll(t)

	if got := len(m.status.Conflicted()); got != 0 {
		t.Fatalf("%d conflicts left after staging, want 0", got)
	}

	m, _ = m.press(t, "m")
	if !strings.Contains(plain(m.View()), "resolved and staged") {
		t.Error("the dialog does not say the merge is ready to continue")
	}

	next, cmd = m.press(t, "enter")
	m = next.applyCmd(t, cmd).reloadAll(t)

	if m.opState != git.OpNone {
		t.Errorf("opState = %q after continue, want none", m.opState)
	}
	if out := run(t, dir, "log", "--oneline", "--merges"); strings.TrimSpace(out) == "" {
		t.Error("continuing should have created the merge commit")
	}
	if body, err := os.ReadFile(filepath.Join(dir, "shared.txt")); err != nil {
		t.Fatalf("ReadFile: %v", err)
	} else if strings.TrimSpace(string(body)) != "resolved by hand" {
		t.Errorf("shared.txt = %q, the manual resolution was lost", body)
	}
}

// Every dialog this milestone adds has to respect the terminal too.
func TestMergeDialogsFitTerminal(t *testing.T) {
	for _, size := range []struct{ w, h int }{{120, 32}, {80, 24}, {60, 12}, {40, 10}} {
		m, _ := newMergeModel(t, size.w, size.h)
		m = m.openMergeFrom(t, "feature")

		lines := strings.Split(m.View(), "\n")
		if len(lines) != size.h {
			t.Errorf("%dx%d merge dialog: view is %d lines, want %d", size.w, size.h, len(lines), size.h)
		}
		for i, line := range lines {
			if w := lipglossWidth(line); w > size.w {
				t.Errorf("%dx%d merge dialog: line %d is %d columns, want at most %d",
					size.w, size.h, i+1, w, size.w)
			}
		}
	}
}

// Resolving a conflict means leaving tuigy, so the dialog says which editor
// that will be rather than leaving the user to find out.
func TestOperationDialogNamesTheEditor(t *testing.T) {
	m, _ := newMergeModel(t, 120, 32)
	m.editor = "/opt/homebrew/bin/micro"

	m = m.openMergeFrom(t, "feature")
	m = m.selectMergeTarget(t, "main")
	next, cmd := m.press(t, "enter")
	m = next.applyCmd(t, cmd).reloadAll(t)

	m, _ = m.press(t, "m")
	if m.modal != modalOperation {
		t.Fatal("m should open the operation dialog")
	}
	if got := plain(m.View()); !strings.Contains(got, "Resolve each one in micro") {
		t.Errorf("the dialog does not name the editor:\n%s", got)
	}
}
