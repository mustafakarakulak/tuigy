package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newMergeRepo builds main plus a feature branch that adds one file, with main
// left one commit ahead so the merge is a real one rather than a fast-forward.
func newMergeRepo(t *testing.T) (*Repo, string) {
	t.Helper()

	repo, dir := newTestRepo(t)

	gitRun(t, dir, "checkout", "-qb", "feature")
	writeFile(t, dir, "feature.txt", "feature\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "feature work")

	gitRun(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "main.txt", "main\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "main work")

	return repo, dir
}

func TestCanFastForward(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	gitRun(t, dir, "checkout", "-qb", "feature")
	writeFile(t, dir, "feature.txt", "feature\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "feature work")

	// main is an ancestor of feature, so main can be fast-forwarded.
	ff, err := repo.CanFastForward(ctx, "feature", "main")
	if err != nil {
		t.Fatalf("CanFastForward: %v", err)
	}
	if !ff {
		t.Error("main should be fast-forwardable to feature")
	}

	// Not the other way around.
	ff, err = repo.CanFastForward(ctx, "main", "feature")
	if err != nil {
		t.Fatalf("CanFastForward: %v", err)
	}
	if ff {
		t.Error("feature is ahead of main and must not be fast-forwardable to it")
	}
}

// A fast-forward into a branch you are not on must not disturb the working tree
// or move you off the branch you are working on.
func TestMergeIntoFastForwardsWithoutCheckout(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	gitRun(t, dir, "branch", "target")
	writeFile(t, dir, "source.txt", "source\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "source work")

	// Leave an uncommitted change behind: a checkout would be visible here.
	writeFile(t, dir, "dirty.txt", "uncommitted\n")

	if err := repo.MergeInto(ctx, "main", "target", "main"); err != nil {
		t.Fatalf("MergeInto: %v", err)
	}

	st := mustStatus(t, repo)
	if st.Branch != "main" {
		t.Errorf("branch = %q, a fast-forward must not check anything out", st.Branch)
	}
	if len(st.Untracked()) != 1 {
		t.Errorf("untracked = %v, the working tree should be untouched", paths(st.Untracked()))
	}

	target := strings.TrimSpace(gitRun(t, dir, "rev-parse", "target"))
	head := strings.TrimSpace(gitRun(t, dir, "rev-parse", "main"))
	if target != head {
		t.Errorf("target = %s, want it advanced to %s", target, head)
	}
}

func TestMergeIntoCurrentBranch(t *testing.T) {
	ctx := context.Background()
	repo, dir := newMergeRepo(t)

	if err := repo.MergeInto(ctx, "feature", "main", "main"); err != nil {
		t.Fatalf("MergeInto: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "feature.txt")); err != nil {
		t.Error("the merge did not bring in the feature branch's file")
	}
	if out := gitRun(t, dir, "log", "--oneline", "--merges"); strings.TrimSpace(out) == "" {
		t.Error("a merge commit should have been created")
	}
	if got := mustStatus(t, repo).Branch; got != "main" {
		t.Errorf("branch = %q, want main", got)
	}
}

// Merging into another branch that cannot fast-forward has to check it out.
// The point of the test is that it works and leaves you on the target.
func TestMergeIntoOtherBranchChecksItOut(t *testing.T) {
	ctx := context.Background()
	repo, dir := newMergeRepo(t)

	// Work from a third branch so neither source nor target is current.
	gitRun(t, dir, "checkout", "-qb", "somewhere-else")

	if err := repo.MergeInto(ctx, "feature", "main", "somewhere-else"); err != nil {
		t.Fatalf("MergeInto: %v", err)
	}

	if got := mustStatus(t, repo).Branch; got != "main" {
		t.Errorf("branch = %q, want main after the merge", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "feature.txt")); err != nil {
		t.Error("the merge did not bring in the feature branch's file")
	}
}

func TestMergeIntoRejectsSelfMerge(t *testing.T) {
	repo, _ := newMergeRepo(t)
	if err := repo.MergeInto(context.Background(), "main", "main", "main"); err == nil {
		t.Error("merging a branch into itself should be rejected")
	}
}

func TestMergeConflictThenAbort(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "shared.txt", "base\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "base")

	gitRun(t, dir, "checkout", "-qb", "feature")
	writeFile(t, dir, "shared.txt", "feature side\n")
	gitRun(t, dir, "commit", "-am", "feature side")

	gitRun(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "shared.txt", "main side\n")
	gitRun(t, dir, "commit", "-am", "main side")

	if err := repo.Merge(ctx, "feature"); err == nil {
		t.Fatal("the merge should have stopped on a conflict")
	}

	if got := repo.State(); got != OpMerge {
		t.Fatalf("State() = %q, want %q", got, OpMerge)
	}
	if got := paths(mustStatus(t, repo).Conflicted()); len(got) != 1 || got[0] != "shared.txt" {
		t.Fatalf("Conflicted() = %v, want [shared.txt]", got)
	}

	if err := repo.AbortOperation(ctx); err != nil {
		t.Fatalf("AbortOperation: %v", err)
	}
	if got := repo.State(); got != OpNone {
		t.Errorf("State() = %q after abort, want none", got)
	}
	if !mustStatus(t, repo).IsClean() {
		t.Error("abort should have restored a clean working tree")
	}
}

func TestMergeConflictThenContinue(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "shared.txt", "base\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "base")

	gitRun(t, dir, "checkout", "-qb", "feature")
	writeFile(t, dir, "shared.txt", "feature side\n")
	gitRun(t, dir, "commit", "-am", "feature side")

	gitRun(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "shared.txt", "main side\n")
	gitRun(t, dir, "commit", "-am", "main side")

	if err := repo.Merge(ctx, "feature"); err == nil {
		t.Fatal("the merge should have stopped on a conflict")
	}

	// Resolve the way a user would: edit the file, then stage it.
	writeFile(t, dir, "shared.txt", "resolved by hand\n")
	if err := repo.Stage(ctx, "shared.txt"); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	// --continue must not hang waiting on an editor.
	if err := repo.ContinueOperation(ctx); err != nil {
		t.Fatalf("ContinueOperation: %v", err)
	}

	if got := repo.State(); got != OpNone {
		t.Errorf("State() = %q after continue, want none", got)
	}
	if !mustStatus(t, repo).IsClean() {
		t.Error("the working tree should be clean once the merge is committed")
	}
	if out := gitRun(t, dir, "log", "--oneline", "--merges"); strings.TrimSpace(out) == "" {
		t.Error("continuing should have created the merge commit")
	}
}

func TestContinueWithoutOperationFails(t *testing.T) {
	repo, _ := newTestRepo(t)
	if err := repo.ContinueOperation(context.Background()); err == nil {
		t.Error("continuing with nothing in progress should fail")
	}
	if err := repo.AbortOperation(context.Background()); err == nil {
		t.Error("aborting with nothing in progress should fail")
	}
}
