package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mustafakarakulak/tuigy/internal/git"
)

// maxPreviewBytes caps what the preview pane will read from disk. It is the
// size past which a file stops being something anyone reads in a side pane.
const maxPreviewBytes = 2 << 20

// The files tab is the repository as git sees it: tracked files plus untracked
// ones that .gitignore does not exclude.
//
// It is a tree rather than a list because that is the shape the question has —
// "what is in here" is asked of a directory, not of a flat set of paths — and
// because the changed-files list next door already covers the other question.

// treeNode is one entry in the repository tree. A directory holds children; a
// file holds none.
type treeNode struct {
	name string
	// path is relative to the repository root, and is what the expanded set and
	// the status lookup are both keyed by.
	path     string
	dir      bool
	children []*treeNode
}

// fileRow is one drawn line of the tree.
type fileRow struct {
	node  *treeNode
	depth int
}

// buildTree turns a sorted list of paths into a tree. The root itself is never
// drawn: its children are the top level of the repository.
func buildTree(paths []string) *treeNode {
	root := &treeNode{dir: true}

	for _, p := range paths {
		parts := strings.Split(p, "/")
		node := root

		for i, part := range parts {
			if part == "" {
				break
			}
			isDir := i < len(parts)-1
			node = node.child(part, strings.Join(parts[:i+1], "/"), isDir)
		}
	}

	root.sort()
	return root
}

// child finds or creates the named entry under n.
func (n *treeNode) child(name, path string, dir bool) *treeNode {
	for _, c := range n.children {
		if c.name == name {
			return c
		}
	}

	// git lists a wholly untracked directory as a single entry ending in "/",
	// so a name can arrive as a directory with nothing under it.
	created := &treeNode{name: name, path: path, dir: dir}
	n.children = append(n.children, created)
	return created
}

// sort orders every level: directories first, then files, each alphabetically.
// That is the order a file manager uses and the one people expect to scan.
func (n *treeNode) sort() {
	slices.SortFunc(n.children, compareNodes)
	for _, c := range n.children {
		c.sort()
	}
}

func compareNodes(a, b *treeNode) int {
	if a.dir != b.dir {
		if a.dir {
			return -1
		}
		return 1
	}
	return strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
}

// flattenTree lists the rows to draw, descending only into directories the user
// has opened.
func flattenTree(root *treeNode, expanded map[string]bool) []fileRow {
	if root == nil {
		return nil
	}

	var rows []fileRow
	var walk func(n *treeNode, depth int)
	walk = func(n *treeNode, depth int) {
		for _, c := range n.children {
			rows = append(rows, fileRow{node: c, depth: depth})
			if c.dir && expanded[c.path] {
				walk(c, depth+1)
			}
		}
	}

	walk(root, 0)
	return rows
}

// expandTo opens every directory above path, which is what makes a filtered
// result and a freshly changed file reachable without any navigating.
func expandTo(expanded map[string]bool, path string) {
	for i, r := range path {
		if r == '/' {
			expanded[path[:i]] = true
		}
	}
}

// expandEverything opens every directory the paths imply.
//
// It is derived from the paths rather than from the tree, because every
// directory in the tree is by definition an ancestor of some file in it.
func expandEverything(paths []string) map[string]bool {
	expanded := make(map[string]bool)
	for _, p := range paths {
		expandTo(expanded, p)
	}
	return expanded
}

// expandSubtree opens a directory and everything underneath it.
func expandSubtree(expanded map[string]bool, paths []string, dir string) {
	expanded[dir] = true

	prefix := dir + "/"
	for _, p := range paths {
		if strings.HasPrefix(p, prefix) {
			expandTo(expanded, p)
		}
	}
}

// collapseSubtree closes a directory and forgets that anything inside it was
// open, so that opening it again shows one level rather than however many were
// unfolded before.
func collapseSubtree(expanded map[string]bool, dir string) {
	prefix := dir + "/"
	for open := range expanded {
		if open == dir || strings.HasPrefix(open, prefix) {
			delete(expanded, open)
		}
	}
}

// parentDir is the directory holding a path, or empty at the top of the tree.
func parentDir(p string) string {
	if i := strings.LastIndexByte(p, '/'); i > 0 {
		return p[:i]
	}
	return ""
}

// changedPaths is the set of files the status reports, used to mark the tree.
func changedPaths(st *git.Status) map[string]git.FileStatus {
	if st == nil {
		return nil
	}
	out := make(map[string]git.FileStatus, len(st.Files))
	for _, f := range st.Files {
		out[f.Path] = f
	}
	return out
}

// fileLetter is the status mark for a path, empty when git has nothing to say
// about it. The worktree side is preferred: what is on disk is what the tree
// is showing.
func fileLetter(changed map[string]git.FileStatus, path string) string {
	f, ok := changed[path]
	if !ok {
		return ""
	}
	switch {
	case f.Unmerged:
		return "!"
	case f.Worktree != git.StatusUnmodified:
		return f.Worktree.String()
	default:
		return f.Index.String()
	}
}

