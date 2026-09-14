package git

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func branchNames(branches []Branch) []string {
	out := make([]string, len(branches))
	for i, b := range branches {
		out[i] = b.Name
	}
	return out
}

func findBranch(t *testing.T, branches []Branch, name string) Branch {
	t.Helper()
	for _, b := range branches {
		if b.Name == name {
			return b
		}
	}
	t.Fatalf("no branch named %q in %v", name, branchNames(branches))
	return Branch{}
}

func TestParseTrack(t *testing.T) {
	cases := []struct {
		in     string
		ahead  int
		behind int
		gone   bool
	}{
		{"", 0, 0, false},
		{"[gone]", 0, 0, true},
		{"[ahead 2]", 2, 0, false},
		{"[behind 3]", 0, 3, false},
		{"[ahead 2, behind 3]", 2, 3, false},
	}

	for _, c := range cases {
		ahead, behind, gone := parseTrack(c.in)
		if ahead != c.ahead || behind != c.behind || gone != c.gone {
			t.Errorf("parseTrack(%q) = (%d, %d, %v), want (%d, %d, %v)",
				c.in, ahead, behind, gone, c.ahead, c.behind, c.gone)
		}
	}
}

func TestBranchesListsLocalAndCurrent(t *testing.T) {
	repo, dir := newTestRepo(t)

	gitRun(t, dir, "branch", "develop")
	gitRun(t, dir, "branch", "feature/payment")

	branches, err := repo.Branches(context.Background())
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}

	got := branchNames(branches)
	slices.Sort(got)
	if want := []string{"develop", "feature/payment", "main"}; !slices.Equal(got, want) {
		t.Fatalf("Branches() = %v, want %v", got, want)
	}

	if main := findBranch(t, branches, "main"); !main.Current {
		t.Error("main should be marked current")
	}
	if dev := findBranch(t, branches, "develop"); dev.Current {
		t.Error("develop should not be marked current")
	}

	main := findBranch(t, branches, "main")
	if main.Subject != "initial" {
		t.Errorf("Subject = %q, want %q", main.Subject, "initial")
	}
	if main.Hash == "" {
		t.Error("Hash is empty")
	}
	if main.Committed.IsZero() {
		t.Error("Committed is zero")
	}
}

func TestBranchesReportsAheadBehindAndSkipsRemoteHEAD(t *testing.T) {
	repo, dir := newTestRepo(t)

	remote := tempDir(t)
	gitRun(t, remote, "init", "--bare", "-b", "main")
	gitRun(t, dir, "remote", "add", "origin", remote)
	gitRun(t, dir, "push", "-u", "origin", "main")
	// Give origin a HEAD symref, which must not show up as a branch.
	gitRun(t, dir, "remote", "set-head", "origin", "main")

	other := tempDir(t)
	gitRun(t, other, "clone", remote, ".")
	gitRun(t, other, "config", "user.email", "o@example.com")
	gitRun(t, other, "config", "user.name", "o")
	writeFile(t, other, "remote.txt", "remote\n")
	gitRun(t, other, "add", ".")
	gitRun(t, other, "commit", "-m", "remote commit")
	gitRun(t, other, "push")

	writeFile(t, dir, "local.txt", "local\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "local commit")
	gitRun(t, dir, "fetch")

	branches, err := repo.Branches(context.Background())
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}

	main := findBranch(t, branches, "main")
	if main.Upstream != "origin/main" {
		t.Errorf("Upstream = %q, want origin/main", main.Upstream)
	}
	if main.Ahead != 1 || main.Behind != 1 {
		t.Errorf("ahead/behind = %d/%d, want 1/1", main.Ahead, main.Behind)
	}

	if remoteBranch := findBranch(t, branches, "origin/main"); !remoteBranch.Remote {
		t.Error("origin/main should be marked remote")
	}
	for _, b := range branches {
		if strings.HasSuffix(b.Ref, "/HEAD") {
			t.Errorf("refs/remotes/*/HEAD should be skipped, got %q", b.Ref)
		}
	}
}

