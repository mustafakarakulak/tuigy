// Package ai generates commit messages by handing a diff to a coding agent the
// user already runs, rather than talking to a model provider directly.
//
// That choice keeps tuigy out of the business of API keys, billing and SDK
// versions, and it means the generated message comes from whatever agent the
// user has already chosen and configured.
package ai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommandEnv names the environment variable holding the command to run. The
// diff arrives on its standard input and the message is read from its output.
const CommandEnv = "TUIGY_AI_COMMIT"

// maxDiff caps how much of a diff is sent. A large refactor can produce
// megabytes, which no agent needs to see to name the change.
const maxDiff = 96 << 10

const prompt = `Write a git commit message for the staged diff on stdin.

Use the Conventional Commits format: type(scope): subject.
Keep the subject in the imperative mood and under 72 characters.
Add a short body only if the change is not self-explanatory.
Output only the commit message: no code fences, no commentary, no preamble.`

// Generator turns a staged diff into a commit message.
type Generator struct {
	// name is what to show the user, such as "claude".
	name string
	// argv is the command to run. When it is a shell invocation it came from
	// the environment, and the user is responsible for its contents.
	argv []string
}

// Name identifies the agent behind this generator.
func (g *Generator) Name() string { return g.name }

// Detect finds a way to generate commit messages, or reports nil when there is
// none. A missing generator is not an error: the feature simply stays hidden.
//
// configured is the command from the config file; the environment variable
// wins over it, so a shell session can point at a different agent without
// editing anything.
func Detect(configured string) *Generator {
	command := strings.TrimSpace(os.Getenv(CommandEnv))
	if command == "" {
		command = strings.TrimSpace(configured)
	}
	if command != "" {
		return &Generator{
			name: "configured command",
			argv: []string{"sh", "-c", command},
		}
	}

	// Only agents whose headless interface is known are invoked unprompted.
	// Anything else is one environment variable away.
	if path, err := exec.LookPath("claude"); err == nil {
		return &Generator{name: "claude", argv: []string{path, "-p", prompt}}
	}

	return nil
}

// CommitMessage asks the agent to name the change the diff describes.
func (g *Generator) CommitMessage(ctx context.Context, diff string) (string, error) {
	if strings.TrimSpace(diff) == "" {
		return "", errors.New("there is nothing staged to describe")
	}

	if len(diff) > maxDiff {
		diff = diff[:maxDiff] + "\n\n[diff truncated]\n"
	}

	cmd := exec.CommandContext(ctx, g.argv[0], g.argv[1:]...)
	cmd.Stdin = strings.NewReader(diff)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%s: %s", g.name, firstLine(msg))
		}
		return "", fmt.Errorf("%s: %w", g.name, err)
	}

	message := clean(stdout.String())
	if message == "" {
		return "", fmt.Errorf("%s returned an empty message", g.name)
	}
	return message, nil
}

// clean strips the wrapping an agent tends to add around its answer.
func clean(out string) string {
	out = strings.TrimSpace(out)

	// A fenced block is the most common wrapper; unwrap it rather than leaving
	// backticks in the commit message.
	if strings.HasPrefix(out, "```") {
		if _, rest, ok := strings.Cut(out, "\n"); ok {
			if body, _, ok := strings.Cut(rest, "```"); ok {
				out = body
			} else {
				out = rest
			}
		}
	}

	return strings.TrimSpace(out)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
