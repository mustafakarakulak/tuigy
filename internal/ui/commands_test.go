package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mustafakarakulak/tuigy/internal/git"
)

// The commands are what actually talk to git. The rest of the tests feed the
// model messages directly, so these exercise the other half of that contract:
// that each command really produces the message the model expects.

func TestLoadStatusProducesAStatusMessage(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	msg, ok := m.loadStatus()().(statusMsg)
	if !ok {
		t.Fatalf("loadStatus produced %T, want statusMsg", m.loadStatus()())
	}
	if msg.status == nil {
		t.Fatal("statusMsg carries no status")
	}
	if msg.status.Branch != "main" {
		t.Errorf("branch = %q, want main", msg.status.Branch)
	}
	if msg.state != git.OpNone {
		t.Errorf("state = %q, want none", msg.state)
	}
}

func TestLoadBranchesProducesABranchMessage(t *testing.T) {
	m, _ := newBranchModel(t, 120, 32)

	msg, ok := m.loadBranches()().(branchesMsg)
	if !ok {
		t.Fatal("loadBranches did not produce a branchesMsg")
	}

	var names []string
	for _, b := range msg.branches {
		names = append(names, b.Name)
	}
	if !strings.Contains(strings.Join(names, " "), "develop") {
		t.Errorf("branches = %v, want develop among them", names)
	}
}

func TestLoadDiffProducesADiffMessage(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	r, ok := m.selected()
	if !ok {
		t.Fatal("no file selected")
	}

	msg, ok := m.loadDiff(r)().(diffMsg)
	if !ok {
		t.Fatal("loadDiff did not produce a diffMsg")
	}
	if msg.key != diffKey(r) {
		t.Errorf("key = %q, want the row's key", msg.key)
	}
	if !strings.Contains(msg.text, "diff --git") {
		t.Errorf("text does not look like a diff:\n%s", msg.text)
	}
}

// A diff message arriving for a row the cursor has left is ignored, so that a
// slow request cannot overwrite the pane with an older file's diff.
func TestStaleDiffMessagesAreIgnored(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	current, _ := m.selected()
	next, _ := m.Update(diffMsg{key: "some other row", text: "should not appear"})
	m = next.(Model)

	if strings.Contains(m.diff.View(), "should not appear") {
		t.Error("a stale diff was rendered")
	}

	next, _ = m.Update(diffMsg{key: diffKey(current), text: "@@ -1 +1 @@\n+fresh"})
	m = next.(Model)
	if !strings.Contains(m.diff.View(), "fresh") {
		t.Error("the current row's diff was not rendered")
	}
}

func TestLoadHistoryAndDetailProduceMessages(t *testing.T) {
	m, _ := newHistoryModel(t, 120, 32)

	commitsMessage, ok := m.loadHistory("feature", 0)().(commitsMsg)
	if !ok {
		t.Fatal("loadHistory did not produce a commitsMsg")
	}
	if commitsMessage.ref != "feature" || len(commitsMessage.commits) == 0 {
		t.Fatalf("commitsMsg = %+v", commitsMessage)
	}

	head := commitsMessage.commits[0]
	detailMessage, ok := m.loadDetail(head)().(detailMsg)
	if !ok {
		t.Fatal("loadDetail did not produce a detailMsg")
	}
	if detailMessage.key != head.Hash {
		t.Errorf("key = %q, want the commit hash", detailMessage.key)
	}
	if detailMessage.detail.Commit.Subject != head.Subject {
		t.Errorf("detail is for %q, want %q", detailMessage.detail.Commit.Subject, head.Subject)
	}
}

func TestLoadDetailReportsAFailure(t *testing.T) {
	m, _ := newHistoryModel(t, 120, 32)

	if _, ok := m.loadDetail(git.Commit{Hash: "0000000000000000000000000000000000000000"})().(errMsg); !ok {
		t.Error("a missing commit should produce an errMsg")
	}
}

func TestLoadStashCommandsProduceMessages(t *testing.T) {
	m, dir := newStashModel(t, 120, 32)
	run(t, dir, "stash", "push", "-m", "saved for later")

	stashMessage, ok := m.loadStashes()().(stashesMsg)
	if !ok {
		t.Fatal("loadStashes did not produce a stashesMsg")
	}
	if len(stashMessage.stashes) != 1 {
		t.Fatalf("%d stashes, want 1", len(stashMessage.stashes))
	}

	diffMessage, ok := m.loadStashDiff(stashMessage.stashes[0])().(stashDiffMsg)
	if !ok {
		t.Fatal("loadStashDiff did not produce a stashDiffMsg")
	}
	if !strings.Contains(diffMessage.text, "service.go") {
		t.Errorf("stash diff does not mention the changed file:\n%s", diffMessage.text)
	}
}

func TestLoadStashDiffReportsAFailure(t *testing.T) {
	m, _ := newStashModel(t, 120, 32)

	if _, ok := m.loadStashDiff(git.Stash{Ref: "stash@{99}"})().(errMsg); !ok {
		t.Error("a missing stash should produce an errMsg")
	}
}

