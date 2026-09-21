package term

import (
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
)

// waitFor polls the screen until want appears on it, which is the only honest
// way to test a child process: there is no moment at which its output is known
// to have arrived.
func waitFor(t *testing.T, s *Session, want string) Screen {
	t.Helper()

	deadline := time.After(10 * time.Second)
	for {
		screen := s.Read()
		if strings.Contains(strings.Join(screen.Lines, "\n"), want) {
			return screen
		}

		select {
		case <-deadline:
			t.Fatalf("waiting for %q, screen was:\n%s", want, strings.Join(screen.Lines, "\n"))
		case <-s.Updates():
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func start(t *testing.T, width, height int) *Session {
	t.Helper()

	t.Setenv("SHELL", "/bin/sh")
	s, err := Start(t.TempDir(), width, height)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSessionRunsACommand(t *testing.T) {
	s := start(t, 40, 10)

	s.SendText("echo tuigy-was-here")
	s.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})

	waitFor(t, s, "tuigy-was-here")
}

func TestScreenIsAlwaysTheFullHeight(t *testing.T) {
	s := start(t, 40, 12)

	if got := len(s.Read().Lines); got != 12 {
		t.Fatalf("got %d lines, want 12", got)
	}
}

func TestSessionStartsInTheGivenDirectory(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")

	// Wide enough that a temporary directory's path lands on one row: a path
	// wrapped across two rows is still correct but no longer one substring.
	dir := t.TempDir()
	s, err := Start(dir, 200, 10)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Close() }()

	s.SendText("pwd")
	s.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})

	// macOS reports /var as a symlink to /private/var, so the tail is what can
	// be asserted on.
	waitFor(t, s, strings.TrimPrefix(dir, "/private"))
}

func TestResizeIsReportedToTheChild(t *testing.T) {
	s := start(t, 40, 10)
	s.Resize(100, 20)

	if got := len(s.Read().Lines); got != 20 {
		t.Fatalf("got %d lines after resize, want 20", got)
	}

	s.SendText("echo $(tput cols)x$(tput lines)")
	s.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})

	waitFor(t, s, "100x20")
}

func TestExitEndsTheSession(t *testing.T) {
	s := start(t, 40, 10)

	s.SendText("exit")
	s.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})

	select {
	case <-s.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("the session did not finish after exit")
	}

	if !s.Exited() {
		t.Fatal("Exited() is false after the shell left")
	}
	// A finished session must still be safe to drive: the interface keeps
	// drawing it until the user closes the pane.
	s.SendText("ignored")
	s.Resize(20, 5)
	if err := s.Close(); err != nil {
		t.Fatalf("Close after exit: %v", err)
	}
}

func TestCloseStopsAShellThatIsStillRunning(t *testing.T) {
	s := start(t, 40, 10)
	waitFor(t, s, "$")

	done := make(chan error, 1)
	go func() { done <- s.Close() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not return")
	}
}
