package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafakarakulak/tuigy/internal/ai"
)

// withFakeAgent points the generator at a command for the rest of the test.
// It must be set before the model is built, because detection happens once.
func withFakeAgent(t *testing.T, command string) {
	t.Helper()
	t.Setenv(ai.CommandEnv, command)
}

// withoutAgent hides whatever agent the machine happens to have, while keeping
// git reachable so the fixture can still be built.
func withoutAgent(t *testing.T) {
	t.Helper()
	t.Setenv(ai.CommandEnv, "")

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

func TestGenerateCommitMessage(t *testing.T) {
	withFakeAgent(t, "printf 'feat(payment): add retry handling\\n'")
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, "c")
	if m.modal != modalCommit {
		t.Fatal("c should open the commit view")
	}
	if !strings.Contains(plain(m.View()), "ctrl+g") {
		t.Error("the commit view does not advertise generating a message")
	}

	next, cmd := m.press(t, "ctrl+g")
	m = next
	if !m.generating {
		t.Error("the view should say it is waiting on the agent")
	}
	m = m.applyCmd(t, cmd)

	if m.generating {
		t.Error("generating should be over once the answer arrives")
	}
	if got := m.commit.Value(); got != "feat(payment): add retry handling" {
		t.Errorf("commit message = %q", got)
	}
	// The message is a draft: nothing is committed until the user confirms.
	if m.modal != modalCommit {
		t.Error("the commit view should stay open for review")
	}
}

func TestGenerateReplacesTheMessageWhenAskedAgain(t *testing.T) {
	withFakeAgent(t, "printf 'fix: second attempt\\n'")
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, "c")
	for _, r := range "typed by hand" {
		m, _ = m.press(t, string(r))
	}

	next, cmd := m.press(t, "ctrl+g")
	m = next.applyCmd(t, cmd)

	if got := m.commit.Value(); got != "fix: second attempt" {
		t.Errorf("commit message = %q, want the regenerated one", got)
	}
}

func TestGenerateSurfacesTheAgentsError(t *testing.T) {
	withFakeAgent(t, "echo 'rate limited' >&2; exit 1")
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, "c")
	next, cmd := m.press(t, "ctrl+g")
	m = next.applyCmd(t, cmd)

	if m.err == nil {
		t.Fatal("the agent's failure should reach the user")
	}
	if !strings.Contains(m.err.Error(), "rate limited") {
		t.Errorf("error = %q, want the agent's own message", m.err)
	}
	if m.generating {
		t.Error("generating should be over after a failure")
	}
	if m.modal != modalCommit {
		t.Error("the commit view should stay open so the message can be typed instead")
	}
}

// Without an agent the feature stays out of the way rather than breaking.
func TestWithoutAnAgentTheFeatureIsHidden(t *testing.T) {
	withoutAgent(t)
	m, _ := newTestModel(t, 120, 32)

	if m.ai != nil {
		t.Fatal("no agent should have been detected")
	}

	m, _ = m.press(t, "c")
	if strings.Contains(plain(m.View()), "ctrl+g") {
		t.Error("the commit view should not advertise a generator that is not there")
	}
	if strings.Contains(plain(m.footerView()), "ctrl+g") {
		t.Error("the footer should not advertise a generator that is not there")
	}

	m, _ = m.press(t, "ctrl+g")
	if m.err == nil {
		t.Fatal("pressing the key anyway should explain how to set it up")
	}
	if !strings.Contains(m.err.Error(), ai.CommandEnv) {
		t.Errorf("error = %q, want it to name the environment variable", m.err)
	}
}

// The key is advertised as a general shortcut, so it has to work as one: from
// the file list it opens the commit view and starts writing, rather than
// silently doing nothing.
func TestGenerateFromTheChangesList(t *testing.T) {
	withFakeAgent(t, "printf 'feat(payment): add retry handling\\n'")
	m, _ := newTestModel(t, 120, 32)

	next, cmd := m.press(t, "ctrl+g")
	m = next

	if m.modal != modalCommit {
		t.Fatal("ctrl+g should open the commit view")
	}
	if !m.generating {
		t.Error("it should start writing the message straight away")
	}
	if m.busy == "" {
		t.Error("the view should say it is waiting on the agent")
	}

	m = m.applyCmd(t, cmd)
	if got := m.commit.Value(); got != "feat(payment): add retry handling" {
		t.Errorf("commit message = %q", got)
	}
	if m.busy != "" {
		t.Errorf("busy = %q once the message has arrived", m.busy)
	}
}

func TestGenerateFromTheChangesListWithNothingStaged(t *testing.T) {
	withFakeAgent(t, "printf 'feat: x\\n'")
	m, _ := newTestModel(t, 120, 32)

	// Put back the one staged file.
	_, cmd := m.press(t, "A")
	opResult(t, cmd)
	m = m.refresh(t)

	after, _ := m.press(t, "ctrl+g")
	if after.modal == modalCommit {
		t.Error("the commit view must not open with nothing staged")
	}
	if after.generating {
		t.Error("nothing should have been asked of the agent")
	}
	if after.err == nil {
		t.Fatal("the user should be told there is nothing to describe")
	}
}

// "AI" on its own says nothing; the hint names the agent that will answer.
func TestGenerateHintNamesTheAgent(t *testing.T) {
	withFakeAgent(t, "printf 'feat: x\\n'")
	m, _ := newTestModel(t, 120, 32)

	if got := plain(m.footerView()); !strings.Contains(got, "ctrl+g") {
		t.Errorf("the footer does not offer the key at all:\n%s", got)
	}

	m, _ = m.press(t, "c")
	if got := plain(m.footerView()); !strings.Contains(got, m.ai.Name()) {
		t.Errorf("the footer does not name the agent:\n%s", got)
	}
	if got := plain(m.View()); !strings.Contains(got, m.ai.Name()) {
		t.Errorf("the commit view does not name the agent:\n%s", got)
	}
}

// Without an agent the key stays out of the way rather than opening a view and
// then explaining it cannot do the thing.
func TestGenerateFromTheChangesListWithoutAnAgent(t *testing.T) {
	withoutAgent(t)
	m, _ := newTestModel(t, 120, 32)

	if strings.Contains(plain(m.footerView()), "ctrl+g") {
		t.Error("the footer offers a generator that is not there")
	}

	after, _ := m.press(t, "ctrl+g")
	if after.err == nil {
		t.Fatal("pressing it anyway should explain how to set one up")
	}
}
