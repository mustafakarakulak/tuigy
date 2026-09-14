package git

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// newHunkRepo writes a file with three well-separated changes, so each one is
// its own hunk.
func newHunkRepo(t *testing.T) (*Repo, string) {
	t.Helper()
	repo, dir := newTestRepo(t)

	var lines []string
	for i := range 30 {
		lines = append(lines, "line "+string(rune('a'+i%26))+string(rune('0'+i/26)))
	}
	writeFile(t, dir, "file.txt", strings.Join(lines, "\n")+"\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "base")

	lines[2] = "FIRST CHANGE"
	lines[14] = "SECOND CHANGE"
	lines[26] = "THIRD CHANGE"
	writeFile(t, dir, "file.txt", strings.Join(lines, "\n")+"\n")

	return repo, dir
}

func unstagedDiff(t *testing.T, repo *Repo, path string) string {
	t.Helper()
	diff, err := repo.FileDiff(context.Background(), FileStatus{Path: path, Worktree: StatusModified}, false)
	if err != nil {
		t.Fatalf("FileDiff: %v", err)
	}
	return diff
}

func TestSplitHunks(t *testing.T) {
	repo, _ := newHunkRepo(t)
	diff := unstagedDiff(t, repo, "file.txt")

	header, hunks := SplitHunks(diff)
	if len(hunks) != 3 {
		t.Fatalf("%d hunks, want 3", len(hunks))
	}

	// The header is what git apply needs to know which file this is.
	joined := strings.Join(header, "\n")
	for _, want := range []string{"diff --git", "--- a/file.txt", "+++ b/file.txt"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the header is missing %q:\n%s", want, joined)
		}
	}

	for i, h := range hunks {
		if !strings.HasPrefix(h.Header, "@@") {
			t.Errorf("hunk %d has no header: %q", i, h.Header)
		}
		if len(h.Body) == 0 {
			t.Errorf("hunk %d has no body", i)
		}
	}
	if !strings.Contains(strings.Join(hunks[0].Body, "\n"), "FIRST CHANGE") {
		t.Error("the first hunk does not hold the first change")
	}
}

func TestHunkPatchRoundTrip(t *testing.T) {
	repo, _ := newHunkRepo(t)
	diff := unstagedDiff(t, repo, "file.txt")

	patch, err := HunkPatch(diff, 1)
	if err != nil {
		t.Fatalf("HunkPatch: %v", err)
	}

	if !strings.Contains(patch, "SECOND CHANGE") {
		t.Errorf("the patch does not hold the chosen hunk:\n%s", patch)
	}
	for _, other := range []string{"FIRST CHANGE", "THIRD CHANGE"} {
		if strings.Contains(patch, other) {
			t.Errorf("the patch also carries %q, so it is not one hunk:\n%s", other, patch)
		}
	}
	if !strings.HasSuffix(patch, "\n") {
		t.Error("a patch git will accept has to end with a newline")
	}
}

func TestHunkPatchRejectsNonsense(t *testing.T) {
	repo, _ := newHunkRepo(t)
	diff := unstagedDiff(t, repo, "file.txt")

	if _, err := HunkPatch(diff, 9); err == nil {
		t.Error("a hunk that does not exist should be an error")
	}
	if _, err := HunkPatch(diff, -1); err == nil {
		t.Error("a negative hunk should be an error")
	}
	if _, err := HunkPatch("", 0); err == nil {
		t.Error("an empty diff should be an error")
	}
}

