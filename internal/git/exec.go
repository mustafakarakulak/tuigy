package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// globalArgs is prepended to every git invocation.
//
//	core.quotepath=false : keep non-ASCII paths unescaped
//	color.ui=false       : a user's color.ui=always must not pollute our parsing
var globalArgs = []string{"-c", "core.quotepath=false", "-c", "color.ui=false"}

const (
	lockRetries = 5
	lockBackoff = 80 * time.Millisecond

	// maxDiffOutput caps how much a single diff read buffers. Dropping a
	// vendored dependency produces a diff of hundreds of megabytes, and the
	// view can only ever show the first few thousand lines of it.
	maxDiffOutput = 8 << 20
)

// CmdError carries a failed git invocation along with the stderr output, which
// is what we show the user.
type CmdError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *CmdError) Error() string {
	if msg := strings.TrimSpace(e.Stderr); msg != "" {
		return msg
	}
	return fmt.Sprintf("git %s: %v", strings.Join(e.Args, " "), e.Err)
}

func (e *CmdError) Unwrap() error { return e.Err }

// ExitCode reports git's exit status, or -1 if it cannot be determined.
func (e *CmdError) ExitCode() int {
	var ee *exec.ExitError
	if errors.As(e.Err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// ExitCode reports the git exit status behind err, or -1 if err is not a git error.
func ExitCode(err error) int {
	var ce *CmdError
	if errors.As(err, &ce) {
		return ce.ExitCode()
	}
	return -1
}

// Runner invokes the git binary inside a working tree.
//
// Writes are serialised and retried on lock contention: tuigy typically runs
// against the same repository as an AI coding agent, which makes a busy
// .git/index.lock an expected condition rather than an exceptional one.
type Runner struct {
	dir     string
	writeMu sync.Mutex
}

func NewRunner(dir string) *Runner { return &Runner{dir: dir} }

// Read runs a command that does not modify the repository.
//
// It sets GIT_OPTIONAL_LOCKS=0 so that frequently repeated reads such as status
// and diff never take the index lock and never block a concurrent agent.
func (r *Runner) Read(ctx context.Context, args ...string) (string, error) {
	return r.exec(ctx, args, []string{"GIT_OPTIONAL_LOCKS=0"})
}

// ReadC is Read with a C locale, for commands whose output git translates.
// for-each-ref's tracking summary is one such case.
func (r *Runner) ReadC(ctx context.Context, args ...string) (string, error) {
	return r.exec(ctx, args, []string{"GIT_OPTIONAL_LOCKS=0", "LC_ALL=C"})
}

// ReadDiff runs a read command whose output is a diff, discarding anything past
// a sane cap so that one enormous commit cannot exhaust memory.
func (r *Repo) readDiff(ctx context.Context, args ...string) (string, error) {
	return r.run.readCapped(ctx, args, maxDiffOutput)
}

// Write runs a command that modifies the repository.
func (r *Runner) Write(ctx context.Context, args ...string) (string, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	var (
		out string
		err error
	)
	for attempt := range lockRetries {
		out, err = r.exec(ctx, args, []string{"GIT_EDITOR=true"})
		if !isLockError(err) {
			return out, err
		}
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-time.After(lockBackoff * time.Duration(attempt+1)):
		}
	}
	return out, err
}

func (r *Runner) readCapped(ctx context.Context, args []string, limit int) (string, error) {
	return r.execCapped(ctx, args, []string{"GIT_OPTIONAL_LOCKS=0"}, limit)
}

func (r *Runner) exec(ctx context.Context, args, extraEnv []string) (string, error) {
	return r.execCapped(ctx, args, extraEnv, 0)
}

func (r *Runner) execCapped(ctx context.Context, args, extraEnv []string, limit int) (string, error) {
	full := append(append([]string{}, globalArgs...), args...)

	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Dir = r.dir
	cmd.Env = append(baseEnv(), extraEnv...)

	stdout := &cappedBuffer{limit: limit}
	var stderr bytes.Buffer
	cmd.Stdout = stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.String(), &CmdError{Args: full, Stderr: stderr.String(), Err: err}
	}
	return stdout.String(), nil
}

// cappedBuffer collects output up to a limit and silently drops the rest.
//
// It always reports a full write so that git is never killed by a broken pipe
// partway through; the cost of reading the remainder is small next to the cost
// of holding it.
type cappedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if c.limit <= 0 {
		return c.buf.Write(p)
	}
	if room := c.limit - c.buf.Len(); room > 0 {
		c.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

func (c *cappedBuffer) String() string { return c.buf.String() }

// baseEnv stops git from doing anything that would corrupt the TUI.
//
// GIT_TERMINAL_PROMPT=0 is a deliberate trade-off: pushing to an HTTPS remote
// without a credential helper fails fast with an error we can display, rather
// than hanging on a prompt the user cannot see.
//
// GIT_EDITOR is set on writes only, where a command such as merge --continue
// would otherwise stop for a commit message. Reads are left alone so that git
// can still be asked what the user's real editor is.
func baseEnv() []string {
	return append(os.Environ(),
		"GIT_PAGER=cat",
		"PAGER=cat",
		"GIT_TERMINAL_PROMPT=0",
	)
}

func isLockError(err error) bool {
	var ce *CmdError
	if !errors.As(err, &ce) {
		return false
	}
	return strings.Contains(ce.Stderr, "index.lock")
}
