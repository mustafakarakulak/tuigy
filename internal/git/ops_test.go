package git

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestStageAndUnstage(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "README.md", "degisti\n")
	writeFile(t, dir, "yeni.txt", "yeni\n")

	if err := repo.Stage(ctx, "README.md", "yeni.txt"); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	st := mustStatus(t, repo)
	got := paths(st.Staged())
	slices.Sort(got)
	if want := []string{"README.md", "yeni.txt"}; !slices.Equal(got, want) {
		t.Fatalf("Staged() = %v, istenen %v", got, want)
	}

	if err := repo.Unstage(ctx, "README.md"); err != nil {
		t.Fatalf("Unstage: %v", err)
	}

	st = mustStatus(t, repo)
	if got, want := paths(st.Staged()), []string{"yeni.txt"}; !slices.Equal(got, want) {
		t.Errorf("Staged() = %v, istenen %v", got, want)
	}
	// Unstage must leave the working tree alone: the file is still modified.
	if got, want := paths(st.Unstaged()), []string{"README.md"}; !slices.Equal(got, want) {
		t.Errorf("Unstaged() = %v, istenen %v", got, want)
	}
}

func TestStageAllAndUnstageAll(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "README.md", "degisti\n")
	writeFile(t, dir, "yeni.txt", "yeni\n")

	if err := repo.StageAll(ctx); err != nil {
		t.Fatalf("StageAll: %v", err)
	}
	if n := len(mustStatus(t, repo).Staged()); n != 2 {
		t.Fatalf("staged files after StageAll = %d, want 2", n)
	}

	if err := repo.UnstageAll(ctx); err != nil {
		t.Fatalf("UnstageAll: %v", err)
	}
	st := mustStatus(t, repo)
	if n := len(st.Staged()); n != 0 {
		t.Errorf("staged files after UnstageAll = %d, want 0", n)
	}
	if n := len(st.Unstaged()) + len(st.Untracked()); n != 2 {
		t.Errorf("UnstageAll lost changes: %d files left, want 2", n)
	}
}

func TestDiscard(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "README.md", "degisti\n")
	writeFile(t, dir, "yeni.txt", "yeni\n")

	st := mustStatus(t, repo)
	for _, f := range append(st.Unstaged(), st.Untracked()...) {
		if err := repo.Discard(ctx, f); err != nil {
			t.Fatalf("Discard(%s): %v", f.Path, err)
		}
	}

	if !mustStatus(t, repo).IsClean() {
		t.Error("working tree should be clean after Discard")
	}
	if _, err := os.Stat(filepath.Join(dir, "yeni.txt")); !os.IsNotExist(err) {
		t.Error("takipsiz dosya diskten silinmeliydi")
	}
}

func TestCommit(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "yeni.txt", "icerik\n")
	if err := repo.Stage(ctx, "yeni.txt"); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if err := repo.Commit(ctx, "feat: yeni dosya ekle"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if !mustStatus(t, repo).IsClean() {
		t.Error("working tree should be clean after Commit")
	}

	msg, err := repo.LastCommitMessage(ctx)
	if err != nil {
		t.Fatalf("LastCommitMessage: %v", err)
	}
	if msg != "feat: yeni dosya ekle" {
		t.Errorf("LastCommitMessage() = %q", msg)
	}
}

func TestCommitRejectsEmptyMessage(t *testing.T) {
	repo, _ := newTestRepo(t)
	if err := repo.Commit(context.Background(), "   "); err == nil {
		t.Error("committing with an empty message should fail")
	}
}

// With nothing staged, git's error must reach the user rather than panic.
func TestCommitWithNothingStagedReturnsError(t *testing.T) {
	repo, _ := newTestRepo(t)
	err := repo.Commit(context.Background(), "bos commit")
	if err == nil {
		t.Fatal("committing with nothing staged should fail")
	}
	var cmdErr *CmdError
	if !errorAs(err, &cmdErr) {
		t.Fatalf("hata tipi = %T, *CmdError bekleniyordu", err)
	}
}

func TestAmend(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "yeni.txt", "icerik\n")
	if err := repo.Stage(ctx, "yeni.txt"); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if err := repo.Amend(ctx, "initial: duzeltildi"); err != nil {
		t.Fatalf("Amend: %v", err)
	}

	msg, err := repo.LastCommitMessage(ctx)
	if err != nil {
		t.Fatalf("LastCommitMessage: %v", err)
	}
	if msg != "initial: duzeltildi" {
		t.Errorf("LastCommitMessage() = %q", msg)
	}
	if c := strings.TrimSpace(gitRun(t, dir, "rev-list", "--count", "HEAD")); c != "1" {
		t.Errorf("commit count = %s, amend must not create a new commit", c)
	}
}

func TestFileDiff(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "README.md", "degisti\n")
	st := mustStatus(t, repo)

	unstaged := st.Unstaged()
	if len(unstaged) != 1 {
		t.Fatalf("Unstaged() = %v", paths(unstaged))
	}

	diff, err := repo.FileDiff(ctx, unstaged[0], false)
	if err != nil {
		t.Fatalf("FileDiff: %v", err)
	}
	if !strings.Contains(diff, "+degisti") || !strings.Contains(diff, "-ilk") {
		t.Errorf("diff is missing the expected lines:\n%s", diff)
	}

	// The same file has nothing staged yet.
	staged, err := repo.FileDiff(ctx, unstaged[0], true)
	if err != nil {
		t.Fatalf("FileDiff(staged): %v", err)
	}
	if staged != "" {
		t.Errorf("staged diff should be empty:\n%s", staged)
	}
}

// An untracked file is diffed with --no-index, whose exit code 1 is not an error.
func TestFileDiffUntracked(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "yeni.txt", "tamami yeni\n")

	untracked := mustStatus(t, repo).Untracked()
	if len(untracked) != 1 {
		t.Fatalf("Untracked() = %v", paths(untracked))
	}

	diff, err := repo.FileDiff(ctx, untracked[0], false)
	if err != nil {
		t.Fatalf("FileDiff: %v", err)
	}
	if !strings.Contains(diff, "+tamami yeni") {
		t.Errorf("untracked diff is missing the added line:\n%s", diff)
	}
}

func TestOpenOutsideRepositoryFails(t *testing.T) {
	if _, err := Open(context.Background(), tempDir(t)); err == nil {
		t.Error("Open outside a repository should fail")
	}
}
