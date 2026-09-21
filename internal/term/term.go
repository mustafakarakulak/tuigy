// Package term runs a shell on a pseudo-terminal and keeps a screen of what it
// has printed.
//
// The screen is a real terminal emulator rather than a scrollback of bytes,
// because the things people actually run in a repository — an editor, a test
// watcher, a coding agent — redraw themselves with cursor movement and expect
// to be talking to a terminal.
package term

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

// closeGrace is how long a shell is given to leave on its own after the
// pseudo-terminal is closed, before it is killed.
const closeGrace = 2 * time.Second

// Session is a shell running on its own pseudo-terminal.
//
// Three goroutines meet here: the one reading the child's output, the one
// carrying keystrokes back to it, and the interface rendering the screen. Every
// touch of the emulator's screen state goes through mu; the input pipe has its
// own synchronisation and is deliberately left outside it, so a keystroke can
// never block on a redraw.
type Session struct {
	cmd *exec.Cmd
	tty *os.File
	emu *vt.Emulator

	mu            sync.Mutex
	cursorVisible bool

	// updates carries "something changed, draw again". It holds one token: the
	// interface only ever needs to know that it is behind, not how far.
	updates chan struct{}

	// done is closed when the shell exits.
	done      chan struct{}
	closeOnce sync.Once
}

// Start launches the user's shell in dir on a pseudo-terminal of the given size.
func Start(dir string, width, height int) (*Session, error) {
	width, height = max(width, 1), max(height, 1)

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell)
	cmd.Dir = dir
	// TERM is what the emulator can actually honour, whatever the outer
	// terminal claims to be. TUIGY marks the shell as one tuigy opened, which
	// is what a prompt would need to say so.
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "TUIGY=1")

	tty, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(height), Cols: uint16(width)})
	if err != nil {
		return nil, err
	}

	s := &Session{
		cmd:           cmd,
		tty:           tty,
		emu:           vt.NewEmulator(width, height),
		cursorVisible: true,
		updates:       make(chan struct{}, 1),
		done:          make(chan struct{}),
	}

	s.emu.SetCallbacks(vt.Callbacks{
		CursorVisibility: func(visible bool) {
			s.cursorVisible = visible // already under mu: callbacks run inside Write
		},
	})

	go s.pump()
	go s.forwardInput()

	return s, nil
}

// pump feeds the child's output into the emulator until the shell exits.
func (s *Session) pump() {
	defer s.finish()

	buf := make([]byte, 32*1024)
	for {
		n, err := s.tty.Read(buf)
		if n > 0 {
			s.mu.Lock()
			_, _ = s.emu.Write(buf[:n])
			s.mu.Unlock()
			s.notify()
		}
		if err != nil {
			return
		}
	}
}

// forwardInput carries what the emulator encodes from keystrokes to the child.
//
// Going through the emulator rather than encoding keys here is what makes
// arrow keys work inside a program that has switched the terminal into
// application cursor mode: the emulator knows which mode it is in and we do not.
func (s *Session) forwardInput() {
	_, _ = io.Copy(s.tty, s.emu)
}

// finish records that the shell has exited and releases the pseudo-terminal.
func (s *Session) finish() {
	s.closeOnce.Do(func() {
		_ = s.cmd.Wait()
		_ = s.tty.Close()
		s.closeInput()
		close(s.done)
	})
	s.notify()
}

// closeInput unblocks forwardInput by closing the pipe it is reading from.
//
// Emulator.Close would do this too, but it also sets an unsynchronised flag
// that Emulator.Read tests on its way in, which is a data race between the two
// goroutines that meet here. The pipe underneath is synchronised properly, so
// it is closed directly and the flag is left alone: nothing writes to the
// emulator after the shell has gone.
func (s *Session) closeInput() {
	if pipe, ok := s.emu.InputPipe().(*io.PipeWriter); ok {
		_ = pipe.CloseWithError(io.EOF)
	}
}

