package git

import (
	"context"
	"strings"
	"testing"
)

// newRepoWithRemote returns a repository whose main branch tracks a bare remote,
// plus the paths of the remote and of a second clone used to move it forward.
func newRepoWithRemote(t *testing.T) (repo *Repo, dir, remote, other string) {
	t.Helper()

	repo, dir = newTestRepo(t)
	remote = tempDir(t)
	gitRun(t, remote, "init", "--bare", "-b", "main")
	gitRun(t, dir, "remote", "add", "origin", remote)
	gitRun(t, dir, "push", "-u", "origin", "main")

	other = tempDir(t)
	gitRun(t, other, "clone", remote, ".")
	gitRun(t, other, "config", "user.email", "o@example.com")
	gitRun(t, other, "config", "user.name", "o")

	return repo, dir, remote, other
}

func TestPushSetsUpstreamForANewBranch(t *testing.T) {
	ctx := context.Background()
	repo, dir, _, _ := newRepoWithRemote(t)

	gitRun(t, dir, "checkout", "-qb", "feature/new")
	writeFile(t, dir, "feature.txt", "x\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "feature work")

	st := mustStatus(t, repo)
	if st.Upstream != "" {
		t.Fatalf("setup: upstream = %q, want none", st.Upstream)
	}

	if err := repo.Push(ctx, st.Branch, st.Upstream); err != nil {
		t.Fatalf("Push: %v", err)
	}

	st = mustStatus(t, repo)
	if st.Upstream != "origin/feature/new" {
		t.Errorf("upstream = %q, want origin/feature/new", st.Upstream)
	}
	if st.Ahead != 0 {
		t.Errorf("ahead = %d after push, want 0", st.Ahead)
	}
}

func TestPushWithExistingUpstream(t *testing.T) {
	ctx := context.Background()
	repo, dir, _, _ := newRepoWithRemote(t)

	writeFile(t, dir, "more.txt", "x\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "more work")

	st := mustStatus(t, repo)
	if err := repo.Push(ctx, st.Branch, st.Upstream); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if got := mustStatus(t, repo).Ahead; got != 0 {
		t.Errorf("ahead = %d after push, want 0", got)
	}
}

func TestPullFastForwards(t *testing.T) {
	ctx := context.Background()
	repo, dir, _, other := newRepoWithRemote(t)

	writeFile(t, other, "remote.txt", "from remote\n")
	gitRun(t, other, "add", ".")
	gitRun(t, other, "commit", "-m", "remote work")
	gitRun(t, other, "push")

	if err := repo.Pull(ctx); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	if out := gitRun(t, dir, "log", "--oneline"); !strings.Contains(out, "remote work") {
		t.Errorf("pull did not bring in the remote commit:\n%s", out)
	}
	if got := mustStatus(t, repo).Behind; got != 0 {
		t.Errorf("behind = %d after pull, want 0", got)
	}
}

// A divergent branch must be reported, never silently reconciled with a merge
// commit the user did not ask for.
func TestPullRefusesToMergeDivergentHistory(t *testing.T) {
	ctx := context.Background()
	repo, dir, _, other := newRepoWithRemote(t)

	writeFile(t, other, "remote.txt", "from remote\n")
	gitRun(t, other, "add", ".")
	gitRun(t, other, "commit", "-m", "remote work")
	gitRun(t, other, "push")

	writeFile(t, dir, "local.txt", "from local\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "local work")

	if err := repo.Pull(ctx); err == nil {
		t.Fatal("Pull should refuse a divergent branch")
	}

	out := gitRun(t, dir, "log", "--oneline", "--merges")
	if strings.TrimSpace(out) != "" {
		t.Errorf("pull created a merge commit:\n%s", out)
	}
}

func TestFetchPrunesDeletedRemoteBranches(t *testing.T) {
	ctx := context.Background()
	repo, dir, _, other := newRepoWithRemote(t)

	gitRun(t, other, "checkout", "-qb", "short-lived")
	writeFile(t, other, "f.txt", "x\n")
	gitRun(t, other, "add", ".")
	gitRun(t, other, "commit", "-m", "work")
	gitRun(t, other, "push", "-u", "origin", "short-lived")

	if err := repo.Fetch(ctx, false); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !hasBranch(mustBranches(t, repo), "origin/short-lived") {
		t.Fatal("fetch did not pick up origin/short-lived")
	}

	gitRun(t, other, "push", "origin", "--delete", "short-lived")
	if err := repo.Fetch(ctx, true); err != nil {
		t.Fatalf("Fetch(all): %v", err)
	}
	if hasBranch(mustBranches(t, repo), "origin/short-lived") {
		t.Error("fetch did not prune the deleted remote branch")
	}

	_ = dir
}

func TestDefaultRemote(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	if _, err := repo.defaultRemote(ctx); err == nil {
		t.Error("defaultRemote should fail when no remote is configured")
	}

	gitRun(t, dir, "remote", "add", "upstream", tempDir(t))
	got, err := repo.defaultRemote(ctx)
	if err != nil {
		t.Fatalf("defaultRemote: %v", err)
	}
	if got != "upstream" {
		t.Errorf("defaultRemote() = %q, want the only remote %q", got, "upstream")
	}

	// origin wins once it exists, whatever else is configured.
	gitRun(t, dir, "remote", "add", "origin", tempDir(t))
	got, err = repo.defaultRemote(ctx)
	if err != nil {
		t.Fatalf("defaultRemote: %v", err)
	}
	if got != "origin" {
		t.Errorf("defaultRemote() = %q, want origin", got)
	}
}

func hasBranch(branches []Branch, name string) bool {
	for _, b := range branches {
		if b.Name == name {
			return true
		}
	}
	return false
}
