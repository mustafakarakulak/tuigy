package git

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseStashSubject(t *testing.T) {
	cases := []struct {
		in              string
		branch, message string
	}{
		{"On main: my own message", "main", "my own message"},
		{"WIP on main: cae0671 initial", "main", "initial"},
		{"On feature/x: fix the thing", "feature/x", "fix the thing"},
		{"something unexpected", "", "something unexpected"},
	}

	for _, c := range cases {
		branch, message := parseStashSubject(c.in)
		if branch != c.branch || message != c.message {
			t.Errorf("parseStashSubject(%q) = (%q, %q), want (%q, %q)",
				c.in, branch, message, c.branch, c.message)
		}
	}
}

func TestStashListAndDiff(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "README.md", "first change\n")
	if err := repo.Stash(ctx, ""); err != nil {
		t.Fatalf("Stash: %v", err)
	}
	writeFile(t, dir, "README.md", "second change\n")
	if err := repo.Stash(ctx, "work in progress"); err != nil {
		t.Fatalf("Stash: %v", err)
	}

	if !mustStatus(t, repo).IsClean() {
		t.Error("stashing should have left a clean working tree")
	}

	stashes, err := repo.Stashes(ctx)
	if err != nil {
		t.Fatalf("Stashes: %v", err)
	}
	if len(stashes) != 2 {
		t.Fatalf("%d stashes, want 2", len(stashes))
	}

	// Most recent first.
	if stashes[0].Message != "work in progress" {
		t.Errorf("first stash message = %q", stashes[0].Message)
	}
	if stashes[0].Ref != "stash@{0}" {
		t.Errorf("first stash ref = %q, want stash@{0}", stashes[0].Ref)
	}
	if stashes[0].Branch != "main" {
		t.Errorf("first stash branch = %q, want main", stashes[0].Branch)
	}
	if stashes[0].Date.IsZero() {
		t.Error("stash date is zero")
	}

	diff, err := repo.StashDiff(ctx, stashes[0].Ref)
	if err != nil {
		t.Fatalf("StashDiff: %v", err)
	}
	if !strings.Contains(diff, "+second change") {
		t.Errorf("stash diff is missing the change:\n%s", diff)
	}
}

// Untracked files are deliberately left alone, matching plain `git stash`.
func TestStashLeavesUntrackedFilesAlone(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "README.md", "changed\n")
	writeFile(t, dir, "scratch.txt", "not tracked\n")

	if err := repo.Stash(ctx, ""); err != nil {
		t.Fatalf("Stash: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "scratch.txt")); err != nil {
		t.Error("the untracked file should still be on disk")
	}
	if got := paths(mustStatus(t, repo).Untracked()); !slices.Equal(got, []string{"scratch.txt"}) {
		t.Errorf("Untracked() = %v, want [scratch.txt]", got)
	}
}

func TestStashApplyKeepsTheStash(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "README.md", "changed\n")
	if err := repo.Stash(ctx, "keep me"); err != nil {
		t.Fatalf("Stash: %v", err)
	}

	if err := repo.StashApply(ctx, "stash@{0}"); err != nil {
		t.Fatalf("StashApply: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.TrimSpace(string(body)) != "changed" {
		t.Errorf("README.md = %q, the change was not restored", body)
	}

	stashes, err := repo.Stashes(ctx)
	if err != nil {
		t.Fatalf("Stashes: %v", err)
	}
	if len(stashes) != 1 {
		t.Errorf("%d stashes after apply, want the stash kept", len(stashes))
	}
}

func TestStashPopRemovesTheStash(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "README.md", "changed\n")
	if err := repo.Stash(ctx, "pop me"); err != nil {
		t.Fatalf("Stash: %v", err)
	}

	if err := repo.StashPop(ctx, "stash@{0}"); err != nil {
		t.Fatalf("StashPop: %v", err)
	}

	if got := paths(mustStatus(t, repo).Unstaged()); !slices.Equal(got, []string{"README.md"}) {
		t.Errorf("Unstaged() = %v, want the change back", got)
	}

	stashes, err := repo.Stashes(ctx)
	if err != nil {
		t.Fatalf("Stashes: %v", err)
	}
	if len(stashes) != 0 {
		t.Errorf("%d stashes after pop, want none", len(stashes))
	}
}

// Operations take a ref rather than an index, because dropping one renumbers
// everything below it.
func TestStashDropTargetsTheRightEntry(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	for _, msg := range []string{"oldest", "middle", "newest"} {
		writeFile(t, dir, "README.md", msg+"\n")
		if err := repo.Stash(ctx, msg); err != nil {
			t.Fatalf("Stash: %v", err)
		}
	}

	// stash@{1} is "middle".
	if err := repo.StashDrop(ctx, "stash@{1}"); err != nil {
		t.Fatalf("StashDrop: %v", err)
	}

	stashes, err := repo.Stashes(ctx)
	if err != nil {
		t.Fatalf("Stashes: %v", err)
	}

	var messages []string
	for _, s := range stashes {
		messages = append(messages, s.Message)
	}
	if want := []string{"newest", "oldest"}; !slices.Equal(messages, want) {
		t.Errorf("stashes = %v, want %v", messages, want)
	}
}

func TestStashOperationsRejectAnEmptyRef(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	if err := repo.StashApply(ctx, ""); err == nil {
		t.Error("apply with no ref should fail")
	}
	if err := repo.StashDrop(ctx, ""); err == nil {
		t.Error("drop with no ref should fail")
	}
	if _, err := repo.StashDiff(ctx, ""); err == nil {
		t.Error("diff with no ref should fail")
	}
}

func TestStashesOnRepositoryWithNoStashes(t *testing.T) {
	repo, _ := newTestRepo(t)

	stashes, err := repo.Stashes(context.Background())
	if err != nil {
		t.Fatalf("Stashes: %v", err)
	}
	if len(stashes) != 0 {
		t.Errorf("%d stashes, want none", len(stashes))
	}
}
