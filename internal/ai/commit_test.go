package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// withCommand points Detect at a shell command for the duration of a test.
func withCommand(t *testing.T, command string) *Generator {
	t.Helper()
	t.Setenv(CommandEnv, command)

	g := Detect("")
	if g == nil {
		t.Fatal("Detect() = nil with a command configured")
	}
	return g
}

func TestDetectPrefersTheConfiguredCommand(t *testing.T) {
	g := withCommand(t, "echo hello")
	if g.Name() != "configured command" {
		t.Errorf("Name() = %q", g.Name())
	}
}

func TestDetectReportsNothingWhenUnavailable(t *testing.T) {
	t.Setenv(CommandEnv, "")
	t.Setenv("PATH", t.TempDir()) // hide any agent the machine happens to have

	if g := Detect(""); g != nil {
		t.Errorf("Detect() = %+v, want nil when no agent is available", g)
	}
}

// The environment variable is the more immediate of the two, so it wins.
func TestEnvironmentOverridesTheConfiguredCommand(t *testing.T) {
	t.Setenv(CommandEnv, "printf 'from the environment\n'")

	g := Detect("printf 'from the config file\n'")
	if g == nil {
		t.Fatal("Detect() = nil")
	}

	got, err := g.CommitMessage(context.Background(), "some diff\n")
	if err != nil {
		t.Fatalf("CommitMessage: %v", err)
	}
	if got != "from the environment" {
		t.Errorf("CommitMessage() = %q, want the environment's command to win", got)
	}
}

// With no variable set, the config file's command is used.
func TestConfiguredCommandIsUsedWhenTheEnvironmentIsUnset(t *testing.T) {
	t.Setenv(CommandEnv, "")

	g := Detect("printf 'from the config file\n'")
	if g == nil {
		t.Fatal("Detect() = nil with a command from the config file")
	}

	got, err := g.CommitMessage(context.Background(), "some diff\n")
	if err != nil {
		t.Fatalf("CommitMessage: %v", err)
	}
	if got != "from the config file" {
		t.Errorf("CommitMessage() = %q", got)
	}
}

func TestCommitMessageReadsTheDiffOnStdin(t *testing.T) {
	// The fake agent answers with whatever it was given, so the test can check
	// that the diff actually reaches the command.
	g := withCommand(t, "cat")

	got, err := g.CommitMessage(context.Background(), "diff --git a/x b/x\n+added\n")
	if err != nil {
		t.Fatalf("CommitMessage: %v", err)
	}
	if !strings.Contains(got, "+added") {
		t.Errorf("the command did not receive the diff, got %q", got)
	}
}

func TestCommitMessageUnwrapsCodeFences(t *testing.T) {
	cases := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "fenced with a language",
			command: "printf '```text\\nfeat: add retry handling\\n```\\n'",
			want:    "feat: add retry handling",
		},
		{
			name:    "fenced without a closing fence",
			command: "printf '```\\nfix: bound the retry loop\\n'",
			want:    "fix: bound the retry loop",
		},
		{
			name:    "plain, with surrounding blank lines",
			command: "printf '\\n\\nchore: tidy imports\\n\\n'",
			want:    "chore: tidy imports",
		},
		{
			name:    "a body is kept",
			command: "printf 'feat: add retry\\n\\nRetries failed charges twice.\\n'",
			want:    "feat: add retry\n\nRetries failed charges twice.",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := withCommand(t, c.command)

			got, err := g.CommitMessage(context.Background(), "some diff\n")
			if err != nil {
				t.Fatalf("CommitMessage: %v", err)
			}
			if got != c.want {
				t.Errorf("CommitMessage() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestCommitMessageSurfacesTheCommandsError(t *testing.T) {
	g := withCommand(t, "echo 'not logged in' >&2; exit 1")

	_, err := g.CommitMessage(context.Background(), "some diff\n")
	if err == nil {
		t.Fatal("a failing command should produce an error")
	}
	if !strings.Contains(err.Error(), "not logged in") {
		t.Errorf("error = %q, want the command's own message", err)
	}
}

func TestCommitMessageRejectsEmptyOutput(t *testing.T) {
	g := withCommand(t, "true")

	if _, err := g.CommitMessage(context.Background(), "some diff\n"); err == nil {
		t.Error("an empty answer should produce an error")
	}
}

func TestCommitMessageRejectsAnEmptyDiff(t *testing.T) {
	g := withCommand(t, "cat")

	if _, err := g.CommitMessage(context.Background(), "   \n"); err == nil {
		t.Error("generating from nothing should produce an error")
	}
}

// A very large diff is cut down rather than sent in full.
func TestCommitMessageTruncatesAHugeDiff(t *testing.T) {
	g := withCommand(t, "wc -c")

	huge := strings.Repeat("+a line of a very large refactor\n", 8000)
	if len(huge) <= maxDiff {
		t.Fatalf("test fixture is only %d bytes, which is not over the cap", len(huge))
	}

	got, err := g.CommitMessage(context.Background(), huge)
	if err != nil {
		t.Fatalf("CommitMessage: %v", err)
	}

	var sent int
	if _, err := fmt.Sscan(strings.TrimSpace(got), &sent); err != nil {
		t.Fatalf("could not read the byte count from %q: %v", got, err)
	}
	if sent > maxDiff+64 {
		t.Errorf("%d bytes were sent, want the diff capped near %d", sent, maxDiff)
	}
}

// Agents are chatty on failure; only the first line belongs in the footer.
func TestCommitMessageReportsOnlyTheFirstLineOfAnError(t *testing.T) {
	g := withCommand(t, "printf 'rate limited\\nretry after 60s\\nsee the docs\\n' >&2; exit 1")

	_, err := g.CommitMessage(context.Background(), "some diff\n")
	if err == nil {
		t.Fatal("a failing command should produce an error")
	}
	if strings.Contains(err.Error(), "retry after") {
		t.Errorf("error = %q, want only the first line", err)
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("error = %q, want the first line", err)
	}
}

// A command that cannot be started at all still has to say something useful.
func TestCommitMessageReportsACommandThatCannotRun(t *testing.T) {
	g := withCommand(t, "exit 127")

	if _, err := g.CommitMessage(context.Background(), "some diff\n"); err == nil {
		t.Error("a command that fails silently should still be an error")
	}
}
