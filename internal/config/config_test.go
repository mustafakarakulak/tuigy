package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeConfig puts a config file where Load will find it.
func writeConfig(t *testing.T, name, body string) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if err := os.MkdirAll(filepath.Join(dir, "tuigy"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tuigy", name), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestLoadWithoutAFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with no file: %v", err)
	}
	if cfg.Theme != "" || len(cfg.Keys) != 0 || len(cfg.Colors) != 0 {
		t.Errorf("Load() = %+v, want an empty config", cfg)
	}
}

func TestLoad(t *testing.T) {
	writeConfig(t, "config.yml", `
theme: dracula
editor: "code --wait"

colors:
  accent: "#bd93f9"

keys:
  commit: c
  quit: [q, ctrl+c]

ai:
  command: my-agent --headless
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Theme != "dracula" {
		t.Errorf("Theme = %q", cfg.Theme)
	}
	if cfg.Colors["accent"] != "#bd93f9" {
		t.Errorf("Colors = %v", cfg.Colors)
	}
	if cfg.AI.Command != "my-agent --headless" {
		t.Errorf("AI.Command = %q", cfg.AI.Command)
	}
	if cfg.Editor != "code --wait" {
		t.Errorf("Editor = %q", cfg.Editor)
	}

	bindings := cfg.Bindings()
	// A single key and a list of keys must both work.
	if got := bindings["commit"]; !slices.Equal(got, []string{"c"}) {
		t.Errorf("commit = %v, want [c]", got)
	}
	if got := bindings["quit"]; !slices.Equal(got, []string{"q", "ctrl+c"}) {
		t.Errorf("quit = %v, want [q ctrl+c]", got)
	}
}

// Both spellings of the extension are common enough to accept.
func TestLoadAcceptsTheYamlExtension(t *testing.T) {
	writeConfig(t, "config.yaml", "theme: nord\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Theme != "nord" {
		t.Errorf("Theme = %q, want nord", cfg.Theme)
	}
}

// A file that cannot be understood is reported, never quietly ignored.
func TestLoadReportsBrokenYAML(t *testing.T) {
	writeConfig(t, "config.yml", "theme: [not, a, string]\n")

	_, err := Load()
	if err == nil {
		t.Fatal("a config that cannot be parsed should be an error")
	}
	if !strings.Contains(err.Error(), "config.yml") {
		t.Errorf("error = %q, want it to name the file", err)
	}
}

func TestPathHonoursXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/somewhere")

	got, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if want := filepath.Join("/somewhere", "tuigy", "config.yml"); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

// Without XDG_CONFIG_HOME the file lives under the home directory, which is
// where most people will look for it.
func TestPathFallsBackToTheHomeDirectory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/someone")

	got, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if want := filepath.Join("/home/someone", ".config", "tuigy", "config.yml"); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestKeysRejectsAnUnexpectedShape(t *testing.T) {
	writeConfig(t, "config.yml", "keys:\n  commit:\n    a: b\n")

	_, err := Load()
	if err == nil {
		t.Fatal("a mapping where keys were expected should be an error")
	}
	if !strings.Contains(err.Error(), "key") {
		t.Errorf("error = %q, want it to explain what was expected", err)
	}
}

func TestBindingsOfAnEmptyConfig(t *testing.T) {
	var cfg Config
	if got := cfg.Bindings(); got != nil {
		t.Errorf("Bindings() = %v, want nil when nothing is configured", got)
	}
}

// Configuration paths are shown where width is tight, so they read the way a
// person would say them.
func TestShortPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}

	inside := filepath.Join(home, ".config", "tuigy", "config.yml")
	if got, want := ShortPath(inside), filepath.Join("~", ".config", "tuigy", "config.yml"); got != want {
		t.Errorf("ShortPath(%q) = %q, want %q", inside, got, want)
	}

	outside := filepath.Join("/etc", "tuigy", "config.yml")
	if got := ShortPath(outside); got != outside {
		t.Errorf("ShortPath(%q) = %q, want it unchanged", outside, got)
	}
}
