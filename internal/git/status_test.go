package git

import (
	"context"
	"slices"
	"testing"
)

func TestStatusBuckets(t *testing.T) {
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "staged.txt", "yeni\n")
	gitRun(t, dir, "add", "staged.txt")

	writeFile(t, dir, "README.md", "degisti\n") // takipli, unstaged
	writeFile(t, dir, "untracked.txt", "hic eklenmedi\n")

	st := mustStatus(t, repo)

	if got, want := paths(st.Staged()), []string{"staged.txt"}; !slices.Equal(got, want) {
		t.Errorf("Staged() = %v, istenen %v", got, want)
	}
	if got, want := paths(st.Unstaged()), []string{"README.md"}; !slices.Equal(got, want) {
		t.Errorf("Unstaged() = %v, istenen %v", got, want)
	}
	if got, want := paths(st.Untracked()), []string{"untracked.txt"}; !slices.Equal(got, want) {
		t.Errorf("Untracked() = %v, istenen %v", got, want)
	}
	if st.Branch != "main" {
		t.Errorf("Branch = %q, istenen %q", st.Branch, "main")
	}
	if st.IsClean() {
		t.Error("IsClean() = true, want false while changes exist")
	}
}

// A file changed in both the index and the working tree belongs in both lists.
func TestStatusPartiallyStagedFileAppearsInBothBuckets(t *testing.T) {
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "README.md", "birinci degisiklik\n")
	gitRun(t, dir, "add", "README.md")
	writeFile(t, dir, "README.md", "ikinci degisiklik\n")

	st := mustStatus(t, repo)

	if got := paths(st.Staged()); !slices.Equal(got, []string{"README.md"}) {
		t.Errorf("Staged() = %v, README.md bekleniyordu", got)
	}
	if got := paths(st.Unstaged()); !slices.Equal(got, []string{"README.md"}) {
		t.Errorf("Unstaged() = %v, README.md bekleniyordu", got)
	}
}

func TestStatusRenameKeepsOriginalPath(t *testing.T) {
	repo, dir := newTestRepo(t)

	gitRun(t, dir, "mv", "README.md", "DOCS.md")

	st := mustStatus(t, repo)

	staged := st.Staged()
	if len(staged) != 1 {
		t.Fatalf("Staged() = %v, tek girdi bekleniyordu", paths(staged))
	}
	if staged[0].Path != "DOCS.md" {
		t.Errorf("Path = %q, istenen %q", staged[0].Path, "DOCS.md")
	}
	if staged[0].OrigPath != "README.md" {
		t.Errorf("OrigPath = %q, istenen %q", staged[0].OrigPath, "README.md")
	}
	if staged[0].Index != StatusRenamed {
		t.Errorf("Index = %q, istenen %q", staged[0].Index, StatusRenamed)
	}
}

// A rename entry consumes an extra field under -z; make sure that does not
// shift the parsing of the entry that follows it.
func TestStatusRenameDoesNotShiftFollowingEntries(t *testing.T) {
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "a.txt", "a\n")
	gitRun(t, dir, "add", "a.txt")
	gitRun(t, dir, "commit", "-m", "a ekle")

	gitRun(t, dir, "mv", "a.txt", "z.txt")
	writeFile(t, dir, "README.md", "degisti\n")
	gitRun(t, dir, "add", "README.md")

	st := mustStatus(t, repo)

	got := paths(st.Staged())
	slices.Sort(got)
	if want := []string{"README.md", "z.txt"}; !slices.Equal(got, want) {
		t.Errorf("Staged() = %v, istenen %v", got, want)
	}
}

func TestStatusUnmergedFilesAreConflicted(t *testing.T) {
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "conflict.txt", "temel\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "temel")

	gitRun(t, dir, "checkout", "-b", "feature")
	writeFile(t, dir, "conflict.txt", "feature tarafi\n")
	gitRun(t, dir, "commit", "-am", "feature")

	gitRun(t, dir, "checkout", "main")
	writeFile(t, dir, "conflict.txt", "main tarafi\n")
	gitRun(t, dir, "commit", "-am", "main")

	if _, err := gitTry(dir, "merge", "feature"); err == nil {
		t.Fatal("merge should have produced a conflict")
	}

	st := mustStatus(t, repo)

	if got, want := paths(st.Conflicted()), []string{"conflict.txt"}; !slices.Equal(got, want) {
		t.Errorf("Conflicted() = %v, istenen %v", got, want)
	}
	// A conflicted file must not leak into the ordinary staged/unstaged lists.
	if got := paths(st.Staged()); len(got) != 0 {
		t.Errorf("Staged() = %v, want empty", got)
	}
	if got := paths(st.Unstaged()); len(got) != 0 {
		t.Errorf("Unstaged() = %v, want empty", got)
	}
	if got := repo.State(); got != OpMerge {
		t.Errorf("State() = %q, istenen %q", got, OpMerge)
	}
}

func TestStatusAheadBehind(t *testing.T) {
	repo, dir := newTestRepo(t)

	remote := tempDir(t)
	gitRun(t, remote, "init", "--bare", "-b", "main")
	gitRun(t, dir, "remote", "add", "origin", remote)
	gitRun(t, dir, "push", "-u", "origin", "main")

	// Advance the remote through a second clone, which puts us behind.
	other := tempDir(t)
	gitRun(t, other, "clone", remote, ".")
	gitRun(t, other, "config", "user.email", "other@example.com")
	gitRun(t, other, "config", "user.name", "other")
	writeFile(t, other, "other.txt", "uzak\n")
	gitRun(t, other, "add", ".")
	gitRun(t, other, "commit", "-m", "uzak commit")
	gitRun(t, other, "push")

	// Yerelde iki commit: bu bizi ahead yapar.
	for _, name := range []string{"yerel1.txt", "yerel2.txt"} {
		writeFile(t, dir, name, "yerel\n")
		gitRun(t, dir, "add", ".")
		gitRun(t, dir, "commit", "-m", name)
	}
	gitRun(t, dir, "fetch")

	st := mustStatus(t, repo)

	if st.Ahead != 2 {
		t.Errorf("Ahead = %d, istenen 2", st.Ahead)
	}
	if st.Behind != 1 {
		t.Errorf("Behind = %d, istenen 1", st.Behind)
	}
	if st.Upstream != "origin/main" {
		t.Errorf("Upstream = %q, istenen %q", st.Upstream, "origin/main")
	}
	if st.Head == "" {
		t.Error("Head is empty, want a short hash")
	}
}

func TestStatusDetachedHead(t *testing.T) {
	repo, dir := newTestRepo(t)

	head := mustStatus(t, repo).Head
	gitRun(t, dir, "checkout", head)

	st := mustStatus(t, repo)
	if !st.Detached {
		t.Error("Detached = false, true bekleniyordu")
	}
	if st.Branch != "" {
		t.Errorf("Branch = %q, want empty while detached", st.Branch)
	}
}

// A repository with no commits yet must be readable without blowing up.
func TestStatusUnbornBranch(t *testing.T) {
	dir := tempDir(t)
	gitRun(t, dir, "init", "-b", "main")
	writeFile(t, dir, "yeni.txt", "icerik\n")

	repo, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	st := mustStatus(t, repo)
	if st.Head != "" {
		t.Errorf("Head = %q, want empty before the first commit", st.Head)
	}
	if got, want := paths(st.Untracked()), []string{"yeni.txt"}; !slices.Equal(got, want) {
		t.Errorf("Untracked() = %v, istenen %v", got, want)
	}
}