// Staging one hunk leaves the others in the working tree, which is the whole
// point: a file ends up both staged and unstaged.
func TestStageHunkLeavesTheRestBehind(t *testing.T) {
	ctx := context.Background()
	repo, dir := newHunkRepo(t)

	diff := unstagedDiff(t, repo, "file.txt")
	if err := repo.StageHunk(ctx, diff, 1); err != nil {
		t.Fatalf("StageHunk: %v", err)
	}

	st := mustStatus(t, repo)
	if !containsFile(st.Staged(), "file.txt") {
		t.Error("nothing was staged")
	}
	if !containsFile(st.Unstaged(), "file.txt") {
		t.Error("the other hunks should still be unstaged")
	}

	staged, err := repo.FileDiff(ctx, FileStatus{Path: "file.txt", Index: StatusModified}, true)
	if err != nil {
		t.Fatalf("FileDiff: %v", err)
	}
	if !strings.Contains(staged, "SECOND CHANGE") {
		t.Errorf("the staged diff does not hold the chosen hunk:\n%s", staged)
	}
	for _, other := range []string{"FIRST CHANGE", "THIRD CHANGE"} {
		if strings.Contains(staged, other) {
			t.Errorf("%q was staged too:\n%s", other, staged)
		}
	}

	// The file on disk is untouched: only the index moved.
	body, err := os.ReadFile(filepath.Join(dir, "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	for _, want := range []string{"FIRST CHANGE", "SECOND CHANGE", "THIRD CHANGE"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the working tree lost %q", want)
		}
	}
}

func TestUnstageHunk(t *testing.T) {
	ctx := context.Background()
	repo, _ := newHunkRepo(t)

	// Stage everything, then take one hunk back out.
	if err := repo.Stage(ctx, "file.txt"); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	staged, err := repo.FileDiff(ctx, FileStatus{Path: "file.txt", Index: StatusModified}, true)
	if err != nil {
		t.Fatalf("FileDiff: %v", err)
	}
	if err := repo.UnstageHunk(ctx, staged, 0); err != nil {
		t.Fatalf("UnstageHunk: %v", err)
	}

	after, err := repo.FileDiff(ctx, FileStatus{Path: "file.txt", Index: StatusModified}, true)
	if err != nil {
		t.Fatalf("FileDiff: %v", err)
	}
	if strings.Contains(after, "FIRST CHANGE") {
		t.Errorf("the first hunk is still staged:\n%s", after)
	}
	for _, want := range []string{"SECOND CHANGE", "THIRD CHANGE"} {
		if !strings.Contains(after, want) {
			t.Errorf("%q was taken out too:\n%s", want, after)
		}
	}
}

// Staging hunks one at a time has to end up where staging the file would.
func TestStagingEveryHunkMatchesStagingTheFile(t *testing.T) {
	ctx := context.Background()
	repo, _ := newHunkRepo(t)

	for i := range 3 {
		// The diff shrinks as hunks move across, so index 0 each time.
		diff := unstagedDiff(t, repo, "file.txt")
		if err := repo.StageHunk(ctx, diff, 0); err != nil {
			t.Fatalf("StageHunk %d: %v", i, err)
		}
	}

	st := mustStatus(t, repo)
	if containsFile(st.Unstaged(), "file.txt") {
		t.Error("something was left unstaged")
	}
	if !containsFile(st.Staged(), "file.txt") {
		t.Fatal("nothing is staged")
	}
}

func TestApplyToIndexRejectsAnEmptyPatch(t *testing.T) {
	repo, _ := newTestRepo(t)
	if err := repo.ApplyToIndex(context.Background(), "  \n", false); err == nil {
		t.Error("an empty patch should be an error")
	}
}

// A patch that does not fit is reported rather than half-applied.
func TestApplyToIndexReportsAPatchThatDoesNotFit(t *testing.T) {
	repo, _ := newHunkRepo(t)

	bad := "diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n" +
		"@@ -1,1 +1,1 @@\n-nothing like the real line\n+something else\n"

	if err := repo.ApplyToIndex(context.Background(), bad, false); err == nil {
		t.Error("a patch that does not match should fail")
	}
}

func containsFile(files []FileStatus, path string) bool {
	return slices.ContainsFunc(files, func(f FileStatus) bool { return f.Path == path })
}
