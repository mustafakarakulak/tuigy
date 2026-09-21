# 16. The terminal is a band, and reserves one key

Status: accepted

## Context

[15](0015-a-workspace-around-the-git-tool.md) accepted an embedded shell. Hosting one
raises three questions that have to be answered together, because a wrong answer to
any of them makes the pane not worth having.

**Where does it go.** It was first built as a takeover of the detail pane — the
diff, the commit body, the file preview — on whichever tab was open. That was wrong
within a minute of using it: running a command hid the thing the command was about,
and no file could be reviewed while anything ran.

**Which keys does it get.** A shell needs every keystroke tuigy binds, plus ctrl+c,
ctrl+d, ctrl+u and tab. [14](0014-what-tuigy-is-not.md) called the way out of that
"tmux's prefix, reimplemented worse". It is not wrong; the question is how much worse.

**Who writes the emulator.** A PTY is easy. A VT parser that a shell, vim and a
coding agent all render correctly against is not.

## Decision

**A band across the bottom, under both panes.** Not a third column and not a
replacement for anything. It asks for 12 rows and never takes more than half the
body, so the panes above always keep a drawable box. The whole reason to run
something here is to see what it does to the files already on screen, and a layout
that hides them defeats it.

**Exactly one reserved keystroke: ctrl+o.** Everything else reaches the child, ctrl+c
included — a shell that cannot be interrupted is not a shell. ctrl+o is chosen
because almost nothing binds it: it is bash's `operate-and-get-next`, which is real
but rare, and the alternatives (ctrl+b, ctrl+a) are exactly what tmux and screen take.
The footer shows nothing but that one key for as long as the shell is focused, so the
way out is never something to remember.

**Typing `exit` closes the band.** Leaving a dead screen up to be dismissed with a
second keystroke makes the user say the same thing twice.

**Emulation is a dependency.** `creack/pty` for the pseudo-terminal,
`charmbracelet/x/vt` for the screen. Keystrokes are encoded *by the emulator* rather
than by us, because the encoding depends on modes the child sets: an arrow key is
`\x1b[A` normally and `\x1bOA` to a program that has asked for application cursor
keys, and getting that wrong breaks the arrow keys in vim and in the shell's own line
editor. Resizes are forwarded to the PTY, so a full-screen program redraws to fit.

## Consequences

Nothing running in the pane can use ctrl+o. That is the whole tax, and it is smaller
than the one 14 feared.

The band has no scrollback of its own. What scrolled past belongs to the program that
printed it, and reaching it is that program's job — `less`, or the shell's own
history. A scrollback view would need its own focus mode inside a pane that already
has one.

Emulation bugs are upstream's. That is the point of the dependency, but it also means
they cannot be fixed here on a deadline. One is already worked around:
`vt.Emulator.Close` writes an unsynchronised flag that `Read` tests on its way in,
which races with the goroutine carrying keystrokes to the child. The pipe underneath
is synchronised properly, so `internal/term` closes it directly and leaves the flag
alone. If that workaround is ever removed, `go test -race ./internal/term` is what
notices.

Shell sessions are per-repository. Switching projects closes the band, because a shell
opened in one working tree has no business in another.

The pane is a `tea.Cmd` blocked on the session, not a poll: the emulator signals a
single-token channel when the screen changes, so a command printing thousands of lines
costs one redraw per frame the interface actually draws.