func TestBranchesReportsGoneUpstream(t *testing.T) {
	repo, dir := newTestRepo(t)

	remote := tempDir(t)
	gitRun(t, remote, "init", "--bare", "-b", "main")
	gitRun(t, dir, "remote", "add", "origin", remote)
	gitRun(t, dir, "checkout", "-qb", "temporary")
	gitRun(t, dir, "push", "-u", "origin", "temporary")

	// Delete the branch on the remote, then prune.
	gitRun(t, dir, "push", "origin", "--delete", "temporary")
	gitRun(t, dir, "fetch", "--prune")

	branch := findBranch(t, mustBranches(t, repo), "temporary")
	if !branch.Gone {
		t.Error("Gone = false, want true for a deleted upstream")
	}
}

func TestCheckoutAndCreateBranch(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	gitRun(t, dir, "branch", "develop")

	if err := repo.Checkout(ctx, "develop"); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if got := mustStatus(t, repo).Branch; got != "develop" {
		t.Fatalf("branch = %q, want develop", got)
	}

	// A new branch must be created from its stated source, not from HEAD.
	writeFile(t, dir, "on-develop.txt", "x\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "develop only")

	if err := repo.CreateBranch(ctx, "feature/payment", "main"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if got := mustStatus(t, repo).Branch; got != "feature/payment" {
		t.Errorf("branch = %q, want feature/payment", got)
	}
	if out := gitRun(t, dir, "log", "--oneline"); strings.Contains(out, "develop only") {
		t.Errorf("feature/payment was branched from develop, not main:\n%s", out)
	}
}

func TestCreateBranchRejectsEmptyName(t *testing.T) {
	repo, _ := newTestRepo(t)
	if err := repo.CreateBranch(context.Background(), "  ", "main"); err == nil {
		t.Error("an empty branch name should be rejected")
	}
}

func TestCheckoutRemoteCreatesTrackingBranch(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	remote := tempDir(t)
	gitRun(t, remote, "init", "--bare", "-b", "main")
	gitRun(t, dir, "remote", "add", "origin", remote)
	gitRun(t, dir, "push", "-u", "origin", "main")

	other := tempDir(t)
	gitRun(t, other, "clone", remote, ".")
	gitRun(t, other, "config", "user.email", "o@example.com")
	gitRun(t, other, "config", "user.name", "o")
	gitRun(t, other, "checkout", "-qb", "feature/remote-only")
	writeFile(t, other, "f.txt", "x\n")
	gitRun(t, other, "add", ".")
	gitRun(t, other, "commit", "-m", "remote work")
	gitRun(t, other, "push", "-u", "origin", "feature/remote-only")

	gitRun(t, dir, "fetch")
	if err := repo.CheckoutRemote(ctx, "origin/feature/remote-only"); err != nil {
		t.Fatalf("CheckoutRemote: %v", err)
	}

	st := mustStatus(t, repo)
	if st.Branch != "feature/remote-only" {
		t.Errorf("branch = %q, want feature/remote-only", st.Branch)
	}
	if st.Upstream != "origin/feature/remote-only" {
		t.Errorf("upstream = %q, want origin/feature/remote-only", st.Upstream)
	}
}

func TestDeleteBranchRefusesUnmergedWithoutForce(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	gitRun(t, dir, "checkout", "-qb", "throwaway")
	writeFile(t, dir, "throwaway.txt", "x\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "unmerged work")
	gitRun(t, dir, "checkout", "-q", "main")

	if err := repo.DeleteBranch(ctx, "throwaway", false); err == nil {
		t.Fatal("deleting an unmerged branch without force should fail")
	}
	if err := repo.DeleteBranch(ctx, "throwaway", true); err != nil {
		t.Fatalf("forced DeleteBranch: %v", err)
	}

	for _, b := range mustBranches(t, repo) {
		if b.Name == "throwaway" {
			t.Error("throwaway should be gone")
		}
	}
}

func mustBranches(t *testing.T, repo *Repo) []Branch {
	t.Helper()
	branches, err := repo.Branches(context.Background())
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	return branches
}
