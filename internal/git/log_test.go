package git

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func subjects(commits []Commit) []string {
	out := make([]string, len(commits))
	for i, c := range commits {
		out[i] = c.Subject
	}
	return out
}

func TestLogListsNewestFirst(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	for _, name := range []string{"second", "third"} {
		writeFile(t, dir, name+".txt", name+"\n")
		gitRun(t, dir, "add", ".")
		gitRun(t, dir, "commit", "-m", name)
	}

	commits, err := repo.Log(ctx, "", "", 0, 10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	if got, want := subjects(commits), []string{"third", "second", "initial"}; !slices.Equal(got, want) {
		t.Fatalf("Log() = %v, want %v", got, want)
	}

	head := commits[0]
	if head.Hash == "" || head.Short == "" {
		t.Error("commit hashes are empty")
	}
	if !strings.HasPrefix(head.Hash, head.Short) {
		t.Errorf("Short %q is not a prefix of Hash %q", head.Short, head.Hash)
	}
	if head.Author != "tuigy test" {
		t.Errorf("Author = %q", head.Author)
	}
	if head.Date.IsZero() {
		t.Error("Date is zero")
	}
	if !strings.Contains(head.Refs, "main") {
		t.Errorf("Refs = %q, want it to mention main", head.Refs)
	}
	if head.IsMerge() {
		t.Error("an ordinary commit must not report itself as a merge")
	}
}

func TestLogPaginates(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	for _, name := range []string{"a", "b", "c", "d"} {
		writeFile(t, dir, name+".txt", name+"\n")
		gitRun(t, dir, "add", ".")
		gitRun(t, dir, "commit", "-m", name)
	}

	first, err := repo.Log(ctx, "", "", 0, 2)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	second, err := repo.Log(ctx, "", "", 2, 2)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	if got, want := subjects(first), []string{"d", "c"}; !slices.Equal(got, want) {
		t.Errorf("first page = %v, want %v", got, want)
	}
	if got, want := subjects(second), []string{"b", "a"}; !slices.Equal(got, want) {
		t.Errorf("second page = %v, want %v", got, want)
	}

	// Past the end there is simply nothing more.
	last, err := repo.Log(ctx, "", "", 10, 2)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(last) != 0 {
		t.Errorf("page past the end = %v, want empty", subjects(last))
	}
}

func TestLogOfAnotherBranch(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	gitRun(t, dir, "checkout", "-qb", "feature")
	writeFile(t, dir, "feature.txt", "x\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "feature work")
	gitRun(t, dir, "checkout", "-q", "main")

	commits, err := repo.Log(ctx, "feature", "", 0, 10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if got := subjects(commits); !slices.Contains(got, "feature work") {
		t.Errorf("Log(feature) = %v, want it to include the feature commit", got)
	}

	commits, err = repo.Log(ctx, "main", "", 0, 10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if got := subjects(commits); slices.Contains(got, "feature work") {
		t.Errorf("Log(main) = %v, want the feature commit excluded", got)
	}
}

// A repository with no commits must read as an empty history, not an error.
func TestLogOnEmptyRepository(t *testing.T) {
	dir := tempDir(t)
	gitRun(t, dir, "init", "-b", "main")

	repo, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	commits, err := repo.Log(context.Background(), "", "", 0, 10)
	if err != nil {
		t.Fatalf("Log on an empty repository: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("Log() = %v, want empty", subjects(commits))
	}
}

func TestCommitDetail(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	writeFile(t, dir, "added.txt", "new\n")
	writeFile(t, dir, "README.md", "changed\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "feat: two files\n\nA longer explanation\nover two lines.")

	detail, err := repo.CommitDetail(ctx, "HEAD")
	if err != nil {
		t.Fatalf("CommitDetail: %v", err)
	}

	if detail.Commit.Subject != "feat: two files" {
		t.Errorf("Subject = %q", detail.Commit.Subject)
	}
	if !strings.Contains(detail.Body, "over two lines.") {
		t.Errorf("Body = %q, want the full message", detail.Body)
	}

	byPath := map[string]StatusCode{}
	for _, f := range detail.Files {
		byPath[f.Path] = f.Status
	}
	if byPath["added.txt"] != StatusAdded {
		t.Errorf("added.txt status = %q, want A", byPath["added.txt"])
	}
	if byPath["README.md"] != StatusModified {
		t.Errorf("README.md status = %q, want M", byPath["README.md"])
	}
	if !strings.Contains(detail.Diff, "+new") {
		t.Errorf("diff is missing the added line:\n%s", detail.Diff)
	}
}

func TestCommitDetailOfARename(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	gitRun(t, dir, "mv", "README.md", "DOCS.md")
	gitRun(t, dir, "commit", "-m", "docs: rename")

	detail, err := repo.CommitDetail(ctx, "HEAD")
	if err != nil {
		t.Fatalf("CommitDetail: %v", err)
	}
	if len(detail.Files) != 1 {
		t.Fatalf("Files = %+v, want one entry", detail.Files)
	}

	f := detail.Files[0]
	if f.Status != StatusRenamed {
		t.Errorf("Status = %q, want R", f.Status)
	}
	if f.OrigPath != "README.md" || f.Path != "DOCS.md" {
		t.Errorf("rename = %q → %q, want README.md → DOCS.md", f.OrigPath, f.Path)
	}
}

// A merge commit reports a combined status with one letter per parent, which
// must not be mistaken for a rename's letter-plus-score.
func TestCommitDetailOfAMerge(t *testing.T) {
	ctx := context.Background()
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

	if _, err := gitTry(dir, "merge", "feature"); err == nil {
		t.Fatal("expected a conflict")
	}
	writeFile(t, dir, "shared.txt", "resolved\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "--no-edit")

	detail, err := repo.CommitDetail(ctx, "HEAD")
	if err != nil {
		t.Fatalf("CommitDetail: %v", err)
	}
	if !detail.Commit.IsMerge() {
		t.Error("IsMerge() = false, want true")
	}
	for _, f := range detail.Files {
		if f.Path == "" {
			t.Errorf("a file entry has no path: %+v", f)
		}
		if f.OrigPath != "" {
			t.Errorf("a merge status was mistaken for a rename: %+v", f)
		}
	}
}

func TestCherryPickAppliesInOrder(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	gitRun(t, dir, "checkout", "-qb", "feature")
	for _, name := range []string{"one", "two"} {
		writeFile(t, dir, name+".txt", name+"\n")
		gitRun(t, dir, "add", ".")
		gitRun(t, dir, "commit", "-m", name)
	}

	commits, err := repo.Log(ctx, "feature", "", 0, 2)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	// Log is newest first; cherry-pick wants them chronologically.
	oldestFirst := []string{commits[1].Hash, commits[0].Hash}

	gitRun(t, dir, "checkout", "-q", "main")
	if err := repo.CherryPick(ctx, oldestFirst...); err != nil {
		t.Fatalf("CherryPick: %v", err)
	}

	if got := subjects(mustLog(t, repo, "main", 2)); !slices.Equal(got, []string{"two", "one"}) {
		t.Errorf("main history = %v, want the commits replayed in order", got)
	}
	for _, name := range []string{"one.txt", "two.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s was not brought over", name)
		}
	}
}

func TestCherryPickIntoAnotherBranch(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	gitRun(t, dir, "branch", "target")
	gitRun(t, dir, "checkout", "-qb", "feature")
	writeFile(t, dir, "picked.txt", "x\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "worth picking")

	commits := mustLog(t, repo, "feature", 1)
	if err := repo.CherryPickInto(ctx, "target", "feature", commits[0].Hash); err != nil {
		t.Fatalf("CherryPickInto: %v", err)
	}

	if got := mustStatus(t, repo).Branch; got != "target" {
		t.Errorf("branch = %q, want target", got)
	}
	if got := subjects(mustLog(t, repo, "target", 1)); got[0] != "worth picking" {
		t.Errorf("target history = %v", got)
	}
}

func TestCherryPickConflictLeavesResumableState(t *testing.T) {
	ctx := context.Background()
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

	commits := mustLog(t, repo, "feature", 1)
	if err := repo.CherryPick(ctx, commits[0].Hash); err == nil {
		t.Fatal("the cherry-pick should have conflicted")
	}

	if got := repo.State(); got != OpCherryPick {
		t.Fatalf("State() = %q, want %q", got, OpCherryPick)
	}
	if got := paths(mustStatus(t, repo).Conflicted()); len(got) != 1 {
		t.Fatalf("Conflicted() = %v, want one file", got)
	}

	// The same continue/abort pair that serves a merge must serve this too.
	if err := repo.AbortOperation(ctx); err != nil {
		t.Fatalf("AbortOperation: %v", err)
	}
	if got := repo.State(); got != OpNone {
		t.Errorf("State() = %q after abort, want none", got)
	}
}

func TestCherryPickWithNoCommitsFails(t *testing.T) {
	repo, _ := newTestRepo(t)
	if err := repo.CherryPick(context.Background()); err == nil {
		t.Error("cherry-picking nothing should fail")
	}
}

func mustLog(t *testing.T, repo *Repo, ref string, limit int) []Commit {
	t.Helper()
	commits, err := repo.Log(context.Background(), ref, "", 0, limit)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	return commits
}

// The history filter is handed to git so that a match older than the loaded
// page is still found.
func TestLogGrepSearchesTheWholeHistory(t *testing.T) {
	ctx := context.Background()
	repo, dir := newTestRepo(t)

	for _, subject := range []string{
		"feat(payment): add retry",
		"docs: tidy the readme",
		"fix(payment): bound the retry loop",
	} {
		writeFile(t, dir, "f.txt", subject+"\n")
		gitRun(t, dir, "add", ".")
		gitRun(t, dir, "commit", "-m", subject)
	}

	// A page smaller than the history still finds the oldest match.
	commits, err := repo.Log(ctx, "", "payment", 0, 2)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if got := subjects(commits); len(got) != 2 {
		t.Fatalf("Log(grep) = %v, want the two payment commits", got)
	}
	for _, c := range commits {
		if !strings.Contains(c.Subject, "payment") {
			t.Errorf("%q does not match the query", c.Subject)
		}
	}

	// Matching is case-insensitive and literal, not a regular expression.
	if got, err := repo.Log(ctx, "", "PAYMENT", 0, 10); err != nil {
		t.Fatalf("Log: %v", err)
	} else if len(got) != 2 {
		t.Errorf("a differently-cased query matched %d commits, want 2", len(got))
	}

	if got, err := repo.Log(ctx, "", "retry.*loop", 0, 10); err != nil {
		t.Fatalf("Log: %v", err)
	} else if len(got) != 0 {
		t.Errorf("a regular expression matched %d commits; the query is literal", len(got))
	}
}