// Init starts the loads the first frame needs.
func TestInitLoadsStatusAndBranches(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	batch, ok := m.Init()().(tea.BatchMsg)
	if !ok {
		t.Fatal("Init did not produce a batch of commands")
	}
	if len(batch) != 3 {
		t.Errorf("Init started %d commands, want status, branches and the tick", len(batch))
	}
}

// A failing operation surfaces its error and still refreshes, so the view never
// disagrees with the repository about what happened.
func TestFailedOperationReportsAndRefreshes(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	next, cmd := m.Update(opDoneMsg{label: "did a thing", err: errStub("it did not work")})
	m = next.(Model)

	if m.err == nil {
		t.Fatal("a failed operation should be reported")
	}
	if cmd == nil {
		t.Error("a failed operation should still refresh")
	}
	if !strings.Contains(m.footerView(), "it did not work") {
		t.Errorf("the footer does not show the failure:\n%s", m.footerView())
	}
}

// A success message is shown briefly and then cleared.
func TestFlashIsClearedAfterwards(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	next, _ := m.Update(opDoneMsg{label: "staged everything"})
	m = next.(Model)
	if m.flash != "staged everything" {
		t.Fatalf("flash = %q", m.flash)
	}

	// A clear for an older flash must not wipe a newer one.
	next, _ = m.Update(clearFlashMsg{id: m.flashID - 1})
	m = next.(Model)
	if m.flash == "" {
		t.Error("an out-of-date clear wiped the current message")
	}

	next, _ = m.Update(clearFlashMsg{id: m.flashID})
	if next.(Model).flash != "" {
		t.Error("the message was not cleared")
	}
}

// Returning from the editor refreshes, since the file has probably changed.
func TestEditorReturnRefreshes(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	next, cmd := m.Update(editorDoneMsg{})
	if cmd == nil {
		t.Error("returning from the editor should refresh")
	}
	if next.(Model).err != nil {
		t.Errorf("unexpected error: %v", next.(Model).err)
	}

	next, _ = m.Update(editorDoneMsg{err: errStub("editor exploded")})
	if next.(Model).err == nil {
		t.Error("an editor failure should be reported")
	}
}

type errStub string

func (e errStub) Error() string { return string(e) }

// A slow push used to show nothing at all, which reads as a frozen program.
func TestRunningOperationIsVisible(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	next, cmd := m.Update(opStartedMsg{label: "pushing main"})
	m = next.(Model)

	if m.busy != "pushing main" {
		t.Fatalf("busy = %q, want the operation", m.busy)
	}
	if cmd == nil {
		t.Error("the spinner should start turning")
	}
	if got := m.footerView(); !strings.Contains(got, "pushing main") {
		t.Errorf("the footer does not say what is happening:\n%s", got)
	}

	// The result takes the same place, so nothing jumps.
	next, _ = m.Update(opDoneMsg{label: "pushed main"})
	m = next.(Model)

	if m.busy != "" {
		t.Errorf("busy = %q after the operation finished", m.busy)
	}
	if got := m.footerView(); !strings.Contains(got, "pushed main") {
		t.Errorf("the footer does not report the result:\n%s", got)
	}
}

// The spinner stops on its own rather than ticking forever in the background.
func TestSpinnerStopsWhenIdle(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	next, _ := m.Update(opStartedMsg{label: "fetching"})
	m = next.(Model)

	next, cmd := m.Update(m.spinner.Tick())
	m = next.(Model)
	if cmd == nil {
		t.Error("the spinner should keep turning while busy")
	}

	next, _ = m.Update(opDoneMsg{label: "fetched"})
	m = next.(Model)

	if _, cmd := m.Update(m.spinner.Tick()); cmd != nil {
		t.Error("the spinner should stop once there is nothing to wait for")
	}
}

// A failure clears the busy state too, or the spinner would turn forever.
func TestFailedOperationStopsTheSpinner(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	next, _ := m.Update(opStartedMsg{label: "pushing"})
	m = next.(Model)

	next, _ = m.Update(opDoneMsg{label: "pushed", err: errStub("no network")})
	m = next.(Model)

	if m.busy != "" {
		t.Errorf("busy = %q after a failure", m.busy)
	}
	if !strings.Contains(m.footerView(), "no network") {
		t.Errorf("the footer does not show the failure:\n%s", m.footerView())
	}
}

// Every operation says what it is doing as well as what it did.
func TestOperationsHaveBothLabels(t *testing.T) {
	m, _ := newTestModel(t, 120, 32)

	next, cmd := m.press(t, "a") // stage all
	m = next

	msg := cmd()
	grouped, ok := groupedCommands(msg)
	if !ok || len(grouped) != 2 {
		t.Fatalf("an operation produced %T, want a started/done pair", msg)
	}

	started, ok := grouped[0]().(opStartedMsg)
	if !ok {
		t.Fatalf("the first message is %T, want opStartedMsg", grouped[0]())
	}
	if started.label == "" {
		t.Error("the operation does not say what it is doing")
	}
	if strings.HasSuffix(started.label, "ed") {
		t.Errorf("the label %q reads as finished, not as in progress", started.label)
	}
}