// dirHasChanges reports whether anything under a directory has changed, so that
// a closed directory still says there is something inside worth opening.
func dirHasChanges(changed map[string]git.FileStatus, dir string) bool {
	prefix := dir + "/"
	for path := range changed {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// buildFileRows applies the filter and works out which directories to open.
//
// A filter matches paths rather than names, and every directory above a match
// is opened: a filtered tree that you still have to unfold by hand would answer
// nothing the unfiltered one did not.
func buildFileRows(paths []string, filter string, expanded map[string]bool) []fileRow {
	if filter == "" {
		return flattenTree(buildTree(paths), expanded)
	}

	matched := make([]string, 0, len(paths))
	open := make(map[string]bool, len(expanded))
	for _, p := range paths {
		if !matchesFilter(filter, p) {
			continue
		}
		matched = append(matched, p)
		expandTo(open, p)
	}
	return flattenTree(buildTree(matched), open)
}

// renderFileTree draws the visible window of the tree.
func renderFileTree(rows []fileRow, cursor, offset, width, height int,
	changed map[string]git.FileStatus, expanded map[string]bool) string {

	if len(rows) == 0 {
		return styleDim.Render("nothing here that git tracks")
	}

	end := min(offset+height, len(rows))
	lines := make([]string, 0, end-offset)

	for i := offset; i < end; i++ {
		r := rows[i]
		indent := strings.Repeat("  ", r.depth)

		// Width budget: the status column, a space, and the indent.
		name := truncateLeft(treeLabel(r.node, expanded), max(width-2-len(indent), 1))
		mark, markStyle := treeMark(r.node, changed)

		if i == cursor {
			lines = append(lines, styleSelected.Width(width).Render(mark+" "+indent+name))
			continue
		}
		lines = append(lines, markStyle.Render(mark)+" "+indent+treeStyle(r.node).Render(name))
	}

	return strings.Join(lines, "\n")
}

// treeLabel names an entry: a directory carries the arrow that says whether it
// is open, and a trailing slash so it reads as a directory even without one.
func treeLabel(n *treeNode, expanded map[string]bool) string {
	if !n.dir {
		return n.name
	}
	if expanded[n.path] {
		return "▾ " + n.name + "/"
	}
	return "▸ " + n.name + "/"
}

// treeMark is the single character in front of an entry: a file's git status,
// or a dot on a closed directory holding something that has changed.
func treeMark(n *treeNode, changed map[string]git.FileStatus) (string, lipgloss.Style) {
	if !n.dir {
		if letter := fileLetter(changed, n.path); letter != "" {
			return letter, letterStyle(letter)
		}
		return " ", styleBase
	}
	if dirHasChanges(changed, n.path) {
		return "●", styleDirty
	}
	return " ", styleBase
}

func treeStyle(n *treeNode) lipgloss.Style {
	if n.dir {
		return styleBranch
	}
	return styleBase
}

// ---------------------------------------------------------------- preview

// previewNote describes a file the preview pane will not show, and is empty for
// one it will.
//
// A tree in a tool that cannot edit is for reading, so the pane shows the file.
// Two kinds of file are not worth showing: one too big to hold and one that is
// not text, and saying which is more useful than a screen of noise.
func previewNote(data []byte, size int64) string {
	if size > maxPreviewBytes {
		return fmt.Sprintf("%s — too large to preview", humanSize(size))
	}
	if isBinary(data) {
		return fmt.Sprintf("binary file, %s", humanSize(size))
	}
	return ""
}

// isBinary uses git's own test: a NUL byte near the start of the file.
func isBinary(data []byte) bool {
	for _, b := range data[:min(len(data), 8000)] {
		if b == 0 {
			return true
		}
	}
	return false
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f kB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}

// countFileRows is how many files the tree is currently showing, for the
// header's "n of m" when a filter is on. Directories are not counted: they are
// scaffolding, not results.
func countFileRows(rows []fileRow) int {
	n := 0
	for _, r := range rows {
		if !r.node.dir {
			n++
		}
	}
	return n
}

// maxPreviewLines caps what the preview renders. Past it a file is no longer
// being read, only scrolled, and the viewport pays for every line.
const maxPreviewLines = 5000

// renderPreview lays out a file's contents with a line-number gutter.
//
// The gutter is what makes the pane worth having over an editor's own: it is
// where you read a path and a line number out of a stack trace.
func renderPreview(text string, width int) string {
	if text == "" {
		return styleDim.Render("empty file")
	}

	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	truncated := false
	if len(lines) > maxPreviewLines {
		lines, truncated = lines[:maxPreviewLines], true
	}

	gutter := len(fmt.Sprint(len(lines)))
	out := make([]string, 0, len(lines)+1)
	for i, line := range lines {
		number := styleDim.Render(fmt.Sprintf("%*d ", gutter, i+1))
		out = append(out, number+styleBase.Render(clipStyled(expandTabs(line), max(width-gutter-1, 1))))
	}

	if truncated {
		out = append(out, styleDim.Render(fmt.Sprintf("… %d lines shown", maxPreviewLines)))
	}
	return strings.Join(out, "\n")
}

// renderDirSummary stands in for a preview when the cursor is on a directory:
// what is inside it, and how much of that has changed.
func renderDirSummary(n *treeNode, width int) string {
	files, dirs := 0, 0
	var walk func(*treeNode)
	walk = func(node *treeNode) {
		for _, c := range node.children {
			if c.dir {
				dirs++
				walk(c)
				continue
			}
			files++
		}
	}
	walk(n)

	return strings.Join([]string{
		styleBranch.Render(clipStyled(n.path+"/", width)),
		"",
		styleDim.Render(fmt.Sprintf("%s, %s", plural(files, "file"), plural(dirs, "folder"))),
	}, "\n")
}
