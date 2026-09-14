package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mustafakarakulak/tuigy/internal/git"
	"github.com/mustafakarakulak/tuigy/internal/ui"
)

// openTestRepo creates a repository for configure to be pointed at.
func openTestRepo(t *testing.T) *git.Repo {
	t.Helper()

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	repo, err := git.Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}
	return repo
}

// writeConfig puts a config file where configure will find it, and restores the
// default theme afterwards since styles are process-wide.
func writeConfig(t *testing.T, body string) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Cleanup(func() {
		if err := ui.ApplyTheme("default", nil); err != nil {
			t.Fatalf("restoring the default theme: %v", err)
		}
	})

	if body == "" {
		return
	}
	if err := os.MkdirAll(filepath.Join(dir, "tuigy"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tuigy", "config.yml"), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestConfigureWithoutAConfigFile(t *testing.T) {
	writeConfig(t, "")

	model, err := configure(openTestRepo(t))
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	if model == nil {
		t.Fatal("configure returned no model")
	}
}

func TestConfigureAppliesTheFile(t *testing.T) {
	writeConfig(t, `
theme: nord
colors:
  accent: "#ff79c6"
keys:
  commit: [c, ctrl+k]
ai:
  command: "printf 'feat(x) add a thing'"
`)

	if _, err := configure(openTestRepo(t)); err != nil {
		t.Fatalf("configure: %v", err)
	}
}

// A configuration that cannot be honoured stops startup, and the message says
// which file and what is wrong with it.
func TestConfigureReportsBadConfiguration(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"unknown theme", "theme: solarized\n", "solarized"},
		{"bad colour", "colors:\n  accent: blue\n", "hex colour"},
		{"unknown action", "keys:\n  comit: c\n", "comit"},
		{"action with no keys", "keys:\n  commit: []\n", "no keys"},
		{"broken yaml", "theme: [not, a, string]\n", "yaml"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			writeConfig(t, c.body)

			_, err := configure(openTestRepo(t))
			if err == nil {
				t.Fatalf("%s should have been rejected", c.name)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %q, want it to mention %q", err, c.want)
			}
			if !strings.Contains(err.Error(), "config.yml") {
				t.Errorf("error = %q, want it to name the file", err)
			}
		})
	}
}

// The version string is what a bug report will quote, so it must always say
// something, ldflags or not.
func TestBuildInfo(t *testing.T) {
	got := buildInfo()
	if !strings.HasPrefix(got, "tuigy ") {
		t.Errorf("buildInfo() = %q", got)
	}
	if strings.Contains(got, "%!") {
		t.Errorf("buildInfo() = %q, which looks like a formatting mistake", got)
	}

	version, commit, date = "v1.2.3", "abc1234", "2026-09-14"
	t.Cleanup(func() { version, commit, date = "dev", "none", "unknown" })

	if got := buildInfo(); got != "tuigy v1.2.3 (abc1234, built 2026-09-14)" {
		t.Errorf("buildInfo() with ldflags = %q", got)
	}
}

// The informational flags are part of the tool's contract: a config file is
// written against what they print.
func TestCLIFlags(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"--version"}, []string{"tuigy "}},
		{[]string{"-v"}, []string{"tuigy "}},
		{[]string{"--themes"}, []string{"default", "dracula", "gruvbox", "nord"}},
		{[]string{"--actions"}, []string{"commit", "cherry-pick", "stash-pop", "quit"}},
	}

	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			var out strings.Builder
			if err := cli(c.args, &out); err != nil {
				t.Fatalf("cli(%v): %v", c.args, err)
			}
			for _, want := range c.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output is missing %q:\n%s", want, out.String())
				}
			}
		})
	}
}

func TestCLIRejectsUnknownFlags(t *testing.T) {
	var out strings.Builder
	if err := cli([]string{"--nope"}, &out); err == nil {
		t.Error("an unknown flag should be an error")
	}
}

// Outside a repository tuigy says so rather than starting up empty.
func TestRunOutsideARepository(t *testing.T) {
	t.Chdir(t.TempDir())

	err := run()
	if err == nil {
		t.Fatal("running outside a repository should fail")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Errorf("error = %q, want it to explain the problem", err)
	}
}

func TestCLIConfigFlags(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	var out strings.Builder
	if err := cli([]string{"--config"}, &out); err != nil {
		t.Fatalf("cli(--config): %v", err)
	}
	if !strings.Contains(out.String(), filepath.Join("tuigy", "config.yml")) {
		t.Errorf("--config printed %q", out.String())
	}

	out.Reset()
	if err := cli([]string{"--init-config"}, &out); err != nil {
		t.Fatalf("cli(--init-config): %v", err)
	}
	if !strings.Contains(out.String(), "wrote") {
		t.Errorf("--init-config said %q", out.String())
	}

	// Running it again must not overwrite what is now there.
	out.Reset()
	if err := cli([]string{"--init-config"}, &out); err != nil {
		t.Fatalf("cli(--init-config) again: %v", err)
	}
	if !strings.Contains(out.String(), "already exists") {
		t.Errorf("--init-config said %q the second time", out.String())
	}
}

// Asking for help is not a failure, so it must not exit non-zero.
func TestCLIHelpIsNotAnError(t *testing.T) {
	var out strings.Builder
	if err := cli([]string{"--help"}, &out); err != nil {
		t.Errorf("cli(--help) = %v, want no error", err)
	}
	if !strings.Contains(out.String(), "init-config") {
		t.Errorf("the usage text does not list the flags:\n%s", out.String())
	}
}
