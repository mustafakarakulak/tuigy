package ui

import (
	"strings"
	"testing"
)

func TestTokenise(t *testing.T) {
	got := tokenise("a := foo(bar_1)")
	want := []string{"a", " ", ":=", " ", "foo", "(", "bar_1", ")"}

	if len(got) != len(want) {
		t.Fatalf("tokenise() = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWordDiffFindsTheChangedPart(t *testing.T) {
	left, right := wordDiff(
		"func Charge(amount int64, currency string) error {",
		"func Charge(amount uint64, currency string) error {",
	)

	changed := func(parts []part) []string {
		var out []string
		for _, p := range parts {
			if p.changed {
				out = append(out, p.text)
			}
		}
		return out
	}

	if got := changed(left); len(got) != 1 || got[0] != "int64" {
		t.Errorf("removed side marks %q, want only int64", got)
	}
	if got := changed(right); len(got) != 1 || got[0] != "uint64" {
		t.Errorf("added side marks %q, want only uint64", got)
	}

	// Everything else has to survive, or the line would be rendered wrong.
	rebuild := func(parts []part) string {
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(p.text)
		}
		return b.String()
	}
	if got := rebuild(left); got != "func Charge(amount int64, currency string) error {" {
		t.Errorf("the removed line does not reassemble: %q", got)
	}
}

// Two entirely different lines are all change; two identical ones are none.
func TestWordDiffExtremes(t *testing.T) {
	left, right := wordDiff("alpha beta", "alpha beta")
	for _, parts := range [][]part{left, right} {
		for _, p := range parts {
			if p.changed {
				t.Errorf("identical lines marked %q as changed", p.text)
			}
		}
	}

	left, _ = wordDiff("aaa", "zzz")
	if len(left) != 1 || !left[0].changed {
		t.Errorf("wholly different lines = %+v, want one changed part", left)
	}
}

// The comparison is quadratic, so a minified line falls back rather than stalls.
func TestWordDiffGivesUpOnEnormousLines(t *testing.T) {
	huge := strings.Repeat("token ", maxWordDiffTokens)

	left, right := wordDiff(huge+"a", huge+"b")
	if len(left) != 1 || len(right) != 1 {
		t.Errorf("a very long line was compared word by word: %d/%d parts", len(left), len(right))
	}
}

func TestRenderDiffEmphasisesChangedWords(t *testing.T) {
	raw := "@@ -1,2 +1,2 @@\n context\n-var timeout = 30\n+var timeout = 60\n"

	got := renderDiff(raw, noHunk).content
	if !strings.Contains(got, "30") || !strings.Contains(got, "60") {
		t.Fatalf("the diff lost its content:\n%s", got)
	}

	// The changed number carries the emphasis style and the rest does not.
	if !strings.Contains(got, styleRemovedEmph.Render("30")) {
		t.Errorf("the removed value is not emphasised:\n%q", got)
	}
	if !strings.Contains(got, styleAddedEmph.Render("60")) {
		t.Errorf("the added value is not emphasised:\n%q", got)
	}
	if strings.Contains(got, styleRemovedEmph.Render("var timeout = 30")) {
		t.Error("the whole removed line was emphasised rather than the change")
	}
}

// A block whose removed and added counts differ is not an edited line, so it is
// left alone rather than paired up arbitrarily.
func TestRenderDiffLeavesUnevenBlocksAlone(t *testing.T) {
	raw := "@@ -1,3 +1,2 @@\n-one\n-two\n-three\n+only\n"

	got := renderDiff(raw, noHunk).content
	for _, want := range []string{"one", "two", "three", "only"} {
		if !strings.Contains(got, want) {
			t.Errorf("the diff lost %q:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "\n") + 1; n != 5 {
		t.Errorf("rendered %d lines, want the five that went in:\n%s", n, got)
	}
}

// Long lines are kept whole: the pane scrolls sideways, so cutting them here
// would throw away the part the user wants to reach.
func TestRenderDiffKeepsLongLinesWhole(t *testing.T) {
	long := strings.Repeat("x", 300)
	got := renderDiff("@@ -1 +1 @@\n+"+long+"\n", noHunk).content

	if !strings.Contains(got, long) {
		t.Error("a long line was cut during rendering")
	}
}

func TestRenderDiffFindsHunks(t *testing.T) {
	raw := "diff --git a/f b/f\nindex 1..2 100644\n--- a/f\n+++ b/f\n" +
		"@@ -1,2 +1,2 @@\n context\n-old\n+new\n" +
		"@@ -20,2 +20,2 @@\n context\n-old\n+new\n"

	rendered := renderDiff(raw, noHunk)
	if len(rendered.hunks) != 2 {
		t.Fatalf("found %d hunks, want 2", len(rendered.hunks))
	}

	lines := strings.Split(rendered.content, "\n")
	for _, at := range rendered.hunks {
		if !strings.Contains(lines[at], "@@") {
			t.Errorf("line %d is not a hunk header: %q", at, lines[at])
		}
	}
}

func TestHunkNavigation(t *testing.T) {
	hunks := []int{4, 20, 50}

	for _, c := range []struct{ from, next, prev int }{
		{0, 4, 0},
		{4, 20, 0},
		{20, 50, 4},
		{50, 50, 20}, // already at the last one
		{99, 50, 50}, // past the end
	} {
		if got := nextHunk(hunks, c.from); got != c.next {
			t.Errorf("nextHunk(%d) = %d, want %d", c.from, got, c.next)
		}
		if got := previousHunk(hunks, c.from); got != c.prev {
			t.Errorf("previousHunk(%d) = %d, want %d", c.from, got, c.prev)
		}
	}

	if got := nextHunk(nil, 7); got != 7 {
		t.Errorf("nextHunk with no hunks = %d, want the position unchanged", got)
	}
}

func TestDiffPaneScrollsSideways(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	// Load a diff with a line wider than the pane.
	long := strings.Repeat("abcdefghij", 40)
	next, _ := m.Update(diffMsg{key: m.diffKey, text: "@@ -1 +1 @@\n+" + long + "\n"})
	m = next.(Model)
	m, _ = m.press(t, "enter")

	before := m.diff.View()
	m, _ = m.press(t, "l")
	m, _ = m.press(t, "l")
	after := m.diff.View()

	if before == after {
		t.Error("scrolling right did not move the diff")
	}

	for range 5 {
		m, _ = m.press(t, "h")
	}
	if m.diff.View() != before {
		t.Error("scrolling back left did not return to the start")
	}
}

func TestDiffPaneJumpsBetweenHunks(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	var raw strings.Builder
	for i := range 6 {
		raw.WriteString("@@ -" + strings.Repeat("1", i+1) + ",2 +1,2 @@\n")
		for range 8 {
			raw.WriteString(" context\n")
		}
	}

	next, _ := m.Update(diffMsg{key: m.diffKey, text: raw.String()})
	m = next.(Model)
	m, _ = m.press(t, "enter")

	if len(m.diffHunks) != 6 {
		t.Fatalf("found %d hunks, want 6", len(m.diffHunks))
	}

	m, _ = m.press(t, "]")
	if m.diff.YOffset != m.diffHunks[1] {
		t.Errorf("] moved to %d, want the second hunk at %d", m.diff.YOffset, m.diffHunks[1])
	}

	m, _ = m.press(t, "[")
	if m.diff.YOffset != m.diffHunks[0] {
		t.Errorf("[ moved to %d, want the first hunk at %d", m.diff.YOffset, m.diffHunks[0])
	}
}