// notify says the screen is out of date, without ever blocking: a full channel
// already means exactly what this call would add.
func (s *Session) notify() {
	select {
	case s.updates <- struct{}{}:
	default:
	}
}

// Updates fires when the screen has changed or the shell has exited.
func (s *Session) Updates() <-chan struct{} { return s.updates }

// Done is closed once the shell has exited.
func (s *Session) Done() <-chan struct{} { return s.done }

// Exited reports whether the shell has finished.
func (s *Session) Exited() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// SendKey passes a keystroke to the shell.
func (s *Session) SendKey(key uv.KeyEvent) {
	if s.Exited() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emu.SendKey(key)
}

// SendText pastes literal text, which is how a bracketed paste and a multi-rune
// keystroke both arrive.
func (s *Session) SendText(text string) {
	if text == "" || s.Exited() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emu.SendText(text)
}

// Resize changes the size of the screen and tells the child about it, so that a
// program drawing a full-screen interface redraws at the new size.
func (s *Session) Resize(width, height int) {
	width, height = max(width, 1), max(height, 1)

	s.mu.Lock()
	if s.emu.Width() == width && s.emu.Height() == height {
		s.mu.Unlock()
		return
	}
	s.emu.Resize(width, height)
	s.mu.Unlock()

	_ = pty.Setsize(s.tty, &pty.Winsize{Rows: uint16(height), Cols: uint16(width)})
	s.notify()
}

// Screen is the state the interface draws: the rendered rows plus where the
// cursor is, taken together so that the two can never disagree.
type Screen struct {
	// Lines holds one entry per row, with the child's own colours still in it.
	Lines []string
	// CursorX and CursorY are zero-based, and CursorVisible is false when the
	// child has hidden the cursor or the shell has exited.
	CursorX, CursorY int
	CursorVisible    bool
}

// Read takes a consistent snapshot of the screen.
func (s *Session) Read() Screen {
	s.mu.Lock()
	defer s.mu.Unlock()

	pos := s.emu.CursorPosition()
	return Screen{
		Lines:         splitLines(s.emu.Render(), s.emu.Height()),
		CursorX:       pos.X,
		CursorY:       pos.Y,
		CursorVisible: s.cursorVisible && !s.Exited(),
	}
}

// splitLines cuts the rendered screen into exactly height rows, so that the
// pane is drawn the same whether the shell has filled it or not.
func splitLines(rendered string, height int) []string {
	lines := make([]string, 0, height)
	start := 0
	for i := 0; i < len(rendered) && len(lines) < height; i++ {
		if rendered[i] == '\n' {
			lines = append(lines, rendered[start:i])
			start = i + 1
		}
	}
	if len(lines) < height && start <= len(rendered) {
		lines = append(lines, rendered[start:])
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

// Close ends the session.
//
// Closing the pseudo-terminal is a hangup, which is what closing a terminal
// window does and what a shell knows how to clean up after. A child that
// ignores it is killed rather than left to hold the interface open: nothing
// here is worth hanging tuigy's exit on.
func (s *Session) Close() error {
	if s.Exited() {
		return nil
	}

	err := s.tty.Close()
	s.signal(syscall.SIGHUP)

	select {
	case <-s.done:
	case <-time.After(closeGrace):
		s.signal(syscall.SIGKILL)
		<-s.done
	}

	if errors.Is(err, os.ErrClosed) {
		return nil
	}
	return err
}

func (s *Session) signal(sig os.Signal) {
	if s.cmd.Process == nil {
		return
	}
	// The shell is a process group leader on its own pseudo-terminal, so the
	// signal is sent to the group: a hangup delivered only to the shell would
	// leave whatever it was running behind.
	if pgid, err := syscall.Getpgid(s.cmd.Process.Pid); err == nil {
		if sig, ok := sig.(syscall.Signal); ok {
			_ = syscall.Kill(-pgid, sig)
			return
		}
	}
	_ = s.cmd.Process.Signal(sig)
}
