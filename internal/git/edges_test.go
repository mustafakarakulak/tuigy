package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// unsetEnv removes variables for the duration of a test and puts them back.
func unsetEnv(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		if old, ok := os.LookupEnv(name); ok {
			t.Cleanup(func() { os.Setenv(name, old) })
		}
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("Unsetenv(%s): %v", name, err)
		}
	}
}

// Checking out a remote branch when a local one of that name already exists
// switches to it rather than failing, which is the whole point of the fallback.
func TestCheckoutRemoteFallsBackToTheExistingLocalBranch(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	remote := tempDir(t)
	gitRun(t, remote, "init", "--bare", "-b", "main")
	gitRun(t, dir, "remote", "add", "origin", remote)
	gitRun(t, dir, "checkout", "-qb", "develop")
	gitRun(t, dir, "push", "-qu", "origin", "develop")
	gitRun(t, dir, "checkout", "-q", "main")

	// develop already exists locally, so --track cannot create it.
	if err := repo.CheckoutRemote(ctx, "origin/develop"); err != nil {
		t.Fatalf("CheckoutRemote: %v", err)
	}
	if got := mustStatus(t, repo).Branch; got != "develop" {
		t.Errorf("branch = %q, want develop", got)
	}
}

func TestCheckoutRemoteReportsTheOriginalFailure(t *testing.T) {
	repo, _ := newTestRepo(t)

	err := repo.CheckoutRemote(context.Background(), "origin/nope")
	if err == nil {
		t.Fatal("checking out a branch that does not exist should fail")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error = %q, want it to name the branch", err)
	}
}

func TestCheckoutAcceptsAFullRemoteRef(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	gitRun(t, dir, "branch", "develop")
	if err := repo.Checkout(ctx, "refs/remotes/develop"); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if got := mustStatus(t, repo).Branch; got != "develop" {
		t.Errorf("branch = %q, want develop", got)
	}
}

// Revert and rebase get the same treatment as a merge, so the header and the
// continue/abort pair work for them too.
func TestStateReportsRevertAndRebase(t *testing.T) {
	ctx := context.Background()

	t.Run("revert", func(t *testing.T) {
		repo, dir := newTestRepo(t)

		writeFile(t, dir, "shared.txt", "one\n")
		gitRun(t, dir, "add", ".")
		gitRun(t, dir, "commit", "-m", "first")
		writeFile(t, dir, "shared.txt", "two\n")
		gitRun(t, dir, "commit", "-am", "second")

		// Reverting the first change conflicts with the second.
		if _, err := gitTry(dir, "revert", "--no-edit", "HEAD~1"); err == nil {
			t.Fatal("the revert should have conflicted")
		}
		if got := repo.State(); got != OpRevert {
			t.Fatalf("State() = %q, want %q", got, OpRevert)
		}
		if err := repo.AbortOperation(ctx); err != nil {
			t.Fatalf("AbortOperation: %v", err)
		}
		if got := repo.State(); got != OpNone {
			t.Errorf("State() = %q after abort, want none", got)
		}
	})

	t.Run("rebase", func(t *testing.T) {
		repo, dir := newTestRepo(t)

		writeFile(t, dir, "shared.txt", "base\n")
		gitRun(t, dir, "add", ".")
		gitRun(t, dir, "commit", "-m", "base")

		gitRun(t, dir, "checkout", "-qb", "feature")
		writeFile(t, dir, "shared.txt", "feature\n")
		gitRun(t, dir, "commit", "-am", "feature side")

		gitRun(t, dir, "checkout", "-q", "main")
		writeFile(t, dir, "shared.txt", "main\n")
		gitRun(t, dir, "commit", "-am", "main side")

		gitRun(t, dir, "checkout", "-q", "feature")
		if _, err := gitTry(dir, "rebase", "main"); err == nil {
			t.Fatal("the rebase should have conflicted")
		}
		if got := repo.State(); got != OpRebase {
			t.Fatalf("State() = %q, want %q", got, OpRebase)
		}
		if err := repo.AbortOperation(ctx); err != nil {
			t.Fatalf("AbortOperation: %v", err)
		}
	})
}

