package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkingFilesReviewed(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	first := m.selectedPath(t)
	m, _ = m.press(t, "v")

	if _, ok := m.reviewed[first]; !ok {
		t.Fatalf("%s was not recorded as reviewed", first)
	}
	// Marking steps down, so working through a list is one key repeated.
	if m.selectedPath(t) == first {
		t.Error("the cursor did not move on after marking")
	}

	view := m.View()
	if !strings.Contains(view, "✓ "+first) && !strings.Contains(view, "✓") {
		t.Error("the list does not mark the reviewed file")
	}
	if done, total := m.reviewProgress(); done != 1 || total == 0 {
		t.Errorf("progress = %d/%d, want one file reviewed", done, total)
	}
	if !strings.Contains(view, "1/") {
		t.Error("the header does not show review progress")
	}
}

func TestMarkingReviewedIsAToggle(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	path := m.selectedPath(t)
	m, _ = m.press(t, "v")
	m.cursor = findRow(t, m, secStaged, path)

	m, _ = m.press(t, "v")
	if _, ok := m.reviewed[path]; ok {
		t.Error("pressing it again should undo the review")
	}
}

// The reason this feature exists: an agent that rewrites a file after it was
// reviewed must not leave it looking reviewed.
func TestReviewIsDroppedWhenTheFileChanges(t *testing.T) {
	m, dir := newTestModel(t, 120, 32)

	path := m.selectedPath(t)
	m, _ = m.press(t, "v")
	if _, ok := m.reviewed[path]; !ok {
		t.Fatalf("setup: %s was not reviewed", path)
	}

	// Something else rewrites it. The content is a different length so the
	// change is unmistakable even if the clock is coarse.
	write(t, dir, path, "an agent rewrote this file after it had been read\n")
	m = m.refresh(t)

	if _, ok := m.reviewed[path]; ok {
		t.Error("the review survived the file being rewritten")
	}
	if done, _ := m.reviewProgress(); done != 0 {
		t.Errorf("progress = %d, want the review dropped", done)
	}
}

// A file that changes without its status changing is the case a naive
// implementation would miss, since the poll would see nothing new.
func TestReviewIsDroppedEvenWhenTheStatusIsUnchanged(t *testing.T) {
	m, dir := newTestModel(t, 120, 32)

	// An already-modified file: rewriting it leaves the status letters alone,
	// which is exactly what makes the change invisible to a poll.
	m.cursor = findRow(t, m, secUnstaged, "app.go")
	before := m.statusFP

	m, _ = m.press(t, "v")
	write(t, dir, "app.go", "still modified, but differently than before\n")
	m = m.refresh(t)

	if m.statusFP != before {
		t.Fatal("setup: the status changed, so this is not the case being tested")
	}
	if _, ok := m.reviewed["app.go"]; ok {
		t.Error("the review survived a change the status could not see")
	}
}

func TestReviewIsDroppedWhenTheFileGoes(t *testing.T) {
	m, dir := newTestModel(t, 120, 32)

	path := m.selectedPath(t)
	m, _ = m.press(t, "v")

	if err := os.Remove(filepath.Join(dir, path)); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	m = m.refresh(t)

	if _, ok := m.reviewed[path]; ok {
		t.Error("the review survived the file being deleted")
	}
}

// Staging something you have not read is what this tool exists to prevent, so
// the commit view says so — without getting in the way.
func TestCommitWarnsAboutUnreviewedFiles(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, "c")
	if m.modal != modalCommit {
		t.Fatal("the commit view should open")
	}
	if !strings.Contains(m.View(), "not reviewed") {
		t.Errorf("the commit view does not mention unreviewed files:\n%s", m.View())
	}

	// Reviewing the staged file settles it.
	m, _ = m.press(t, "esc")
	m, _ = m.press(t, "v") // the cursor starts on the staged README.md
	m, _ = m.press(t, "c")

	if m.unreviewedStaged() != 0 {
		t.Fatalf("%d staged files are still unreviewed", m.unreviewedStaged())
	}
	if strings.Contains(m.View(), "not reviewed") {
		t.Error("the warning is still shown once everything staged has been read")
	}
}

// It is a warning, not a gate: the commit still goes through.
func TestUnreviewedFilesDoNotBlockCommitting(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, "c")
	for _, r := range "chore: commit anyway" {
		m, _ = m.press(t, string(r))
	}

	_, cmd := m.press(t, "ctrl+s")
	if done := opResult(t, cmd); done.err != nil {
		t.Fatalf("the commit was refused: %v", done.err)
	}
}
