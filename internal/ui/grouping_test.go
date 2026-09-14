package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/mustafakarakulak/tuigy/internal/git"
)

// modified builds the file list git's parser would produce for changes that
// are in the working tree and not in the index.
func modified(paths ...string) []git.FileStatus {
	files := make([]git.FileStatus, len(paths))
	for i, p := range paths {
		files[i] = git.FileStatus{
			Path:     p,
			Index:    git.StatusUnmodified,
			Worktree: git.StatusModified,
		}
	}
	return files
}

func headings(rows []row) []string {
	var out []string
	for _, r := range rows {
		if r.header && r.dir != "" {
			out = append(out, r.dir)
		}
	}
	return out
}

// A directory holding several changed files is named once instead of being
// repeated down the column.
func TestGroupingNamesSharedDirectories(t *testing.T) {
	rows := groupByDirectory(secUnstaged, modified(
		"internal/git/diff.go",
		"internal/git/repo.go",
		"internal/ui/app.go",
		"internal/ui/view.go",
		"internal/ui/keys.go",
	))

	want := []string{"internal/git/", "internal/ui/"}
	if got := headings(rows); !slices.Equal(got, want) {
		t.Fatalf("headings = %v, want %v", got, want)
	}

	for _, r := range rows {
		if r.header {
			continue
		}
		if !r.indent {
			t.Errorf("%s is under a heading but keeps its full path", r.file.Path)
		}
		if strings.Contains(r.label(), "/") {
			t.Errorf("label = %q, want just the file name", r.label())
		}
	}
}

// A heading for one file is just another line to read, so it is not written.
func TestGroupingLeavesLoneFilesAlone(t *testing.T) {
	rows := groupByDirectory(secUnstaged, modified(
		"README.md",
		"cmd/app/main.go",
		"internal/ui/app.go",
		"internal/ui/view.go",
	))

	if got := headings(rows); !slices.Equal(got, []string{"internal/ui/"}) {
		t.Fatalf("headings = %v, want only the directory with two files", got)
	}

	byPath := map[string]row{}
	for _, r := range rows {
		if !r.header {
			byPath[r.file.Path] = r
		}
	}

	for _, lone := range []string{"README.md", "cmd/app/main.go"} {
		if byPath[lone].indent {
			t.Errorf("%s was indented under a heading of its own", lone)
		}
		if byPath[lone].label() != lone {
			t.Errorf("label = %q, want the full path %q", byPath[lone].label(), lone)
		}
	}
}

// A rename shows both paths, which no single directory heading would describe.
func TestGroupingSkipsRenames(t *testing.T) {
	files := []git.FileStatus{
		{Path: "internal/ui/new.go", OrigPath: "internal/ui/old.go", Index: git.StatusRenamed},
		{Path: "internal/ui/other.go", Index: git.StatusRenamed, OrigPath: "internal/ui/older.go"},
	}

	rows := groupByDirectory(secStaged, files)
	if got := headings(rows); len(got) != 0 {
		t.Errorf("headings = %v, want none for renames", got)
	}
	for _, r := range rows {
		if !strings.Contains(r.label(), "→") {
			t.Errorf("label = %q, want both paths", r.label())
		}
	}
}

// git reports a wholly untracked directory as one entry already.
func TestGroupingSkipsUntrackedDirectories(t *testing.T) {
	files := []git.FileStatus{
		{Path: "vendor/", Worktree: git.StatusUntracked},
		{Path: "build/", Worktree: git.StatusUntracked},
	}

	rows := groupByDirectory(secUntracked, files)
	if got := headings(rows); len(got) != 0 {
		t.Errorf("headings = %v, want none", got)
	}
	if len(rows) != 2 {
		t.Errorf("%d rows, want the two entries unchanged", len(rows))
	}
}

// Directory headings are headings: the cursor passes over them.
func TestCursorSkipsDirectoryHeadings(t *testing.T) {
	rows := groupByDirectory(secUnstaged, modified(
		"internal/ui/app.go",
		"internal/ui/view.go",
	))

	if !rows[0].header {
		t.Fatal("the first row should be the directory heading")
	}
	if got := firstFileRow(rows); got != 1 {
		t.Errorf("firstFileRow = %d, want the row after the heading", got)
	}
}

// Grouping happens after filtering, so a filter that leaves one file in a
// directory drops that directory's heading with it.
func TestGroupingFollowsTheFilter(t *testing.T) {
	status := &git.Status{Files: modified(
		"internal/ui/app.go",
		"internal/ui/view.go",
		"internal/git/repo.go",
	)}

	if got := headings(buildRows(status, "")); !slices.Equal(got, []string{"internal/ui/"}) {
		t.Errorf("unfiltered headings = %v", got)
	}
	if got := headings(buildRows(status, "app")); len(got) != 0 {
		t.Errorf("headings = %v, want none once only one file matches", got)
	}
}

// The rendered list shows the directory once and the names under it.
func TestRenderedListShowsTheGrouping(t *testing.T) {
	rows := buildRows(&git.Status{Files: modified(
		"internal/ui/app.go",
		"internal/ui/view.go",
	)}, "")

	got := plain(renderList(rows, firstFileRow(rows), 0, 46, 10, nil))
	if !strings.Contains(got, "internal/ui/") {
		t.Errorf("the directory is not named:\n%s", got)
	}
	if strings.Contains(got, "internal/ui/app.go") {
		t.Errorf("the prefix is repeated on the file line:\n%s", got)
	}
	for _, name := range []string{"app.go", "view.go"} {
		if !strings.Contains(got, name) {
			t.Errorf("the list is missing %q:\n%s", name, got)
		}
	}
}