func TestOperationCommandCoversEveryState(t *testing.T) {
	for state, want := range map[OpState]string{
		OpMerge:      "merge",
		OpCherryPick: "cherry-pick",
		OpRevert:     "revert",
		OpRebase:     "rebase",
	} {
		got, err := operationCommand(state)
		if err != nil {
			t.Errorf("operationCommand(%q): %v", state, err)
		}
		if got != want {
			t.Errorf("operationCommand(%q) = %q, want %q", state, got, want)
		}
	}

	if _, err := operationCommand(OpNone); err == nil {
		t.Error("operationCommand with nothing in progress should fail")
	}
}

// A status line git never produces is still better reported than guessed at.
func TestParseStatusRejectsAMalformedEntry(t *testing.T) {
	if _, err := parseStatus("1 M. too few fields\x00"); err == nil {
		t.Error("a malformed ordinary entry should be an error")
	}
	if _, err := parseStatus("u UU short\x00"); err == nil {
		t.Error("a malformed unmerged entry should be an error")
	}
}

func TestStatusCodeString(t *testing.T) {
	if got := StatusUnmodified.String(); got != " " {
		t.Errorf("unmodified renders as %q, want a space", got)
	}
	if got := StatusModified.String(); got != "M" {
		t.Errorf("modified renders as %q, want M", got)
	}
}

// Ignored files are listed by git only when asked for, and are never shown.
func TestParseStatusSkipsIgnoredEntries(t *testing.T) {
	status, err := parseStatus("! build/output\x00? real.txt\x00")
	if err != nil {
		t.Fatalf("parseStatus: %v", err)
	}
	if len(status.Files) != 1 || status.Files[0].Path != "real.txt" {
		t.Errorf("Files = %+v, want only the untracked file", status.Files)
	}
}

func TestStagingNothingDoesNothing(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestRepo(t)

	if err := repo.Stage(ctx); err != nil {
		t.Errorf("Stage with no paths: %v", err)
	}
	if err := repo.Unstage(ctx); err != nil {
		t.Errorf("Unstage with no paths: %v", err)
	}
}

func TestAmendRejectsAnEmptyMessage(t *testing.T) {
	repo, _ := newTestRepo(t)
	if err := repo.Amend(context.Background(), "  \n"); err == nil {
		t.Error("amending with an empty message should fail")
	}
}

// With several remotes and none called origin there is no sensible default, so
// the ambiguity is reported rather than guessed at.
func TestDefaultRemoteWithSeveralAndNoOrigin(t *testing.T) {
	repo, dir := newTestRepo(t)

	gitRun(t, dir, "remote", "add", "upstream", tempDir(t))
	gitRun(t, dir, "remote", "add", "fork", tempDir(t))

	_, err := repo.defaultRemote(context.Background())
	if err == nil {
		t.Fatal("an ambiguous set of remotes should be reported")
	}
	if !strings.Contains(err.Error(), "origin") {
		t.Errorf("error = %q, want it to explain what is missing", err)
	}
}

func TestParseStashSubjectWithoutAMessage(t *testing.T) {
	branch, message := parseStashSubject("On main")
	if branch != "" || message != "On main" {
		t.Errorf("parseStashSubject = (%q, %q), want the subject kept verbatim", branch, message)
	}
}

func TestCommitDetailRequiresAHash(t *testing.T) {
	repo, _ := newTestRepo(t)
	if _, err := repo.CommitDetail(context.Background(), ""); err == nil {
		t.Error("CommitDetail with no hash should fail")
	}
}

// A ref that does not resolve has no history, which is not an error: the view
// simply has nothing to show.
func TestLogOfAMissingRef(t *testing.T) {
	repo, _ := newTestRepo(t)

	commits, err := repo.Log(context.Background(), "no-such-branch", "", 0, 10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("Log() = %v, want empty", subjects(commits))
	}
}

func TestLogWithNoLimit(t *testing.T) {
	repo, _ := newTestRepo(t)

	commits, err := repo.Log(context.Background(), "", "", 0, 0)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("Log() = %v, want empty when no commits were asked for", subjects(commits))
	}
}

// A git failure keeps its own message, which is what the user needs to see.
func TestCmdErrorCarriesStderr(t *testing.T) {
	repo, _ := newTestRepo(t)

	_, err := repo.Log(context.Background(), "", "", -1, 10)
	if err == nil {
		t.Skip("git accepted a negative skip, so there is no error to inspect")
	}

	var cmdErr *CmdError
	if !errorAs(err, &cmdErr) {
		t.Fatalf("error is %T, want a *CmdError", err)
	}
	if cmdErr.ExitCode() <= 0 {
		t.Errorf("ExitCode() = %d, want git's own status", cmdErr.ExitCode())
	}
	if cmdErr.Unwrap() == nil {
		t.Error("Unwrap() should expose the underlying failure")
	}
	if ExitCode(errorStub{}) != -1 {
		t.Error("ExitCode of a non-git error should be -1")
	}
}

