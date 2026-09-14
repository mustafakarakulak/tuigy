package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func configPath(t *testing.T) string {
	t.Helper()
	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	return path
}

func TestSetThemeCreatesTheFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path, err := SetTheme("nord")
	if err != nil {
		t.Fatalf("SetTheme: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.TrimSpace(string(body)) != "theme: nord" {
		t.Errorf("file holds %q", body)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Theme != "nord" {
		t.Errorf("Theme = %q, want nord", cfg.Theme)
	}
}

// The file was written by hand, so everything the user put in it has to survive.
func TestSetThemeKeepsTheRestOfTheFile(t *testing.T) {
	writeConfig(t, "config.yml", `# my tuigy settings
theme: dracula

# the editor I actually use
editor: "code --wait"

keys:
  quit: Q
`)

	if _, err := SetTheme("gruvbox"); err != nil {
		t.Fatalf("SetTheme: %v", err)
	}

	body, err := os.ReadFile(configPath(t))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(body)

	for _, want := range []string{
		"# my tuigy settings",
		"theme: gruvbox",
		"# the editor I actually use",
		`editor: "code --wait"`,
		"quit: Q",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the file lost %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "dracula") {
		t.Errorf("the old theme is still there:\n%s", got)
	}
	if strings.Count(got, "theme:") != 1 {
		t.Errorf("there should be exactly one theme setting:\n%s", got)
	}
}

// A file that only documents the options, like the one --init-config writes,
// gains the setting without losing what it explains.
func TestSetThemeOnACommentedFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path := configPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(starter), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := SetTheme("nord"); err != nil {
		t.Fatalf("SetTheme: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(body), "# tuigy configuration") {
		t.Error("the documentation was lost")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Theme != "nord" {
		t.Errorf("Theme = %q, want nord", cfg.Theme)
	}
}

// Saving twice replaces the setting rather than piling up duplicates.
func TestSetThemeTwice(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	for _, name := range []string{"nord", "dracula", "gruvbox"} {
		if _, err := SetTheme(name); err != nil {
			t.Fatalf("SetTheme(%s): %v", name, err)
		}
	}

	body, err := os.ReadFile(configPath(t))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if n := strings.Count(string(body), "theme:"); n != 1 {
		t.Errorf("%d theme settings in the file:\n%s", n, body)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Theme != "gruvbox" {
		t.Errorf("Theme = %q, want the last one saved", cfg.Theme)
	}
}

func TestWriteStarterIsReadable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path, created, err := WriteStarter()
	if err != nil {
		t.Fatalf("WriteStarter: %v", err)
	}
	if !created {
		t.Fatal("created = false for a fresh directory")
	}

	// It is entirely commented out, so it must change nothing.
	cfg, err := Load()
	if err != nil {
		t.Fatalf("the starter file cannot be read back: %v", err)
	}
	if cfg.Theme != "" || cfg.Editor != "" || len(cfg.Keys) != 0 {
		t.Errorf("the starter file changes settings: %+v", cfg)
	}

	// Running it again leaves the file alone.
	if err := os.WriteFile(path, []byte("theme: nord\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, created, err := WriteStarter(); err != nil || created {
		t.Errorf("WriteStarter overwrote an existing file (created=%v, err=%v)", created, err)
	}
	body, _ := os.ReadFile(path)
	if strings.TrimSpace(string(body)) != "theme: nord" {
		t.Errorf("the existing file was changed:\n%s", body)
	}
}
