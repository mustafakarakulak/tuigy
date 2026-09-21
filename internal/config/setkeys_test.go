package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useConfigDir points reads and writes at a temporary configuration directory.
func useConfigDir(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return configPath(t)
}

func putFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func fileBody(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(body)
}

func TestSetKeyBindingCreatesTheFile(t *testing.T) {
	path := useConfigDir(t)

	if _, err := SetKeyBinding("commit", []string{"ctrl+k"}); err != nil {
		t.Fatalf("SetKeyBinding: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Bindings()["commit"]; len(got) != 1 || got[0] != "ctrl+k" {
		t.Errorf("commit = %v, want [ctrl+k]", got)
	}
	_ = fileBody(t, path)
}

// The point of editing rather than regenerating: everything the user wrote by
// hand is still there afterwards, byte for byte.
func TestSetKeyBindingLeavesTheRestOfTheFileAlone(t *testing.T) {
	path := useConfigDir(t)
	putFile(t, path, `# my tuigy config
theme: nord

# the editor has to wait
editor: "code --wait"

keys:
  # muscle memory from another tool
  quit: Q
  commit: c

ai:
  command: "my-agent"
`)

	if _, err := SetKeyBinding("commit", []string{"ctrl+k"}); err != nil {
		t.Fatalf("SetKeyBinding: %v", err)
	}

	got := fileBody(t, path)
	for _, want := range []string{
		"# my tuigy config",
		"theme: nord",
		"# the editor has to wait",
		`editor: "code --wait"`,
		"# muscle memory from another tool",
		"quit: Q",
		`commit: "ctrl+k"`,
		`command: "my-agent"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the file no longer contains %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "commit: c\n") {
		t.Errorf("the old binding was left behind:\n%s", got)
	}
}

func TestSetKeyBindingAddsToAnExistingBlock(t *testing.T) {
	path := useConfigDir(t)
	putFile(t, path, "keys:\n  quit: Q\n")

	if _, err := SetKeyBinding("commit", []string{"ctrl+k"}); err != nil {
		t.Fatalf("SetKeyBinding: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Keys) != 2 {
		t.Errorf("got %d bindings, want the new one beside the old:\n%s", len(cfg.Keys), fileBody(t, path))
	}
}

// The starter file mentions the block entirely in comments. Setting a binding
// has to produce a real one rather than a file that says two different things.
func TestSetKeyBindingWorksAlongsideTheCommentedStarter(t *testing.T) {
	path := useConfigDir(t)
	if _, _, err := WriteStarter(); err != nil {
		t.Fatalf("WriteStarter: %v", err)
	}

	if _, err := SetKeyBinding("quit", []string{"Q"}); err != nil {
		t.Fatalf("SetKeyBinding: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Bindings()["quit"]; len(got) != 1 || got[0] != "Q" {
		t.Errorf("quit = %v, want [Q]:\n%s", got, fileBody(t, path))
	}
	// The documentation in the file is not disturbed by writing beside it.
	if !strings.Contains(fileBody(t, path), "# tuigy configuration") {
		t.Error("the starter's own comments were lost")
	}
}

// A keystroke that means something to YAML has to survive the round trip.
func TestAwkwardKeystrokesSurvive(t *testing.T) {
	useConfigDir(t)

	for _, stroke := range []string{",", "?", "-", "+", "[", "]", ":", "space"} {
		if _, err := SetKeyBinding("commit", []string{stroke}); err != nil {
			t.Fatalf("SetKeyBinding(%q): %v", stroke, err)
		}
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load after %q: %v", stroke, err)
		}
		if got := cfg.Bindings()["commit"]; len(got) != 1 || got[0] != stroke {
			t.Errorf("commit = %v after setting %q", got, stroke)
		}
	}
}

func TestSeveralKeysForOneAction(t *testing.T) {
	useConfigDir(t)

	if _, err := SetKeyBinding("commit", []string{"c", "ctrl+k"}); err != nil {
		t.Fatalf("SetKeyBinding: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Bindings()["commit"]; len(got) != 2 || got[0] != "c" || got[1] != "ctrl+k" {
		t.Errorf("commit = %v, want [c ctrl+k]", got)
	}
}

func TestResetRemovesTheOverride(t *testing.T) {
	path := useConfigDir(t)
	putFile(t, path, "theme: nord\n\nkeys:\n  quit: Q\n  commit: ctrl+k\n")

	if _, err := ResetKeyBinding("quit"); err != nil {
		t.Fatalf("ResetKeyBinding: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, still := cfg.Keys["quit"]; still {
		t.Errorf("quit is still overridden:\n%s", fileBody(t, path))
	}
	if _, gone := cfg.Keys["commit"]; !gone {
		t.Errorf("resetting one binding removed another:\n%s", fileBody(t, path))
	}
	if !strings.Contains(fileBody(t, path), "theme: nord") {
		t.Error("resetting a binding disturbed the rest of the file")
	}
}

// An empty keys block is not a valid shape, so the last reset takes the block
// with it rather than leaving the file unreadable.
func TestResettingTheLastOverrideRemovesTheBlock(t *testing.T) {
	path := useConfigDir(t)
	putFile(t, path, "theme: nord\n\nkeys:\n  quit: Q\n")

	if _, err := ResetKeyBinding("quit"); err != nil {
		t.Fatalf("ResetKeyBinding: %v", err)
	}

	got := fileBody(t, path)
	if strings.Contains(got, "keys:") {
		t.Errorf("the empty block was left behind:\n%s", got)
	}
	if _, err := Load(); err != nil {
		t.Fatalf("the file no longer loads: %v\n%s", err, got)
	}
}

func TestResetOnAFileWithNoBindingsChangesNothing(t *testing.T) {
	path := useConfigDir(t)
	putFile(t, path, "theme: nord\n")

	if _, err := ResetKeyBinding("quit"); err != nil {
		t.Fatalf("ResetKeyBinding: %v", err)
	}
	if got := fileBody(t, path); got != "theme: nord\n" {
		t.Errorf("the file changed:\n%q", got)
	}
}