type errorStub struct{}

func (errorStub) Error() string { return "not a git error" }

// The editor comes from git, so core.editor works even when no environment
// variable is set — which is how a lot of people configure it.
func TestEditorCommandReadsGitConfig(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	// Actually unset them: git treats an empty GIT_EDITOR as a choice, so
	// t.Setenv("") would not model a machine that has none of these.
	unsetEnv(t, "GIT_EDITOR", "VISUAL", "EDITOR")
	gitRun(t, dir, "config", "core.editor", "my-editor --wait")

	got, chosen := repo.EditorCommand(ctx)
	if got != "my-editor --wait" {
		t.Errorf("EditorCommand() = %q, want core.editor's value", got)
	}
	if !chosen {
		t.Error("chosen = false, want true when core.editor is set")
	}
}

// With nothing configured, git still answers "vi" — but as its own fallback,
// not as a choice, which is what lets tuigy offer something friendlier.
func TestEditorCommandReportsAnUnchosenFallback(t *testing.T) {
	repo, _ := newTestRepo(t)
	unsetEnv(t, "GIT_EDITOR", "VISUAL", "EDITOR")

	got, chosen := repo.EditorCommand(context.Background())
	if chosen {
		t.Errorf("chosen = true with nothing configured; git answered %q", got)
	}
}

// Where git cannot name an editor at all — a dumb terminal with nothing
// configured, which is what a CI container looks like — that must read as an
// answer rather than a failure. It is the case tuigy's own fallback exists for.
func TestEditorCommandWhereGitCannotNameOne(t *testing.T) {
	repo, _ := newTestRepo(t)
	unsetEnv(t, "GIT_EDITOR", "VISUAL", "EDITOR")
	onlyGitOnPath(t)
	t.Setenv("TERM", "dumb")

	command, chosen := repo.EditorCommand(context.Background())
	if chosen {
		t.Error("chosen = true with nothing configured")
	}
	if command != "" {
		t.Errorf("EditorCommand() = %q, want nothing when git refuses to name one", command)
	}
}

// onlyGitOnPath hides every other executable, so git is reachable but has no
// editor to fall back to — the state of a minimal container.
func onlyGitOnPath(t *testing.T) {
	t.Helper()

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git is not on PATH: %v", err)
	}

	dir := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(dir, "git")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	t.Setenv("PATH", dir)
}

// An environment variable counts as a choice just as much as the config does.
func TestEditorCommandCountsTheEnvironment(t *testing.T) {
	repo, _ := newTestRepo(t)
	unsetEnv(t, "GIT_EDITOR", "VISUAL", "EDITOR")
	t.Setenv("EDITOR", "my-editor")

	got, chosen := repo.EditorCommand(context.Background())
	if !chosen {
		t.Error("chosen = false, want true when EDITOR is set")
	}
	if got != "my-editor" {
		t.Errorf("EditorCommand() = %q, want EDITOR's value", got)
	}
}

// Writes still must not stop for an editor, whatever the user configured.
func TestWritesNeverOpenAnEditor(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	// An editor that would hang forever if git ever actually ran it.
	gitRun(t, dir, "config", "core.editor", "sleep 600")

	writeFile(t, dir, "shared.txt", "base\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "base")

	gitRun(t, dir, "checkout", "-qb", "feature")
	writeFile(t, dir, "shared.txt", "feature\n")
	gitRun(t, dir, "commit", "-am", "feature side")

	gitRun(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "shared.txt", "main\n")
	gitRun(t, dir, "commit", "-am", "main side")

	if err := repo.Merge(ctx, "feature"); err == nil {
		t.Fatal("the merge should have conflicted")
	}
	writeFile(t, dir, "shared.txt", "resolved\n")
	if err := repo.Stage(ctx, "shared.txt"); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	// Without the override this would sit in "sleep 600" until the timeout.
	done := make(chan error, 1)
	go func() { done <- repo.ContinueOperation(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ContinueOperation: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("ContinueOperation stopped for an editor")
	}
}
