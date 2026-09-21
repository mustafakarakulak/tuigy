package ui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafakarakulak/tuigy/internal/config"
	"github.com/mustafakarakulak/tuigy/internal/git"
)

// newSecondRepo builds another repository for the switcher to move to.
func newSecondRepo(t *testing.T, name string) *git.Repo {
	t.Helper()

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	dir = filepath.Join(dir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	for _, args := range [][]string{
		{"init", "-b", "trunk"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "other.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo, err := git.Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}
	return repo
}

// Opening a repository is what puts it in the list: there is no step to forget.
func TestOpeningARepositoryRemembersIt(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, dir := newTestModel(t, 120, 32)

	msg, ok := m.rememberProject()().(projectsMsg)
	if !ok {
		t.Fatal("rememberProject did not produce a project list")
	}
	if len(msg.projects) != 1 || msg.projects[0].Path != dir {
		t.Fatalf("got %+v, want one entry for %s", msg.projects, dir)
	}
}

func TestTheSwitcherListsOtherRepositories(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, dir := newTestModel(t, 120, 32)
	other := newSecondRepo(t, "elsewhere")

	m = m.withProjects(t, dir, other.Root)
	m, _ = m.press(t, "ctrl+p")

	if m.modal != modalProjects {
		t.Fatal("ctrl+p did not open the switcher")
	}

	body := plain(m.projectsBox())
	if !strings.Contains(body, "elsewhere") {
		t.Errorf("the switcher does not list the other repository:\n%s", body)
	}
	// Switching to where you already are is not something anyone means to do.
	if matches := m.matchingProjects(); len(matches) != 1 {
		t.Errorf("the switcher offers %d repositories, want only the other one", len(matches))
	}
}

func TestTypingNarrowsTheSwitcher(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, dir := newTestModel(t, 120, 32)
	one := newSecondRepo(t, "payments")
	two := newSecondRepo(t, "billing")

	m = m.withProjects(t, dir, one.Root, two.Root)
	m, _ = m.press(t, "ctrl+p")

	for _, r := range "pay" {
		m, _ = m.press(t, string(r))
	}

	matches := m.matchingProjects()
	if len(matches) != 1 || matches[0].Name != "payments" {
		t.Fatalf("filtering by \"pay\" gave %+v, want only payments", matches)
	}

	m, _ = m.press(t, "backspace")
	m, _ = m.press(t, "backspace")
	m, _ = m.press(t, "backspace")
	if len(m.matchingProjects()) != 2 {
		t.Error("clearing the filter did not bring the other repository back")
	}
}

// The whole point: enter moves the interface to another repository, in place.
func TestSwitchingReplacesTheRepository(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, dir := newTestModel(t, 120, 32)
	other := newSecondRepo(t, "elsewhere")

	m = m.withProjects(t, dir, other.Root)
	m, _ = m.press(t, "ctrl+p")

	_, cmd := m.press(t, "enter")
	if cmd == nil {
		t.Fatal("enter did not start opening the repository")
	}
	opened, ok := cmd().(repoOpenedMsg)
	if !ok {
		t.Fatalf("enter produced %T, want a repoOpenedMsg", cmd())
	}
	if opened.err != nil {
		t.Fatalf("opening the repository failed: %v", opened.err)
	}

	next, _ := m.Update(opened)
	m = next.(Model)

	if m.repo.Root != other.Root {
		t.Fatalf("the model is on %s, want %s", m.repo.Root, other.Root)
	}
	if m.modal != modalNone {
		t.Error("the switcher stayed open after switching")
	}

	// Nothing from the old repository may be left describing the new one.
	if len(m.rows) != 0 || m.status != nil || len(m.allFiles) != 0 {
		t.Error("state from the previous repository survived the switch")
	}
	if m.filter != "" {
		t.Error("a filter set in the previous repository survived the switch")
	}
}

// A remembered path can have been moved or deleted since; that is reported
// rather than left as a switcher that does nothing when you press enter.
func TestSwitchingToAMissingRepositoryIsReported(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, _ := newTestModel(t, 120, 32)

	msg := openRepo(config.Project{Path: filepath.Join(t.TempDir(), "gone"), Name: "gone"})().(repoOpenedMsg)
	next, _ := m.Update(msg)

	if next.(Model).err == nil {
		t.Error("a repository that could not be opened was not reported")
	}
}

func TestForgettingDropsARepositoryFromTheList(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, dir := newTestModel(t, 120, 32)
	other := newSecondRepo(t, "elsewhere")

	m = m.withProjects(t, dir, other.Root)
	m, _ = m.press(t, "ctrl+p")

	_, cmd := m.press(t, "D")
	if cmd == nil {
		t.Fatal("D did not forget the selected repository")
	}
	msg, ok := cmd().(projectsMsg)
	if !ok {
		t.Fatalf("forgetting produced %T, want a projectsMsg", cmd())
	}
	for _, p := range msg.projects {
		if p.Path == other.Root {
			t.Error("the repository is still in the list after being forgotten")
		}
	}
}

func TestTheSwitcherSaysSoWhenThereIsNowhereToGo(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, dir := newTestModel(t, 120, 32)

	m = m.withProjects(t, dir)
	m, _ = m.press(t, "ctrl+p")

	if body := plain(m.projectsBox()); !strings.Contains(body, "another repository") {
		t.Errorf("an empty switcher says nothing useful:\n%s", body)
	}
}

func TestEscapeLeavesTheSwitcher(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m, _ := newTestModel(t, 120, 32)

	m, _ = m.press(t, "ctrl+p")
	m, _ = m.press(t, "esc")

	if m.modal != modalNone {
		t.Error("esc did not leave the switcher")
	}
}

// withProjects records each path and loads the resulting list into the model.
func (m Model) withProjects(t *testing.T, paths ...string) Model {
	t.Helper()

	var projects []config.Project
	for _, path := range paths {
		var err error
		projects, err = config.RememberProject(path, filepath.Base(path))
		if err != nil {
			t.Fatalf("RememberProject(%q): %v", path, err)
		}
	}

	next, _ := m.Update(projectsMsg{projects: projects})
	return next.(Model)
}
