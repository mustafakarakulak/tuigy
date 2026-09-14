package editor

import (
	"os"
	"path/filepath"
	"testing"
)

// onlyOnPath restricts PATH to a directory holding just the named commands.
func onlyOnPath(t *testing.T, names ...string) {
	t.Helper()

	dir := t.TempDir()
	for _, name := range names {
		script := filepath.Join(dir, name)
		if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	t.Setenv("PATH", dir)
}

func TestResolvePrefersTheConfiguredCommand(t *testing.T) {
	onlyOnPath(t, "nano")

	if got := Resolve("code --wait", "vim", true); got != "code --wait" {
		t.Errorf("Resolve() = %q, want tuigy's own setting to win", got)
	}
}

func TestResolveUsesGitsEditorWhenItWasChosen(t *testing.T) {
	onlyOnPath(t, "nano")

	if got := Resolve("", "vim", true); got != "vim" {
		t.Errorf("Resolve() = %q, want the editor git was told to use", got)
	}
}

// git falls back to vi whether or not anyone asked for it, so an unchosen "vi"
// is a default rather than a decision, and a friendlier one is used instead.
func TestResolveAvoidsVIWhenNothingWasChosen(t *testing.T) {
	onlyOnPath(t, "nano")

	got := Resolve("", "vi", false)
	if Name(got) != "nano" {
		t.Errorf("Resolve() = %q, want a friendlier editor than vi", got)
	}
}

func TestResolveOrdersTheFriendlyEditors(t *testing.T) {
	onlyOnPath(t, "nano", "micro")

	if got := Name(Resolve("", "vi", false)); got != "micro" {
		t.Errorf("Resolve() = %q, want micro ahead of nano", got)
	}
}

func TestResolveFallsBackToGitWhenNothingFriendlyExists(t *testing.T) {
	onlyOnPath(t)

	if got := Resolve("", "vim", false); got != "vim" {
		t.Errorf("Resolve() = %q, want git's answer when there is nothing better", got)
	}
}

// git says "true" or ":" to mean "do not open an editor". Honouring that would
// make the key silently do nothing.
func TestResolveIgnoresANoOpEditor(t *testing.T) {
	for _, noOp := range []string{"", "true", ":", "/usr/bin/true"} {
		onlyOnPath(t, "nano")
		if got := Name(Resolve("", noOp, true)); got != "nano" {
			t.Errorf("Resolve(%q) = %q, want a real editor", noOp, got)
		}

		onlyOnPath(t)
		if got := Resolve("", noOp, true); got != "vi" {
			t.Errorf("Resolve(%q) = %q, want the last-resort editor", noOp, got)
		}
	}
}

func TestName(t *testing.T) {
	cases := map[string]string{
		"nano":                    "nano",
		"/opt/homebrew/bin/micro": "micro",
		"code --wait":             "code",
		"/usr/bin/env -S nvim":    "env",
		"":                        "",
	}

	for command, want := range cases {
		if got := Name(command); got != want {
			t.Errorf("Name(%q) = %q, want %q", command, got, want)
		}
	}
}
