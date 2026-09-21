package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// isolate points the configuration directory at a temporary one, so a test can
// never read or write the user's own files.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "tuigy", "projects.yml")
}

func TestNoProjectsFileIsNotAnError(t *testing.T) {
	isolate(t)

	projects, err := LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("got %d projects from nothing, want 0", len(projects))
	}
}

func TestRememberRecordsARepository(t *testing.T) {
	path := isolate(t)

	projects, err := RememberProject("/src/tuigy", "tuigy")
	if err != nil {
		t.Fatalf("RememberProject: %v", err)
	}
	if len(projects) != 1 || projects[0].Path != "/src/tuigy" || projects[0].Name != "tuigy" {
		t.Fatalf("got %+v, want one entry for /src/tuigy", projects)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("the list was not written to %s: %v", path, err)
	}

	// It survives being read back by another process.
	reloaded, err := LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects: %v", err)
	}
	if len(reloaded) != 1 || reloaded[0].Path != "/src/tuigy" {
		t.Errorf("got %+v after reloading, want the same one entry", reloaded)
	}
}

// Opening a repository again moves it to the top rather than adding it twice:
// the list is what you last worked on, in order.
func TestRememberMovesARepositoryToTheTop(t *testing.T) {
	isolate(t)

	mustRemember(t, "/src/one", "one")
	mustRemember(t, "/src/two", "two")
	projects := mustRemember(t, "/src/one", "one")

	if len(projects) != 2 {
		t.Fatalf("got %d projects, want 2", len(projects))
	}
	if projects[0].Path != "/src/one" {
		t.Errorf("the most recent project is %q, want /src/one", projects[0].Path)
	}
}

func TestForgetDropsAnEntry(t *testing.T) {
	isolate(t)

	mustRemember(t, "/src/one", "one")
	mustRemember(t, "/src/two", "two")

	projects, err := ForgetProject("/src/one")
	if err != nil {
		t.Fatalf("ForgetProject: %v", err)
	}
	if len(projects) != 1 || projects[0].Path != "/src/two" {
		t.Errorf("got %+v, want only /src/two", projects)
	}
}

// The list is a switcher, not an archive, so it stops growing.
func TestTheListIsCapped(t *testing.T) {
	isolate(t)

	for i := range maxProjects + 10 {
		mustRemember(t, filepath.Join("/src", string(rune('a'+i%26))+time.Now().Format("150405.000000000")), "p")
	}

	projects, err := LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects: %v", err)
	}
	if len(projects) != maxProjects {
		t.Errorf("got %d projects, want the cap of %d", len(projects), maxProjects)
	}
}

// A projects file that cannot be parsed is reported, but never stops tuigy:
// the next thing written repairs it.
func TestABrokenListIsReportedThenReplaced(t *testing.T) {
	path := isolate(t)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("projects: [oh dear\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := LoadProjects(); err == nil {
		t.Error("a broken projects file should be reported")
	}

	projects, err := RememberProject("/src/tuigy", "tuigy")
	if err != nil {
		t.Fatalf("RememberProject over a broken file: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("got %+v, want the list repaired to one entry", projects)
	}
}

// An entry with no path is meaningless and is dropped rather than shown.
func TestEntriesWithoutAPathAreIgnored(t *testing.T) {
	path := isolate(t)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("projects:\n  - name: ghost\n  - path: /src/real\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	projects, err := LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects: %v", err)
	}
	if len(projects) != 1 || projects[0].Path != "/src/real" {
		t.Errorf("got %+v, want only the entry with a path", projects)
	}
}

func mustRemember(t *testing.T, path, name string) []Project {
	t.Helper()
	projects, err := RememberProject(path, name)
	if err != nil {
		t.Fatalf("RememberProject(%q): %v", path, err)
	}
	return projects
}
